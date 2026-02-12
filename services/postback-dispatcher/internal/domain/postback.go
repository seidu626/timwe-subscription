package domain

import (
	"time"

	"github.com/google/uuid"
)

// PostbackEvent represents the type of postback event
type PostbackEvent string

const (
	PostbackEventSubscribed PostbackEvent = "subscribed"
	PostbackEventFailed     PostbackEvent = "failed"
	PostbackEventCancelled  PostbackEvent = "cancelled"
	PostbackEventConversion PostbackEvent = "conversion"
)

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
	ID                  uuid.UUID      `json:"id" db:"id"`
	TransactionID       uuid.UUID      `json:"transaction_id" db:"transaction_id"`
	Event               PostbackEvent  `json:"event" db:"event"`
	Provider            string         `json:"provider" db:"provider"`
	URLTemplateRendered string         `json:"url_template_rendered" db:"url_template_rendered"`
	HTTPMethod          string         `json:"http_method" db:"http_method"`
	Headers             string         `json:"headers" db:"headers"`
	Body                *string        `json:"body,omitempty" db:"body"`

	// Retry tracking
	AttemptCount int            `json:"attempt_count" db:"attempt_count"`
	MaxAttempts  int            `json:"max_attempts" db:"max_attempts"`
	NextRetryAt  *time.Time     `json:"next_retry_at,omitempty" db:"next_retry_at"`
	Status       PostbackStatus `json:"status" db:"status"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PostbackAttempt represents a single postback attempt
type PostbackAttempt struct {
	ID            uuid.UUID `json:"id" db:"id"`
	OutboxID      uuid.UUID `json:"outbox_id" db:"outbox_id"`
	AttemptNumber int       `json:"attempt_number" db:"attempt_number"`
	HTTPStatus    *int      `json:"http_status,omitempty" db:"http_status"`
	ResponseBody  *string   `json:"response_body,omitempty" db:"response_body"`
	ErrorMessage  *string   `json:"error_message,omitempty" db:"error_message"`
	DurationMs    *int      `json:"duration_ms,omitempty" db:"duration_ms"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
