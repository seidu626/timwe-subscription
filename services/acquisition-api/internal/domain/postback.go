package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PostbackEvent represents the type of postback event
type PostbackEvent string

const (
	PostbackEventSubscribed  PostbackEvent = "subscribed"
	PostbackEventFailed      PostbackEvent = "failed"
	PostbackEventCancelled   PostbackEvent = "cancelled"
	PostbackEventConversion  PostbackEvent = "conversion" // Fired on charge success (Mobplus requirement)
)

// PostbackTemplate defines a configurable postback URL template
type PostbackTemplate struct {
	Method  string            `json:"method"`  // HTTP method: GET or POST
	URL     string            `json:"url"`     // URL template with placeholders like {click_id}
	Headers map[string]string `json:"headers"` // Optional headers
}

// PostbackRules defines per-event postback templates for each provider
// Structure: {"conversion": {"mobplus": {...}}, "subscribed": {"generic": {...}}}
type PostbackRules map[string]map[string]PostbackTemplate

// PostbackContext contains all the data available for template rendering
type PostbackContext struct {
	ClickID         string
	TransactionID   string
	CampaignSlug    string
	MSISDN          string
	MSISDNHash      string // SHA256 hash of MSISDN for privacy
	Provider        string
	Status          string
	Payout          string // Optional payout amount
	OfferID         string
	CampaignID      string
	AdvID           string
	AffID           string
	PubID           string
	Sub1            string
	Sub2            string
	Sub3            string
}

// NewPostbackContext creates a PostbackContext from transaction and attribution data
func NewPostbackContext(tx *AcquisitionTransaction, attr *Attribution) *PostbackContext {
	ctx := &PostbackContext{
		TransactionID: tx.ID.String(),
		CampaignSlug:  tx.CampaignSlug,
		MSISDN:        tx.MSISDN,
		Status:        string(tx.Status),
	}

	// Hash MSISDN for privacy-safe postbacks
	hash := sha256.Sum256([]byte(tx.MSISDN))
	ctx.MSISDNHash = hex.EncodeToString(hash[:])

	if tx.ClickID != nil {
		ctx.ClickID = *tx.ClickID
	}
	if tx.AdProvider != nil {
		ctx.Provider = *tx.AdProvider
	}

	if attr != nil {
		if ctx.ClickID == "" {
			ctx.ClickID = attr.ClickID
		}
		ctx.PubID = attr.PubID
		ctx.Sub1 = attr.Sub1
		ctx.Sub2 = attr.Sub2
		ctx.Sub3 = attr.Sub3
	}

	return ctx
}

// RenderURL replaces template placeholders with actual values
func (c *PostbackContext) RenderURL(template string) string {
	replacements := map[string]string{
		"{click_id}":       c.ClickID,
		"{transaction_id}": c.TransactionID,
		"{campaign_slug}":  c.CampaignSlug,
		"{msisdn_hash}":    c.MSISDNHash,
		"{provider}":       c.Provider,
		"{status}":         c.Status,
		"{payout}":         c.Payout,
		"{offer_id}":       c.OfferID,
		"{campaign_id}":    c.CampaignID,
		"{adv_id}":         c.AdvID,
		"{aff_id}":         c.AffID,
		"{pub_id}":         c.PubID,
		"{sub1}":           c.Sub1,
		"{sub2}":           c.Sub2,
		"{sub3}":           c.Sub3,
		// Mobplus-specific alias
		"{txid}":           c.ClickID,
	}

	result := template
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// PostbackStatus represents the status of a postback
type PostbackStatus string

const (
	PostbackStatusPending    PostbackStatus = "PENDING"
	PostbackStatusProcessing PostbackStatus = "PROCESSING"
	PostbackStatusSuccess    PostbackStatus = "SUCCESS"
	PostbackStatusFailed     PostbackStatus = "FAILED"
	PostbackStatusDLQ        PostbackStatus = "DLQ"
)

// PostbackOutbox represents a queued postback
type PostbackOutbox struct {
	ID                    uuid.UUID      `json:"id" db:"id"`
	TransactionID         uuid.UUID      `json:"transaction_id" db:"transaction_id"`
	Event                 PostbackEvent   `json:"event" db:"event"`
	Provider              string         `json:"provider" db:"provider"`
	URLTemplateRendered   string         `json:"url_template_rendered" db:"url_template_rendered"`
	HTTPMethod            string         `json:"http_method" db:"http_method"`
	Headers               string         `json:"headers" db:"headers"` // JSON string
	Body                  *string        `json:"body,omitempty" db:"body"` // JSON string
	
	// Retry tracking
	AttemptCount          int            `json:"attempt_count" db:"attempt_count"`
	MaxAttempts           int            `json:"max_attempts" db:"max_attempts"`
	NextRetryAt           *time.Time     `json:"next_retry_at,omitempty" db:"next_retry_at"`
	Status                PostbackStatus `json:"status" db:"status"`
	
	// Timestamps
	CreatedAt             time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at" db:"updated_at"`
}

// PostbackAttempt represents a single postback attempt
type PostbackAttempt struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	OutboxID      uuid.UUID  `json:"outbox_id" db:"outbox_id"`
	AttemptNumber int        `json:"attempt_number" db:"attempt_number"`
	HTTPStatus    *int       `json:"http_status,omitempty" db:"http_status"`
	ResponseBody  *string    `json:"response_body,omitempty" db:"response_body"`
	ErrorMessage  *string    `json:"error_message,omitempty" db:"error_message"`
	DurationMs    *int       `json:"duration_ms,omitempty" db:"duration_ms"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}
