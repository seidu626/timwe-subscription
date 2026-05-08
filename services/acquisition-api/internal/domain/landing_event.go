package domain

import "time"

// LandingEventType represents the type of landing page event
type LandingEventType string

const (
	EventLandingView LandingEventType = "landing_view"
	EventLandingClick LandingEventType = "landing_click"
	EventFormSubmit  LandingEventType = "form_submit"
)

// IsValid checks if the event type is valid
func (e LandingEventType) IsValid() bool {
	switch e {
	case EventLandingView, EventLandingClick, EventFormSubmit:
		return true
	}
	return false
}

// LandingEvent represents an anonymous landing page event
type LandingEvent struct {
	ID             int64            `json:"id" db:"id"`
	EventType      LandingEventType `json:"event_type" db:"event_type"`
	CampaignSlug   string           `json:"campaign_slug" db:"campaign_slug"`
	ClickID        *string          `json:"click_id,omitempty" db:"click_id"`
	AdProvider     *string          `json:"ad_provider,omitempty" db:"ad_provider"`
	SessionID      *string          `json:"session_id,omitempty" db:"session_id"`
	IPHash         *string          `json:"ip_hash,omitempty" db:"ip_hash"`
	UserAgentHash  *string          `json:"user_agent_hash,omitempty" db:"user_agent_hash"`
	ReferrerDomain *string          `json:"referrer_domain,omitempty" db:"referrer_domain"`
	CreatedAt      time.Time        `json:"created_at" db:"created_at"`
}

// CreateLandingEventRequest is the API request to create a landing event
type CreateLandingEventRequest struct {
	EventType      LandingEventType `json:"event_type" binding:"required"`
	CampaignSlug   string           `json:"campaign_slug" binding:"required"`
	ClickID        *string          `json:"click_id,omitempty"`
	AdProvider     *string          `json:"ad_provider,omitempty"`
	SessionID      *string          `json:"session_id,omitempty"`
	ReferrerDomain *string          `json:"referrer_domain,omitempty"`
}

// Validate validates the request
func (r *CreateLandingEventRequest) Validate() error {
	if r.CampaignSlug == "" {
		return &ValidationError{Field: "campaign_slug", Message: "campaign_slug is required"}
	}
	if !r.EventType.IsValid() {
		return &ValidationError{Field: "event_type", Message: "invalid event_type; must be landing_view, landing_click, or form_submit"}
	}
	return nil
}

// CreateLandingEventResponse is the API response after creating a landing event
type CreateLandingEventResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}
