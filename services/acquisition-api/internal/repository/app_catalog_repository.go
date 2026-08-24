package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// appLPCopyText is the subset of the landing-page copy JSON (lp_copy) used as
// a catalog fallback when the dedicated app_* columns are null. lp_copy has
// no direct equivalent for category/artwork_url/sample_content, so those stay
// empty when app_* is unset; only name/tagline/description have a pragmatic
// lp_copy source.
type appLPCopyText struct {
	HeroTitle     string `json:"heroTitle"`
	HEDescription string `json:"heDescription"`
	SuccessBody   string `json:"successBody"`
}

type appLPCopyPayload struct {
	En *appLPCopyText `json:"en"`
}

// currencyForCountry is a small, explicit country->currency default used
// because neither campaigns nor acquisition_transactions carry a currency
// column. Unknown countries return "" (omitted from the JSON response)
// rather than guessing.
func currencyForCountry(country string) string {
	switch strings.ToUpper(strings.TrimSpace(country)) {
	case "GH":
		return "GHS"
	case "NG":
		return "NGN"
	case "KE":
		return "KES"
	default:
		return ""
	}
}

// ListAppCatalog returns the Dayline app catalog, optionally filtered by
// tenant and country. An empty tenantKey lists every active tenant's enabled
// campaigns (the marketplace view); rows always carry the owning tenant so
// products stay attributable across tenants. Only enabled campaigns on an
// active tenant are listed, matching GetEnabledBySlug's existing
// public-visibility semantics. Campaigns without a price are excluded: the
// app's confirm screen must disclose the charge before subscribing, so an
// un-priced campaign is not app-sellable.
func (r *CampaignRepository) ListAppCatalog(tenantKey, country string) ([]*domain.AppCatalogProduct, error) {
	query := `
		SELECT t.tenant_key, t.name, t.metadata_json -> 'branding' AS branding,
		       c.slug, c.price, c.billing_cycle, c.flow_type, c.country,
		       c.app_name, c.app_tagline, c.app_description, c.app_category,
		       c.app_artwork_url, c.app_sample_content, c.lp_copy, c.app_featured_rank,
		       (SELECT COUNT(*) FROM subscriptions s
		        WHERE s.tenant_id = c.tenant_id
		          AND s.product_id = c.offer_product_id
		          AND LOWER(s.status) = 'active') AS subscriber_count
		FROM campaigns c
		JOIN tenants t ON t.id = c.tenant_id
		WHERE c.enabled = true AND t.status = 'ACTIVE' AND c.price IS NOT NULL
	`
	args := []interface{}{}
	tenantKey = strings.TrimSpace(tenantKey)
	if tenantKey != "" {
		args = append(args, tenantKey)
		query += fmt.Sprintf(" AND t.tenant_key = $%d", len(args))
	}
	country = strings.TrimSpace(country)
	if country != "" {
		args = append(args, country)
		query += fmt.Sprintf(" AND c.country = $%d", len(args))
	}
	query += " ORDER BY t.tenant_key, c.app_featured_rank NULLS LAST, c.slug"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list app catalog: %w", err)
	}
	defer rows.Close()

	products := make([]*domain.AppCatalogProduct, 0)
	for rows.Next() {
		var (
			rowTenantKey, rowTenantName, slug, flowType, rowCountry        string
			billingCycle, appName, appTagline, appDescription, appCategory sql.NullString
			appArtworkURL, appSampleContent                                sql.NullString
			lpCopy, brandingJSON                                           sql.NullString
			price                                                          sql.NullFloat64
			featuredRank                                                   sql.NullInt64
			subscriberCount                                                int
		)
		if err := rows.Scan(&rowTenantKey, &rowTenantName, &brandingJSON, &slug, &price, &billingCycle, &flowType, &rowCountry,
			&appName, &appTagline, &appDescription, &appCategory,
			&appArtworkURL, &appSampleContent, &lpCopy, &featuredRank, &subscriberCount); err != nil {
			r.logger.Error("failed to scan app catalog row", zap.Error(err))
			continue
		}

		var lpEn *appLPCopyText
		if lpCopy.Valid && lpCopy.String != "" {
			var payload appLPCopyPayload
			if err := json.Unmarshal([]byte(lpCopy.String), &payload); err == nil {
				lpEn = payload.En
			}
		}

		var branding *domain.TenantBranding
		if brandingJSON.Valid && brandingJSON.String != "" {
			var parsed domain.TenantBranding
			if err := json.Unmarshal([]byte(brandingJSON.String), &parsed); err == nil &&
				(parsed.LogoURL != "" || parsed.BannerURL != "" || parsed.BrandColor != "") {
				branding = &parsed
			}
		}

		product := &domain.AppCatalogProduct{
			Slug:            slug,
			Tenant:          rowTenantKey,
			TenantName:      rowTenantName,
			Name:            firstNonEmpty(appName.String, lpTitleOf(lpEn), slug),
			Tagline:         firstNonEmpty(appTagline.String, lpDescriptionOf(lpEn)),
			Description:     firstNonEmpty(appDescription.String, lpSuccessBodyOf(lpEn)),
			Category:        appCategory.String,
			ArtworkURL:      appArtworkURL.String,
			SampleContent:   appSampleContent.String,
			Currency:        currencyForCountry(rowCountry),
			BillingCycle:    billingCycle.String,
			FlowType:        domain.FlowType(flowType),
			SubscriberCount: subscriberCount,
			Featured:        featuredRank.Valid,
			TenantBranding:  branding,
		}
		if price.Valid {
			v := price.Float64
			product.Price = &v
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate app catalog rows: %w", err)
	}
	return products, nil
}

func lpTitleOf(t *appLPCopyText) string {
	if t == nil {
		return ""
	}
	return t.HeroTitle
}

func lpDescriptionOf(t *appLPCopyText) string {
	if t == nil {
		return ""
	}
	return t.HEDescription
}

func lpSuccessBodyOf(t *appLPCopyText) string {
	if t == nil {
		return ""
	}
	return t.SuccessBody
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
