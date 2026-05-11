package domain

import (
	"encoding/json"
	"time"
)

type TenantStatus string

const (
	TenantStatusActive   TenantStatus = "ACTIVE"
	TenantStatusInactive TenantStatus = "INACTIVE"
)

// AdminTenant represents a tenant managed via platform admin APIs.
type AdminTenant struct {
	ID             string          `json:"id"`
	TenantKey      string          `json:"tenant_key"`
	Name           string          `json:"name"`
	Status         TenantStatus    `json:"status"`
	DefaultCountry string          `json:"default_country"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// TenantCreateInput contains the validated tenant create payload.
type TenantCreateInput struct {
	TenantKey      string
	Name           string
	Status         TenantStatus
	DefaultCountry string
	Metadata       json.RawMessage
}

// TenantUpdateInput contains optional tenant catalog fields for update.
type TenantUpdateInput struct {
	Name           *string
	Status         *TenantStatus
	DefaultCountry *string
	Metadata       *json.RawMessage
}

// TenantListFilter is used to filter and paginate tenant catalog records.
type TenantListFilter struct {
	Limit  int
	Offset int
	Status TenantStatus
	Query  string
}

// AdminProduct represents a product managed via admin APIs.
type AdminProduct struct {
	ID              int       `json:"id"`
	TenantID        string    `json:"tenant_id"`
	ProductID       string    `json:"product_id"`
	Name            string    `json:"name"`
	PricePointID    int       `json:"price_point_id"`
	PricePointValue float64   `json:"price_point_value"`
	ShortCode       string    `json:"short_code"`
	CreatedAt       time.Time `json:"created_at"`
}

// ProductListFilter is used to filter and paginate products.
type ProductListFilter struct {
	TenantID  string
	Limit     int
	Offset    int
	Query     string
	ShortCode string
}

// ProductDependencyCounts contains blocking dependency counts for product deletion.
type ProductDependencyCounts struct {
	CampaignCount     int `json:"campaign_count"`
	SubscriptionCount int `json:"subscription_count"`
}

type ChannelStatus string

const (
	ChannelStatusActive   ChannelStatus = "ACTIVE"
	ChannelStatusInactive ChannelStatus = "INACTIVE"
)

// AdminChannel represents a tenant-owned channel catalog entry.
type AdminChannel struct {
	ID           string        `json:"channel_id"`
	TenantID     string        `json:"tenant_id"`
	ChannelKey   string        `json:"channel_key"`
	Provider     string        `json:"provider"`
	Country      string        `json:"country"`
	Operator     *string       `json:"operator,omitempty"`
	Capabilities []string      `json:"capabilities"`
	Status       ChannelStatus `json:"status"`
	Enabled      bool          `json:"enabled"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

func (c *AdminChannel) IsRoutable() bool {
	return c != nil && c.Status == ChannelStatusActive && len(c.Capabilities) > 0
}

type ChannelCredentialStatus string

const (
	ChannelCredentialStatusActive   ChannelCredentialStatus = "ACTIVE"
	ChannelCredentialStatusInactive ChannelCredentialStatus = "INACTIVE"
)

// AdminChannelCredential is a redacted tenant channel credential reference.
type AdminChannelCredential struct {
	ID                string                  `json:"credential_id"`
	TenantID          string                  `json:"tenant_id"`
	ChannelID         string                  `json:"channel_id"`
	Purpose           string                  `json:"purpose"`
	Version           int                     `json:"version"`
	Status            ChannelCredentialStatus `json:"status"`
	SecretRef         string                  `json:"-"`
	SecretRefDisplay  string                  `json:"redacted_display"`
	SecretFingerprint string                  `json:"-"`
	CreatedBy         *string                 `json:"created_by,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	ActivatedAt       *time.Time              `json:"activated_at,omitempty"`
	DeactivatedAt     *time.Time              `json:"deactivated_at,omitempty"`
}

func (c AdminChannelCredential) String() string {
	return "[REDACTED channel credential]"
}

func (c AdminChannelCredential) GoString() string {
	return c.String()
}

// ChannelCredentialBindInput contains credential material before it is converted to a reference.
type ChannelCredentialBindInput struct {
	ChannelID        string
	Purpose          string
	SecretRef        string
	SecretValue      string
	SecretRefDisplay string
}

// ChannelCredentialListFilter is used to filter and paginate channel credential metadata.
type ChannelCredentialListFilter struct {
	TenantID  string
	ChannelID string
	Purpose   string
	Limit     int
	Offset    int
}

// ChannelCreateInput contains normalized channel create data.
type ChannelCreateInput struct {
	ChannelKey   string
	Provider     string
	Country      string
	Operator     *string
	Capabilities []string
	Enabled      *bool
}

// ChannelListFilter is used to filter and paginate tenant channels.
type ChannelListFilter struct {
	TenantID string
	Limit    int
	Offset   int
	Provider string
	Country  string
	Enabled  *bool
}

// UserbaseRecord represents a row in userbase.
type UserbaseRecord struct {
	ID       int    `json:"id"`
	TenantID string `json:"tenant_id"`
	MSISDN   string `json:"msisdn"`
	Type     string `json:"type"`
}

// UserbaseListFilter is used to filter and paginate userbase records.
type UserbaseListFilter struct {
	TenantID string
	Limit    int
	Offset   int
	MSISDN   string
	Type     string
}

// UserbaseImportJobStatus represents import job status.
type UserbaseImportJobStatus string

const (
	UserbaseImportStatusProcessing UserbaseImportJobStatus = "PROCESSING"
	UserbaseImportStatusCompleted  UserbaseImportJobStatus = "COMPLETED"
	UserbaseImportStatusFailed     UserbaseImportJobStatus = "FAILED"
)

// UserbaseImportJob stores metadata for an import job.
type UserbaseImportJob struct {
	ID          string                  `json:"id"`
	TenantID    string                  `json:"tenant_id"`
	Filename    string                  `json:"filename"`
	Status      UserbaseImportJobStatus `json:"status"`
	TotalRows   int                     `json:"total_rows"`
	SuccessRows int                     `json:"success_rows"`
	FailedRows  int                     `json:"failed_rows"`
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt *time.Time              `json:"completed_at,omitempty"`
	CreatedBy   *string                 `json:"created_by,omitempty"`
}

// UserbaseImportError stores a failed import row and reason.
type UserbaseImportError struct {
	ID           int    `json:"id"`
	TenantID     string `json:"tenant_id"`
	JobID        string `json:"job_id"`
	RowNumber    int    `json:"row_number"`
	RawRow       string `json:"raw_row"`
	ErrorMessage string `json:"error_message"`
}

// UserbaseImportInputRow is an in-memory parsed row before validation.
type UserbaseImportInputRow struct {
	RowNumber int
	MSISDN    string
	Type      string
	RawRow    string
}

// AdminActivityLog stores auditable admin actions.
type AdminActivityLog struct {
	ID         string          `json:"id"`
	TenantID   string          `json:"tenant_id,omitempty"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Action     string          `json:"action"`
	Actor      *string         `json:"actor,omitempty"`
	RequestID  *string         `json:"request_id,omitempty"`
	BeforeJSON json.RawMessage `json:"before_json,omitempty"`
	AfterJSON  json.RawMessage `json:"after_json,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// AdminActivityLogFilter is used to filter and paginate activity logs.
type AdminActivityLogFilter struct {
	TenantID   string
	Limit      int
	Offset     int
	EntityType string
	Action     string
	Actor      string
	From       *time.Time
	To         *time.Time
}
