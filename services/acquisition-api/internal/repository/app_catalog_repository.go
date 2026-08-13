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

// ListAppCatalog returns the Dayline app catalog for a tenant, optionally
// filtered by country. Only enabled campaigns on an active tenant are
// listed, matching GetEnabledBySlug's existing public-visibility semantics.
func (r *CampaignRepository) ListAppCatalog(tenantKey, country string) ([]*domain.AppCatalogProduct, error) {
	tenantKey = strings.TrimSpace(tenantKey)
	if tenantKey == "" {
		return nil, fmt.Errorf("tenant is required")
	}

	query := `
		SELECT c.slug, c.price, c.billing_cycle, c.flow_type, c.country,
		       c.app_name, c.app_tagline, c.app_description, c.app_category,
		       c.app_artwork_url, c.app_sample_content, c.lp_copy
		FROM campaigns c
		JOIN tenants t ON t.id = c.tenant_id
		WHERE c.enabled = true AND t.status = 'ACTIVE' AND t.tenant_key = $1
	`
	args := []interface{}{tenantKey}
	country = strings.TrimSpace(country)
	if country != "" {
		query += " AND c.country = $2"
		args = append(args, country)
	}
	query += " ORDER BY c.slug"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list app catalog: %w", err)
	}
	defer rows.Close()

	products := make([]*domain.AppCatalogProduct, 0)
	for rows.Next() {
		var (
			slug, flowType, rowCountry                                     string
			billingCycle, appName, appTagline, appDescription, appCategory sql.NullString
			appArtworkURL, appSampleContent                                sql.NullString
			lpCopy                                                         sql.NullString
			price                                                          sql.NullFloat64
		)
		if err := rows.Scan(&slug, &price, &billingCycle, &flowType, &rowCountry,
			&appName, &appTagline, &appDescription, &appCategory,
			&appArtworkURL, &appSampleContent, &lpCopy); err != nil {
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

		product := &domain.AppCatalogProduct{
			Slug:            slug,
			Name:            firstNonEmpty(appName.String, lpTitleOf(lpEn), slug),
			Tagline:         firstNonEmpty(appTagline.String, lpDescriptionOf(lpEn)),
			Description:     firstNonEmpty(appDescription.String, lpSuccessBodyOf(lpEn)),
			Category:        appCategory.String,
			ArtworkURL:      appArtworkURL.String,
			SampleContent:   appSampleContent.String,
			Currency:        currencyForCountry(rowCountry),
			BillingCycle:    billingCycle.String,
			FlowType:        domain.FlowType(flowType),
			SubscriberCount: 0, // No subscriber-count source in acquisition-api; see result capsule.
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
