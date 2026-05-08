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
