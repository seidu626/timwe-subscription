package domain

import (
	"time"

	"github.com/google/uuid"
)

// OutboundClickStatus represents the status of an outbound click
type OutboundClickStatus string

const (
	OutboundClickStatusCreated   OutboundClickStatus = "CREATED"
	OutboundClickStatusRedirected OutboundClickStatus = "REDIRECTED"
	OutboundClickStatusConverted OutboundClickStatus = "CONVERTED"
	OutboundClickStatusExpired   OutboundClickStatus = "EXPIRED"
)

// OutboundClick represents a server-generated click for outbound redirect flow
type OutboundClick struct {
	ClickID        uuid.UUID           `json:"click_id" db:"click_id"`
	Partner        string              `json:"partner" db:"partner"`
	CampaignSlug   *string             `json:"campaign_slug,omitempty" db:"campaign_slug"`
	OfferProductID *int                `json:"offer_product_id,omitempty" db:"offer_product_id"`
	DestKey        string              `json:"dest_key" db:"dest_key"`
	DestURL        string              `json:"dest_url" db:"dest_url"`
	QueryParams    map[string]string   `json:"query_params,omitempty" db:"query_params"`
	ReferrerDomain *string             `json:"referrer_domain,omitempty" db:"referrer_domain"`
	IPHash         *string             `json:"ip_hash,omitempty" db:"ip_hash"`
	UserAgentHash  *string             `json:"user_agent_hash,omitempty" db:"user_agent_hash"`
	Status         OutboundClickStatus `json:"status" db:"status"`
	CreatedAt      time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at" db:"updated_at"`
}

// ClickOutDestination represents an allowlisted destination for click-out redirects
type ClickOutDestination struct {
	Key            string            `json:"key"`              // e.g. "mobplus_track", "landing_web"
	BaseURL        string            `json:"base_url"`         // e.g. "https://mobplus-track.example.com/click"
	ClickIDParam   string            `json:"click_id_param"`   // e.g. "txid" for Mobplus, "click_id" for others
	PassthroughParams []string       `json:"passthrough_params,omitempty"` // params to copy from request
}

// ClickOutConfig holds the configuration for click-out destinations
type ClickOutConfig struct {
	Destinations map[string]ClickOutDestination `json:"destinations"` // keyed by dest_key
}
