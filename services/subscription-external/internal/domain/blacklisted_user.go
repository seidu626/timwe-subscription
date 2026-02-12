package domain

import (
	"time"
)

// BlacklistedUser represents a user that has been blacklisted
type BlacklistedUser struct {
	ID        int64     `json:"id" db:"id"`
	Msisdn    string    `json:"msisdn" db:"msisdn"`
	Type      string    `json:"type" db:"type"`
	Reason    string    `json:"reason" db:"reason"`
	Source    string    `json:"source" db:"source"`
	RequestID string    `json:"request_id" db:"request_id"`
	PartnerID int       `json:"partner_id" db:"partner_id"`
	ProductID int       `json:"product_id" db:"product_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Status    string    `json:"status" db:"status"`
	Metadata  string    `json:"metadata" db:"metadata"`
}

// BlacklistedUserLog represents a log entry for blacklisted user operations
type BlacklistedUserLog struct {
	ID         int64     `json:"id" db:"id"`
	Msisdn     string    `json:"msisdn" db:"msisdn"`
	Operation  string    `json:"operation" db:"operation"`
	Status     string    `json:"status" db:"status"`
	Error      string    `json:"error" db:"error"`
	RequestID  string    `json:"request_id" db:"request_id"`
	PartnerID  int       `json:"partner_id" db:"partner_id"`
	ProductID  int       `json:"product_id" db:"product_id"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp"`
	Duration   int64     `json:"duration_ms" db:"duration_ms"`
	RetryCount int       `json:"retry_count" db:"retry_count"`
	Metadata   string    `json:"metadata" db:"metadata"`
}

// BlacklistedUserAudit represents an audit trail for blacklisted user operations
type BlacklistedUserAudit struct {
	ID            int64     `json:"id" db:"id"`
	Msisdn        string    `json:"msisdn" db:"msisdn"`
	Action        string    `json:"action" db:"action"`
	PreviousState string    `json:"previous_state" db:"previous_state"`
	NewState      string    `json:"new_state" db:"new_state"`
	UserID        string    `json:"user_id" db:"user_id"`
	IPAddress     string    `json:"ip_address" db:"ip_address"`
	UserAgent     string    `json:"user_agent" db:"user_agent"`
	Timestamp     time.Time `json:"timestamp" db:"timestamp"`
	Reason        string    `json:"reason" db:"reason"`
	Metadata      string    `json:"metadata" db:"metadata"`
}

// BlacklistedUserBatch represents a batch of blacklisted users to process
type BlacklistedUserBatch struct {
	Users        []*BlacklistedUser `json:"users"`
	BatchID      string             `json:"batch_id"`
	CreatedAt    time.Time          `json:"created_at"`
	Status       string             `json:"status"`
	TotalCount   int                `json:"total_count"`
	SuccessCount int                `json:"success_count"`
	FailureCount int                `json:"failure_count"`
	Duration     time.Duration      `json:"duration"`
}

// BlacklistedUserStats represents statistics for blacklisted user operations
type BlacklistedUserStats struct {
	TotalBlacklistedUsers int64            `json:"total_blacklisted_users"`
	TotalInsertions       int64            `json:"total_insertions"`
	TotalCleanups         int64            `json:"total_cleanups"`
	TotalFailures         int64            `json:"total_failures"`
	SuccessRate           float64          `json:"success_rate"`
	AverageOperationTime  float64          `json:"average_operation_time_ms"`
	LastOperation         time.Time        `json:"last_operation"`
	ErrorBreakdown        map[string]int64 `json:"error_breakdown"`
}

// BlacklistedUserFilter represents filters for querying blacklisted users
type BlacklistedUserFilter struct {
	Msisdn    string    `json:"msisdn"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	PartnerID int       `json:"partner_id"`
	ProductID int       `json:"product_id"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Limit     int       `json:"limit"`
	Offset    int       `json:"offset"`
	SortBy    string    `json:"sort_by"`
	SortOrder string    `json:"sort_order"`
}

// BlacklistedUserResponse represents the response from blacklisted user operations
type BlacklistedUserResponse struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Data      *BlacklistedUser       `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BlacklistedUserBatchResponse represents the response from batch operations
type BlacklistedUserBatchResponse struct {
	Success      bool                       `json:"success"`
	Message      string                     `json:"message"`
	BatchID      string                     `json:"batch_id"`
	TotalCount   int                        `json:"total_count"`
	SuccessCount int                        `json:"success_count"`
	FailureCount int                        `json:"failure_count"`
	Duration     time.Duration              `json:"duration"`
	Results      []*BlacklistedUserResponse `json:"results,omitempty"`
	Errors       []string                   `json:"errors,omitempty"`
	Timestamp    time.Time                  `json:"timestamp"`
}
