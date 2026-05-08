package domain

import "time"

type MessageSeries struct {
	ID             int64     `json:"id"`
	TenantID       *string   `json:"tenant_id,omitempty"`
	ChannelID      *string   `json:"channel_id,omitempty"`
	PartnerRoleID  int       `json:"partner_role_id"`
	ProductID      int       `json:"product_id"`
	Name           string    `json:"name"`
	Mode           string    `json:"mode"`
	ContentVersion int       `json:"content_version"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

type ScheduleRule struct {
	SeriesID      int64     `json:"series_id"`
	RuleKind      string    `json:"rule_kind"`
	PreferredTime time.Time `json:"preferred_time"`
	DaysOfWeek    int       `json:"days_of_week"`
	NDays         int       `json:"n_days"`
	SendStartTime time.Time `json:"send_start_time"`
	SendEndTime   time.Time `json:"send_end_time"`
	Timezone      string    `json:"timezone"`
	MaxPerDay     int       `json:"max_per_day"`
	CatchupMode   string    `json:"catchup_mode"`
}

type ContentItem struct {
	ID             int64     `json:"id"`
	TenantID       *string   `json:"tenant_id,omitempty"`
	ChannelID      *string   `json:"channel_id,omitempty"`
	SeriesID       int64     `json:"series_id"`
	ContentVersion int       `json:"content_version"`
	SeqNo          int       `json:"seq_no"`
	MessageText    string    `json:"message_text"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

type Subscription struct {
	ID                 int64
	TenantID           *string
	ChannelID          *string
	PartnerRoleID      int
	ProductID          int
	UserIdentifier     string
	UserIdentifierType string
	EntryChannel       string
	StartDate          time.Time
}

type SubscriptionMessageState struct {
	SubscriptionID int64
	TenantID       *string
	ChannelID      *string
	SeriesID       int64
	CursorSeq      int
	NextSendAt     time.Time
	LastSentAt     *time.Time
}

type OutboxJob struct {
	JobID          string
	IdempotencyKey string
	TenantID       *string
	ChannelID      *string
	SubscriptionID int64
	SeriesID       int64
	ContentItemID  int64
	PlannedSendAt  time.Time
	Status         string
	Attempt        int
	SentAt         *time.Time
	ProcessedAt    *time.Time
	LastError      *string
}

type MissingState struct {
	SubscriptionID int64
	TenantID       *string
	ChannelID      *string
	SeriesID       int64
	StartDate      time.Time
	Rule           ScheduleRule
}

type DueState struct {
	SubscriptionID int64
	TenantID       *string
	ChannelID      *string
	SeriesID       int64
	CursorSeq      int
	NextSendAt     time.Time
}
