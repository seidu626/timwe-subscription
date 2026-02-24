package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TransactionStatus represents the status of an acquisition transaction
type TransactionStatus string

const (
	StatusPending         TransactionStatus = "PENDING"
	StatusActionRequired  TransactionStatus = "ACTION_REQUIRED"
	StatusConfirmRequired TransactionStatus = "CONFIRM_REQUIRED"
	StatusSubscribed      TransactionStatus = "SUBSCRIBED"
	StatusCharged         TransactionStatus = "CHARGED" // Charge success confirmed by subscription-external
	StatusFailed          TransactionStatus = "FAILED"
	StatusCancelled       TransactionStatus = "CANCELLED"
)

// NextAction represents the next action the user should take
type NextAction string

const (
	NextActionOpenSMS          NextAction = "OPEN_SMS"
	NextActionOTP              NextAction = "OTP"
	NextActionRedirect         NextAction = "REDIRECT"
	NextActionShowInstructions NextAction = "SHOW_INSTRUCTIONS"
	NextActionSubscribed       NextAction = "SUBSCRIBED" // HE path - direct subscription
)

// HESource represents the source of Header Enrichment identity
type HESource string

const (
	HESourceReal      HESource = "REAL"      // Real HE headers from MNO
	HESourceSimulated HESource = "SIMULATED" // Simulated for testing
	HESourceNone      HESource = "NONE"      // No HE detected
)

// AcquisitionTransaction represents a web acquisition attempt
type AcquisitionTransaction struct {
	ID            uuid.UUID `json:"id" db:"id"`
	CorrelationID uuid.UUID `json:"correlation_id" db:"correlation_id"`

	// Campaign and user
	CampaignSlug string `json:"campaign_slug" db:"campaign_slug"`
	MSISDN       string `json:"msisdn" db:"msisdn"`

	// Status and flow
	Status            TransactionStatus `json:"status" db:"status"`
	NextAction        *NextAction       `json:"next_action,omitempty" db:"next_action"`
	NextActionPayload json.RawMessage   `json:"next_action_payload,omitempty" db:"next_action_payload"`

	// Attribution
	AdProvider      *string         `json:"ad_provider,omitempty" db:"ad_provider"`
	ClickID         *string         `json:"click_id,omitempty" db:"click_id"`
	AttributionData json.RawMessage `json:"attribution_data" db:"attribution_data"`

	// Request metadata
	IPAddress *string `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent *string `json:"user_agent,omitempty" db:"user_agent"`

	// Consent tracking
	ConsentRequired    bool       `json:"consent_required" db:"consent_required"`
	ConsentChecked     bool       `json:"consent_checked" db:"consent_checked"`
	ConsentVersion     *string    `json:"consent_version,omitempty" db:"consent_version"`
	ConsentTimestamp   *time.Time `json:"consent_timestamp,omitempty" db:"consent_timestamp"`
	LandingVersionHash *string    `json:"landing_version_hash,omitempty" db:"landing_version_hash"`

	// Header Enrichment (HE) tracking
	HESource   *HESource `json:"he_source,omitempty" db:"he_source"`
	HEMSISDN   *string   `json:"he_msisdn,omitempty" db:"he_msisdn"`
	HEOperator *string   `json:"he_operator,omitempty" db:"he_operator"`

	// TIMWE integration
	OfferProductID      *int    `json:"offer_product_id,omitempty" db:"offer_product_id"`
	PricepointID        *int    `json:"pricepoint_id,omitempty" db:"pricepoint_id"`
	PartnerRoleID       *int    `json:"partner_role_id,omitempty" db:"partner_role_id"`
	TimweTransactionID  *string `json:"timwe_transaction_id,omitempty" db:"timwe_transaction_id"`
	TransactionAuthCode *string `json:"transaction_auth_code,omitempty" db:"transaction_auth_code"`
	TimweStatus         *string `json:"timwe_status,omitempty" db:"timwe_status"`

	// Charge tracking (for conversion postbacks)
	ChargedAt              *time.Time `json:"charged_at,omitempty" db:"charged_at"`
	ChargePayout           *string    `json:"charge_payout,omitempty" db:"charge_payout"`
	ConversionPostbackSent bool       `json:"conversion_postback_sent" db:"conversion_postback_sent"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CreateTransactionRequest represents the request to create a new transaction
type CreateTransactionRequest struct {
	CampaignSlug string `json:"campaign_slug" binding:"required"`
	MSISDN       string `json:"msisdn" binding:"required"`

	// Attribution (will be normalized by provider)
	Provider        *string           `json:"provider,omitempty"`
	ClickID         *string           `json:"click_id,omitempty"`
	AttributionData map[string]string `json:"attribution_data,omitempty"`

	// Consent
	ConsentChecked bool `json:"consent_checked"`

	// Request metadata (optional, can be extracted from headers)
	IPAddress *string `json:"ip_address,omitempty"`
	UserAgent *string `json:"user_agent,omitempty"`

	// Header Enrichment context (populated from middleware)
	HESource   *HESource `json:"-"` // Not from JSON, set by handler
	HEMSISDN   *string   `json:"-"` // Not from JSON, set by handler
	HEOperator *string   `json:"-"` // Not from JSON, set by handler
}

// CreateTransactionResponse represents the response after creating a transaction
type CreateTransactionResponse struct {
	TransactionID uuid.UUID              `json:"transaction_id"`
	CorrelationID uuid.UUID              `json:"correlation_id"`
	Status        TransactionStatus      `json:"status"`
	NextAction    NextAction             `json:"next_action"`
	Payload       map[string]interface{} `json:"payload"`
}

// ConfirmTransactionRequest represents the request to confirm a transaction (OTP flow)
type ConfirmTransactionRequest struct {
	TransactionID uuid.UUID `json:"transaction_id" binding:"required"`
	AuthCode      string    `json:"auth_code" binding:"required"`
}

// TransactionStatusResponse represents the current status of a transaction
type TransactionStatusResponse struct {
	TransactionID uuid.UUID              `json:"transaction_id"`
	Status        TransactionStatus      `json:"status"`
	NextAction    *NextAction            `json:"next_action,omitempty"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
}

// Attribution represents normalized attribution data
type Attribution struct {
	Provider     string
	ClickID      string
	PubID        string
	Sub1         string
	Sub2         string
	Sub3         string
	CampaignSlug string
	Creative     string
	Source       string
}
