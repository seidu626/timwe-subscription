// Enhanced domain structures for resubscription
// File: internal/domain/resubscription.go

package domain

import "time"

// EnhancedBackfillRequest represents an enhanced resubscription request
type EnhancedBackfillRequest struct {
	// Basic fields from original BackfillRequest
	Telco         string   `json:"telco"`
	EntryChannel  string   `json:"entry_channel"`
	EntryChannels []string `json:"entry_channels"`
	ProductIds    []string `json:"product_ids"`
	MSISDNS       []string `json:"msisdns,omitempty"`
	StartIndex    int      `json:"start_index"`
	EndIndex      int      `json:"end_index"`

	// Enhanced features
	BatchID             string        `json:"batch_id,omitempty"`
	UseChargingFailures bool          `json:"use_charging_failures"`
	BatchSize           int           `json:"batch_size"`
	MaxWorkers          int           `json:"max_workers"`
	RateLimit           int           `json:"rate_limit_per_second"`
	CheckpointInterval  int           `json:"checkpoint_interval"`
	ForceReprocess      bool          `json:"force_reprocess"`
	DryRun              bool          `json:"dry_run"`
	StopSignal          chan struct{} `json:"-"`

	// Entry channel rotation
	channelIndex int
}

// ChargingFailedSubscription represents a subscription with charging issues
type ChargingFailedSubscription struct {
	ID                    int        `json:"id"`
	MSISDN                string     `json:"msisdn"`
	ProductID             int        `json:"product_id"`
	EntryChannel          string     `json:"entry_channel"`
	Status                string     `json:"status"`
	ChargingFailureCount  int        `json:"charging_failure_count"`
	LastChargingFailureAt *time.Time `json:"last_charging_failure_at"`
	LastResubscribeAt     *time.Time `json:"last_resubscribe_at"`
	CreatedAt             time.Time  `json:"created_at"`
}

// ResubscriptionStats represents processing statistics
type ResubscriptionStats struct {
	BatchID        string     `json:"batch_id"`
	TotalCount     int64      `json:"total_count"`
	ProcessedCount int64      `json:"processed_count"`
	SuccessCount   int64      `json:"success_count"`
	FailureCount   int64      `json:"failure_count"`
	SkippedCount   int64      `json:"skipped_count"`
	ErrorRate      float64    `json:"error_rate"`
	ProcessingRate float64    `json:"processing_rate"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	Duration       float64    `json:"duration_seconds"`
}

// ResubscriptionError represents an error during processing
type ResubscriptionError struct {
	MSISDN       string    `json:"msisdn"`
	ProductID    int       `json:"product_id"`
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	RetryCount   int       `json:"retry_count"`
	Timestamp    time.Time `json:"timestamp"`
}

// GetNextEntryChannel returns the next entry channel in rotation
func (r *EnhancedBackfillRequest) GetNextEntryChannel() string {
	if len(r.EntryChannels) == 0 {
		if r.EntryChannel != "" {
			return r.EntryChannel
		}
		return "USSD" // Default
	}

	channel := r.EntryChannels[r.channelIndex%len(r.EntryChannels)]
	r.channelIndex++
	return channel
}

// ResubscriptionResponse represents the response for resubscription status
type ResubscriptionResponse struct {
	JobID   string                `json:"job_id"`
	BatchID string                `json:"batch_id"`
	Status  string                `json:"status"`
	Stats   *ResubscriptionStats  `json:"stats,omitempty"`
	Errors  []ResubscriptionError `json:"errors,omitempty"`
	Message string                `json:"message"`
}

// BatchConfiguration represents configuration for batch processing
type BatchConfiguration struct {
	MaxRetries              int           `json:"max_retries"`
	RetryDelay              time.Duration `json:"retry_delay"`
	CircuitBreakerThreshold float64       `json:"circuit_breaker_threshold"`
	AdaptiveRateLimit       bool          `json:"adaptive_rate_limit"`
	PauseWindows            []PauseWindow `json:"pause_windows"`
	Timezone                string        `json:"timezone"`
}

// PauseWindow represents a time window to pause processing
type PauseWindow struct {
	Start string `json:"start"` // HH:MM format
	End   string `json:"end"`   // HH:MM format
}

// CheckpointData represents checkpoint information
type CheckpointData struct {
	BatchID             string    `json:"batch_id"`
	TotalCount          int       `json:"total_count"`
	ProcessedCount      int       `json:"processed_count"`
	SuccessCount        int       `json:"success_count"`
	FailureCount        int       `json:"failure_count"`
	LastProcessedID     int       `json:"last_processed_id"`
	LastProcessedMSISDN string    `json:"last_processed_msisdn"`
	Status              string    `json:"status"`
	StartedAt           time.Time `json:"started_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
