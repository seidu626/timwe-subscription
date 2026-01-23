package domain

import (
	"encoding/json"
	"time"
)

// FlowType represents the subscription flow type
type FlowType string

const (
	FlowTypeClickToSMS FlowType = "CLICK_TO_SMS"
	FlowTypeOTP        FlowType = "OTP"
	FlowTypeRedirect   FlowType = "REDIRECT"
	FlowTypeMixed      FlowType = "MIXED"
)

// Campaign represents a marketing campaign configuration
type Campaign struct {
	ID          int                    `json:"id" db:"id"`
	Slug        string                 `json:"slug" db:"slug"`
	Language    string                 `json:"language" db:"language"`
	Country     string                 `json:"country" db:"country"`
	Operator     *string                `json:"operator,omitempty" db:"operator"`
	
	// Offer/product mapping
	OfferProductID int                  `json:"offer_product_id" db:"offer_product_id"`
	PricepointID   *int                 `json:"pricepoint_id,omitempty" db:"pricepoint_id"`
	PartnerRoleID  *int                 `json:"partner_role_id,omitempty" db:"partner_role_id"`
	
	// Flow configuration
	FlowType   FlowType                `json:"flow_type" db:"flow_type"`
	ShortCode  *string                  `json:"short_code,omitempty" db:"short_code"`
	SMSKeyword *string                  `json:"sms_keyword,omitempty" db:"sms_keyword"`
	
	// Pricing
	Price       *float64                `json:"price,omitempty" db:"price"`
	BillingCycle *string                `json:"billing_cycle,omitempty" db:"billing_cycle"`
	TrialFlags  json.RawMessage         `json:"trial_flags,omitempty" db:"trial_flags"`
	
	// Compliance
	TermsURL        *string             `json:"terms_url,omitempty" db:"terms_url"`
	InlineTermsText *string             `json:"inline_terms_text,omitempty" db:"inline_terms_text"`
	ConsentRequired bool                `json:"consent_required" db:"consent_required"`
	ConsentVersion  *string             `json:"consent_version,omitempty" db:"consent_version"`
	
	// Attribution and postback
	AttributionMapping json.RawMessage  `json:"attribution_mapping" db:"attribution_mapping"`
	PostbackRules      json.RawMessage  `json:"postback_rules" db:"postback_rules"`
	
	// Throttles and controls
	Throttles        json.RawMessage    `json:"throttles" db:"throttles"`
	AllowedReferrers []string           `json:"allowed_referrers,omitempty" db:"allowed_referrers"`
	AllowedSources    []string           `json:"allowed_sources,omitempty" db:"allowed_sources"`
	
	// Landing page URLs (multiple LPs can be bound to one campaign)
	LandingPageURLs []string            `json:"landing_page_urls,omitempty" db:"landing_page_urls"`

	// Tracking and analytics configuration (pixels, attribution model)
	TrackingConfig json.RawMessage      `json:"tracking_config,omitempty" db:"tracking_config"`

	// Metadata
	Enabled   bool                      `json:"enabled" db:"enabled"`
	CreatedAt time.Time                 `json:"created_at" db:"created_at"`
	UpdatedAt time.Time                 `json:"updated_at" db:"updated_at"`
	CreatedBy *string                   `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy *string                   `json:"updated_by,omitempty" db:"updated_by"`
}

// PublicCampaign is a public-safe subset of Campaign for landing page rendering
type PublicCampaign struct {
	Slug            string          `json:"slug"`
	Language        string          `json:"language"`
	Country         string          `json:"country"`
	FlowType        FlowType        `json:"flow_type"`
	ShortCode       *string         `json:"short_code,omitempty"`
	SMSKeyword      *string         `json:"sms_keyword,omitempty"`
	Price           *float64        `json:"price,omitempty"`
	BillingCycle    *string         `json:"billing_cycle,omitempty"`
	TermsURL        *string         `json:"terms_url,omitempty"`
	InlineTermsText *string         `json:"inline_terms_text,omitempty"`
	ConsentRequired bool            `json:"consent_required"`
	TrackingConfig  json.RawMessage `json:"tracking_config,omitempty"`
}

// ToPublic converts a Campaign to PublicCampaign
func (c *Campaign) ToPublic() *PublicCampaign {
	return &PublicCampaign{
		Slug:            c.Slug,
		Language:        c.Language,
		Country:         c.Country,
		FlowType:        c.FlowType,
		ShortCode:       c.ShortCode,
		SMSKeyword:      c.SMSKeyword,
		Price:           c.Price,
		BillingCycle:    c.BillingCycle,
		TermsURL:        c.TermsURL,
		InlineTermsText: c.InlineTermsText,
		ConsentRequired: c.ConsentRequired,
		TrackingConfig:  c.TrackingConfig,
	}
}
