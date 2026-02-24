package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// CampaignRepository handles campaign data access
type CampaignRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// ValidateOfferProductMapping verifies that the campaign offer_product_id points to a known product.
// When pricepointID is provided, it must match the mapped product price_point_id.
func (r *CampaignRepository) ValidateOfferProductMapping(offerProductID int, pricepointID *int) error {
	if offerProductID <= 0 {
		return fmt.Errorf("offer_product_id is required")
	}

	var mappedPricePoint int
	err := r.db.QueryRow(`SELECT price_point_id FROM products WHERE product_id = $1`, strconv.Itoa(offerProductID)).Scan(&mappedPricePoint)
	if err == sql.ErrNoRows {
		return fmt.Errorf("offer_product_id %d is not present in products mapping", offerProductID)
	}
	if err != nil {
		return fmt.Errorf("failed to validate offer_product_id mapping: %w", err)
	}

	if pricepointID != nil && *pricepointID > 0 && mappedPricePoint != *pricepointID {
		return fmt.Errorf("pricepoint_id %d does not match mapped product price_point_id %d for offer_product_id %d", *pricepointID, mappedPricePoint, offerProductID)
	}

	return nil
}

// NewCampaignRepository creates a new campaign repository
func NewCampaignRepository(db *sql.DB, logger *zap.Logger) *CampaignRepository {
	return &CampaignRepository{
		db:     db,
		logger: logger,
	}
}

// GetBySlug retrieves a campaign by slug
func (r *CampaignRepository) GetBySlug(slug string) (*domain.Campaign, error) {
	query := `
			SELECT id, slug, language, country, operator, offer_product_id, pricepoint_id,
			       partner_role_id, flow_type, short_code, sms_keyword, price, billing_cycle,
			       trial_flags, terms_url, inline_terms_text, consent_required, consent_version,
			       attribution_mapping, postback_rules, throttles, allowed_referrers,
			       allowed_sources, landing_page_urls, tracking_config, lp_copy,
			       enabled, created_at, updated_at, created_by, updated_by
			FROM campaigns
			WHERE slug = $1 AND enabled = true
		`

	var campaign domain.Campaign
	var operator, shortCode, smsKeyword, termsURL,
		inlineTermsText, consentVersion, createdBy, updatedBy sql.NullString
	var pricepointID, partnerRoleID sql.NullInt64 // Fixed: use NullInt64 for integer columns
	var price sql.NullFloat64
	var billingCycle sql.NullString
	var trialFlags, attributionMapping, postbackRules, throttles, trackingConfig, lpCopy sql.NullString
	var allowedReferrers, allowedSources, landingPageURLs pq.StringArray

	err := r.db.QueryRow(query, slug).Scan(
		&campaign.ID, &campaign.Slug, &campaign.Language, &campaign.Country, &operator,
		&campaign.OfferProductID, &pricepointID, &partnerRoleID, &campaign.FlowType,
		&shortCode, &smsKeyword, &price, &billingCycle, &trialFlags, &termsURL,
		&inlineTermsText, &campaign.ConsentRequired, &consentVersion,
		&attributionMapping, &postbackRules, &throttles, &allowedReferrers,
		&allowedSources, &landingPageURLs, &trackingConfig, &lpCopy,
		&campaign.Enabled, &campaign.CreatedAt, &campaign.UpdatedAt,
		&createdBy, &updatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("campaign not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	// Map nullable fields
	if operator.Valid {
		campaign.Operator = &operator.String
	}
	if pricepointID.Valid {
		id := int(pricepointID.Int64)
		campaign.PricepointID = &id
	}
	if partnerRoleID.Valid {
		id := int(partnerRoleID.Int64)
		campaign.PartnerRoleID = &id
	}
	if shortCode.Valid {
		campaign.ShortCode = &shortCode.String
	}
	if smsKeyword.Valid {
		campaign.SMSKeyword = &smsKeyword.String
	}
	if price.Valid {
		campaign.Price = &price.Float64
	}
	if billingCycle.Valid {
		campaign.BillingCycle = &billingCycle.String
	}
	if termsURL.Valid {
		campaign.TermsURL = &termsURL.String
	}
	if inlineTermsText.Valid {
		campaign.InlineTermsText = &inlineTermsText.String
	}
	if consentVersion.Valid {
		campaign.ConsentVersion = &consentVersion.String
	}
	if createdBy.Valid {
		campaign.CreatedBy = &createdBy.String
	}
	if updatedBy.Valid {
		campaign.UpdatedBy = &updatedBy.String
	}

	// Map JSON fields
	if trialFlags.Valid {
		campaign.TrialFlags = json.RawMessage(trialFlags.String)
	}
	if attributionMapping.Valid {
		campaign.AttributionMapping = json.RawMessage(attributionMapping.String)
	}
	if postbackRules.Valid {
		campaign.PostbackRules = json.RawMessage(postbackRules.String)
	}
	if throttles.Valid {
		campaign.Throttles = json.RawMessage(throttles.String)
	}
	if trackingConfig.Valid {
		campaign.TrackingConfig = json.RawMessage(trackingConfig.String)
	}
	if lpCopy.Valid {
		campaign.LPCopy = json.RawMessage(lpCopy.String)
	}

	campaign.AllowedReferrers = allowedReferrers
	campaign.AllowedSources = allowedSources
	campaign.LandingPageURLs = landingPageURLs

	return &campaign, nil
}

// GetAdminBySlug retrieves a campaign by slug (admin view; enabled + disabled).
func (r *CampaignRepository) GetAdminBySlug(slug string) (*domain.Campaign, error) {
	query := `
			SELECT id, slug, language, country, operator, offer_product_id, pricepoint_id,
			       partner_role_id, flow_type, short_code, sms_keyword, price, billing_cycle,
			       trial_flags, terms_url, inline_terms_text, consent_required, consent_version,
			       attribution_mapping, postback_rules, throttles, allowed_referrers,
			       allowed_sources, landing_page_urls, tracking_config, lp_copy,
			       enabled, created_at, updated_at, created_by, updated_by
			FROM campaigns
			WHERE slug = $1
		`

	var campaign domain.Campaign
	var operator, shortCode, smsKeyword, termsURL,
		inlineTermsText, consentVersion, createdBy, updatedBy sql.NullString
	var pricepointID, partnerRoleID sql.NullInt64
	var price sql.NullFloat64
	var billingCycle sql.NullString
	var trialFlags, attributionMapping, postbackRules, throttles, trackingConfig, lpCopy sql.NullString
	var allowedReferrers, allowedSources, landingPageURLs pq.StringArray

	err := r.db.QueryRow(query, slug).Scan(
		&campaign.ID, &campaign.Slug, &campaign.Language, &campaign.Country, &operator,
		&campaign.OfferProductID, &pricepointID, &partnerRoleID, &campaign.FlowType,
		&shortCode, &smsKeyword, &price, &billingCycle, &trialFlags, &termsURL,
		&inlineTermsText, &campaign.ConsentRequired, &consentVersion,
		&attributionMapping, &postbackRules, &throttles, &allowedReferrers,
		&allowedSources, &landingPageURLs, &trackingConfig, &lpCopy,
		&campaign.Enabled, &campaign.CreatedAt, &campaign.UpdatedAt,
		&createdBy, &updatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("campaign not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	if operator.Valid {
		campaign.Operator = &operator.String
	}
	if pricepointID.Valid {
		id := int(pricepointID.Int64)
		campaign.PricepointID = &id
	}
	if partnerRoleID.Valid {
		id := int(partnerRoleID.Int64)
		campaign.PartnerRoleID = &id
	}
	if shortCode.Valid {
		campaign.ShortCode = &shortCode.String
	}
	if smsKeyword.Valid {
		campaign.SMSKeyword = &smsKeyword.String
	}
	if price.Valid {
		campaign.Price = &price.Float64
	}
	if billingCycle.Valid {
		campaign.BillingCycle = &billingCycle.String
	}
	if termsURL.Valid {
		campaign.TermsURL = &termsURL.String
	}
	if inlineTermsText.Valid {
		campaign.InlineTermsText = &inlineTermsText.String
	}
	if consentVersion.Valid {
		campaign.ConsentVersion = &consentVersion.String
	}
	if createdBy.Valid {
		campaign.CreatedBy = &createdBy.String
	}
	if updatedBy.Valid {
		campaign.UpdatedBy = &updatedBy.String
	}

	if trialFlags.Valid {
		campaign.TrialFlags = json.RawMessage(trialFlags.String)
	}
	if attributionMapping.Valid {
		campaign.AttributionMapping = json.RawMessage(attributionMapping.String)
	}
	if postbackRules.Valid {
		campaign.PostbackRules = json.RawMessage(postbackRules.String)
	}
	if throttles.Valid {
		campaign.Throttles = json.RawMessage(throttles.String)
	}
	if trackingConfig.Valid {
		campaign.TrackingConfig = json.RawMessage(trackingConfig.String)
	}
	if lpCopy.Valid {
		campaign.LPCopy = json.RawMessage(lpCopy.String)
	}

	campaign.AllowedReferrers = allowedReferrers
	campaign.AllowedSources = allowedSources
	campaign.LandingPageURLs = landingPageURLs

	return &campaign, nil
}

// ListEnabled retrieves all enabled campaigns
func (r *CampaignRepository) ListEnabled() ([]*domain.Campaign, error) {
	query := `
			SELECT id, slug, language, country, operator, offer_product_id, pricepoint_id,
			       partner_role_id, flow_type, short_code, sms_keyword, price, billing_cycle,
			       trial_flags, terms_url, inline_terms_text, consent_required, consent_version,
			       attribution_mapping, postback_rules, throttles, allowed_referrers,
			       allowed_sources, landing_page_urls, tracking_config, lp_copy,
			       enabled, created_at, updated_at, created_by, updated_by
			FROM campaigns
			WHERE enabled = true
			ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		campaign, err := r.scanCampaign(rows)
		if err != nil {
			r.logger.Error("Failed to scan campaign", zap.Error(err))
			continue
		}
		campaigns = append(campaigns, campaign)
	}

	return campaigns, nil
}

// ListAll retrieves campaigns (enabled + disabled) with optional filters.
func (r *CampaignRepository) ListAll(enabled *bool, country *string) ([]*domain.Campaign, error) {
	query := `
			SELECT id, slug, language, country, operator, offer_product_id, pricepoint_id,
			       partner_role_id, flow_type, short_code, sms_keyword, price, billing_cycle,
			       trial_flags, terms_url, inline_terms_text, consent_required, consent_version,
			       attribution_mapping, postback_rules, throttles, allowed_referrers,
			       allowed_sources, landing_page_urls, tracking_config, lp_copy,
			       enabled, created_at, updated_at, created_by, updated_by
			FROM campaigns
			WHERE 1=1
		`

	args := []any{}
	argN := 1
	if enabled != nil {
		query += fmt.Sprintf(" AND enabled = $%d", argN)
		args = append(args, *enabled)
		argN++
	}
	if country != nil && *country != "" {
		query += fmt.Sprintf(" AND country = $%d", argN)
		args = append(args, *country)
		argN++
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		campaign, err := r.scanCampaign(rows)
		if err != nil {
			r.logger.Error("Failed to scan campaign", zap.Error(err))
			continue
		}
		campaigns = append(campaigns, campaign)
	}
	return campaigns, nil
}

func toNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	if strings.TrimSpace(*s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func toNullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func toNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func toNullJSON(raw json.RawMessage) sql.NullString {
	if len(raw) == 0 {
		return sql.NullString{}
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

// Create inserts a new campaign.
func (r *CampaignRepository) Create(c *domain.Campaign) (*domain.Campaign, error) {
	if c == nil {
		return nil, errors.New("campaign is nil")
	}

	query := `
			INSERT INTO campaigns (
				slug, language, country, operator,
				offer_product_id, pricepoint_id, partner_role_id,
				flow_type, short_code, sms_keyword,
				price, billing_cycle, trial_flags,
				terms_url, inline_terms_text, consent_required, consent_version,
				attribution_mapping, postback_rules,
				throttles, allowed_referrers, allowed_sources, landing_page_urls,
				tracking_config, lp_copy, enabled, created_by, updated_by
			) VALUES (
				$1,$2,$3,$4,
				$5,$6,$7,
				$8,$9,$10,
				$11,$12,$13,
				$14,$15,$16,$17,
				$18,$19,
				$20,$21,$22,$23,
				$24,$25,$26,$27,$28
			)
			RETURNING slug
		`

	operator := toNullString(c.Operator)
	pricepointID := toNullInt(c.PricepointID)
	partnerRoleID := toNullInt(c.PartnerRoleID)
	shortCode := toNullString(c.ShortCode)
	smsKeyword := toNullString(c.SMSKeyword)
	price := toNullFloat64(c.Price)
	billingCycle := toNullString(c.BillingCycle)
	trialFlags := toNullJSON(c.TrialFlags)
	termsURL := toNullString(c.TermsURL)
	inlineTermsText := toNullString(c.InlineTermsText)
	consentVersion := toNullString(c.ConsentVersion)
	attributionMapping := toNullJSON(c.AttributionMapping)
	postbackRules := toNullJSON(c.PostbackRules)
	throttles := toNullJSON(c.Throttles)
	trackingConfig := toNullJSON(c.TrackingConfig)
	lpCopy := toNullJSON(c.LPCopy)
	createdBy := toNullString(c.CreatedBy)
	updatedBy := toNullString(c.UpdatedBy)

	var slug string
	err := r.db.QueryRow(
		query,
		c.Slug, c.Language, c.Country, operator,
		c.OfferProductID, pricepointID, partnerRoleID,
		string(c.FlowType), shortCode, smsKeyword,
		price, billingCycle, trialFlags,
		termsURL, inlineTermsText, c.ConsentRequired, consentVersion,
		attributionMapping, postbackRules,
		throttles, pq.StringArray(c.AllowedReferrers), pq.StringArray(c.AllowedSources), pq.StringArray(c.LandingPageURLs),
		trackingConfig, lpCopy, c.Enabled, createdBy, updatedBy,
	).Scan(&slug)
	if err != nil {
		return nil, fmt.Errorf("failed to insert campaign: %w", err)
	}

	return r.GetAdminBySlug(slug)
}

// Update updates an existing campaign by slug (slug is immutable).
func (r *CampaignRepository) Update(slug string, c *domain.Campaign) (*domain.Campaign, error) {
	if c == nil {
		return nil, errors.New("campaign is nil")
	}
	if strings.TrimSpace(slug) == "" {
		return nil, errors.New("slug is required")
	}

	query := `
		UPDATE campaigns SET
			language = $1,
			country = $2,
			operator = $3,
			offer_product_id = $4,
			pricepoint_id = $5,
			partner_role_id = $6,
			flow_type = $7,
			short_code = $8,
			sms_keyword = $9,
			price = $10,
			billing_cycle = $11,
			trial_flags = $12,
			terms_url = $13,
			inline_terms_text = $14,
			consent_required = $15,
			consent_version = $16,
			attribution_mapping = $17,
			postback_rules = $18,
				throttles = $19,
				allowed_referrers = $20,
				allowed_sources = $21,
				landing_page_urls = $22,
				tracking_config = $23,
				lp_copy = $24,
				enabled = $25,
				updated_by = $26
			WHERE slug = $27
			RETURNING slug
		`

	operator := toNullString(c.Operator)
	pricepointID := toNullInt(c.PricepointID)
	partnerRoleID := toNullInt(c.PartnerRoleID)
	shortCode := toNullString(c.ShortCode)
	smsKeyword := toNullString(c.SMSKeyword)
	price := toNullFloat64(c.Price)
	billingCycle := toNullString(c.BillingCycle)
	trialFlags := toNullJSON(c.TrialFlags)
	termsURL := toNullString(c.TermsURL)
	inlineTermsText := toNullString(c.InlineTermsText)
	consentVersion := toNullString(c.ConsentVersion)
	attributionMapping := toNullJSON(c.AttributionMapping)
	postbackRules := toNullJSON(c.PostbackRules)
	throttles := toNullJSON(c.Throttles)
	trackingConfig := toNullJSON(c.TrackingConfig)
	lpCopy := toNullJSON(c.LPCopy)
	updatedBy := toNullString(c.UpdatedBy)

	var outSlug string
	err := r.db.QueryRow(
		query,
		c.Language,
		c.Country,
		operator,
		c.OfferProductID,
		pricepointID,
		partnerRoleID,
		string(c.FlowType),
		shortCode,
		smsKeyword,
		price,
		billingCycle,
		trialFlags,
		termsURL,
		inlineTermsText,
		c.ConsentRequired,
		consentVersion,
		attributionMapping,
		postbackRules,
		throttles,
		pq.StringArray(c.AllowedReferrers),
		pq.StringArray(c.AllowedSources),
		pq.StringArray(c.LandingPageURLs),
		trackingConfig,
		lpCopy,
		c.Enabled,
		updatedBy,
		slug,
	).Scan(&outSlug)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("campaign not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update campaign: %w", err)
	}

	return r.GetAdminBySlug(outSlug)
}

// SetEnabled sets the enabled flag for a campaign.
func (r *CampaignRepository) SetEnabled(slug string, enabled bool, updatedBy *string) (*domain.Campaign, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, errors.New("slug is required")
	}
	query := `
		UPDATE campaigns
		SET enabled = $1, updated_by = $2
		WHERE slug = $3
		RETURNING slug
	`

	var outSlug string
	if err := r.db.QueryRow(query, enabled, toNullString(updatedBy), slug).Scan(&outSlug); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("campaign not found: %s", slug)
		}
		return nil, fmt.Errorf("failed to set enabled: %w", err)
	}

	return r.GetAdminBySlug(outSlug)
}

// scanCampaign scans a campaign from database rows
func (r *CampaignRepository) scanCampaign(rows *sql.Rows) (*domain.Campaign, error) {
	var campaign domain.Campaign
	var operator, shortCode, smsKeyword, termsURL,
		inlineTermsText, consentVersion, createdBy, updatedBy sql.NullString
	var pricepointID, partnerRoleID sql.NullInt64 // Fixed: use NullInt64 for integer columns
	var price sql.NullFloat64
	var billingCycle sql.NullString
	var trialFlags, attributionMapping, postbackRules, throttles, trackingConfig, lpCopy sql.NullString
	var allowedReferrers, allowedSources, landingPageURLs pq.StringArray

	err := rows.Scan(
		&campaign.ID, &campaign.Slug, &campaign.Language, &campaign.Country, &operator,
		&campaign.OfferProductID, &pricepointID, &partnerRoleID, &campaign.FlowType,
		&shortCode, &smsKeyword, &price, &billingCycle, &trialFlags, &termsURL,
		&inlineTermsText, &campaign.ConsentRequired, &consentVersion,
		&attributionMapping, &postbackRules, &throttles, &allowedReferrers,
		&allowedSources, &landingPageURLs, &trackingConfig, &lpCopy,
		&campaign.Enabled, &campaign.CreatedAt, &campaign.UpdatedAt,
		&createdBy, &updatedBy,
	)

	if err != nil {
		return nil, err
	}

	// Map nullable fields (same as GetBySlug)
	if operator.Valid {
		campaign.Operator = &operator.String
	}
	if pricepointID.Valid {
		id := int(pricepointID.Int64)
		campaign.PricepointID = &id
	}
	if partnerRoleID.Valid {
		id := int(partnerRoleID.Int64)
		campaign.PartnerRoleID = &id
	}
	if shortCode.Valid {
		campaign.ShortCode = &shortCode.String
	}
	if smsKeyword.Valid {
		campaign.SMSKeyword = &smsKeyword.String
	}
	if price.Valid {
		campaign.Price = &price.Float64
	}
	if billingCycle.Valid {
		campaign.BillingCycle = &billingCycle.String
	}
	if termsURL.Valid {
		campaign.TermsURL = &termsURL.String
	}
	if inlineTermsText.Valid {
		campaign.InlineTermsText = &inlineTermsText.String
	}
	if consentVersion.Valid {
		campaign.ConsentVersion = &consentVersion.String
	}
	if createdBy.Valid {
		campaign.CreatedBy = &createdBy.String
	}
	if updatedBy.Valid {
		campaign.UpdatedBy = &updatedBy.String
	}

	if trialFlags.Valid {
		campaign.TrialFlags = json.RawMessage(trialFlags.String)
	}
	if attributionMapping.Valid {
		campaign.AttributionMapping = json.RawMessage(attributionMapping.String)
	}
	if postbackRules.Valid {
		campaign.PostbackRules = json.RawMessage(postbackRules.String)
	}
	if throttles.Valid {
		campaign.Throttles = json.RawMessage(throttles.String)
	}
	if trackingConfig.Valid {
		campaign.TrackingConfig = json.RawMessage(trackingConfig.String)
	}
	if lpCopy.Valid {
		campaign.LPCopy = json.RawMessage(lpCopy.String)
	}

	campaign.AllowedReferrers = allowedReferrers
	campaign.AllowedSources = allowedSources
	campaign.LandingPageURLs = landingPageURLs

	return &campaign, nil
}
