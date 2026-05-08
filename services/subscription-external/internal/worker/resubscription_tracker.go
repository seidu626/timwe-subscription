package worker

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CheckpointData represents a processing checkpoint for recovery
type CheckpointData struct {
	ID                  int        `json:"id"`
	BatchID             string     `json:"batch_id"`
	TotalCount          int        `json:"total_count"`
	ProcessedCount      int        `json:"processed_count"`
	SuccessCount        int        `json:"success_count"`
	FailureCount        int        `json:"failure_count"`
	LastProcessedID     int        `json:"last_processed_id"`
	LastProcessedMSISDN string     `json:"last_processed_msisdn"`
	Status              string     `json:"status"`
	StartedAt           time.Time  `json:"started_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// ResubscriptionTracker handles tracking and checkpointing for resubscription processing
type ResubscriptionTracker interface {
	InitializeBatch(totalCount int) error
	CheckIfProcessed(msisdn string, productID int) (bool, error)
	RecordAttempt(msisdn string, productID int, subscriptionID int) error
	UpdateResult(msisdn string, productID int, success bool, errorMessage string) error
	SaveCheckpoint(subscriptionID int, msisdn string) error
	LoadCheckpoint() (*CheckpointData, error)
	MarkCompleted() error
	GetStats() *CheckpointData
	LogProgress()
}

// DatabaseResubscriptionTracker implements ResubscriptionTracker using database storage
type DatabaseResubscriptionTracker struct {
	db       *sql.DB
	batchID  string
	interval int
	logger   *zap.Logger
	mu       sync.RWMutex
	stats    *CheckpointData
}

// NewDatabaseResubscriptionTracker creates a new tracker instance
func NewDatabaseResubscriptionTracker(db *sql.DB, batchID string, interval int, logger *zap.Logger) *DatabaseResubscriptionTracker {
	return &DatabaseResubscriptionTracker{
		db:       db,
		batchID:  batchID,
		interval: interval,
		logger:   logger,
		stats:    &CheckpointData{BatchID: batchID, Status: "pending"},
	}
}

// InitializeBatch creates a new batch tracking record
func (t *DatabaseResubscriptionTracker) InitializeBatch(totalCount int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	query := `
		INSERT INTO resubscription_checkpoints 
		(batch_id, total_count, processed_count, success_count, failure_count, status, started_at)
		VALUES ($1, $2, 0, 0, 0, 'in_progress', NOW())
		ON CONFLICT (batch_id) 
		DO UPDATE SET 
			total_count = $2,
			processed_count = 0,
			success_count = 0,
			failure_count = 0,
			status = 'in_progress',
			started_at = NOW(),
			updated_at = NOW()
	`

	_, err := t.db.Exec(query, t.batchID, totalCount)
	if err != nil {
		return fmt.Errorf("failed to initialize batch: %w", err)
	}

	t.stats.TotalCount = totalCount
	t.stats.Status = "in_progress"
	t.stats.StartedAt = time.Now()

	t.logger.Info("Batch initialized",
		zap.String("batchID", t.batchID),
		zap.Int("totalCount", totalCount))

	return nil
}

// CheckIfProcessed checks if a subscription has already been processed
func (t *DatabaseResubscriptionTracker) CheckIfProcessed(msisdn string, productID int) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM resubscription_tracking 
			WHERE msisdn = $1 
			AND product_id = $2 
			AND process_batch_id = $3
			AND resubscribe_status IN ('success', 'in_progress')
		)
	`
	var exists bool
	err := t.db.QueryRow(query, msisdn, productID, t.batchID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if processed: %w", err)
	}
	return exists, nil
}

// RecordAttempt records a resubscription attempt
func (t *DatabaseResubscriptionTracker) RecordAttempt(msisdn string, productID int, subscriptionID int) error {
	query := `
		INSERT INTO resubscription_tracking 
		(subscription_id, msisdn, product_id, process_batch_id, resubscribe_status, created_at)
		VALUES ($1, $2, $3, $4, 'in_progress', NOW())
		ON CONFLICT (subscription_id, process_batch_id) 
		DO UPDATE SET 
			resubscribe_status = 'in_progress',
			updated_at = NOW()
	`
	_, err := t.db.Exec(query, subscriptionID, msisdn, productID, t.batchID)
	if err != nil {
		return fmt.Errorf("failed to record attempt: %w", err)
	}
	return nil
}

// UpdateResult updates the result of a resubscription attempt
func (t *DatabaseResubscriptionTracker) UpdateResult(msisdn string, productID int, success bool, errorMessage string) error {
	status := "success"
	if !success {
		status = "failed"
	}

	query := `
		UPDATE resubscription_tracking
		SET resubscribe_status = $1,
			error_message = $2,
			resubscribe_at = NOW(),
			updated_at = NOW()
		WHERE msisdn = $3 
		AND product_id = $4 
		AND process_batch_id = $5
	`
	_, err := t.db.Exec(query, status, errorMessage, msisdn, productID, t.batchID)
	if err != nil {
		return fmt.Errorf("failed to update result: %w", err)
	}

	// Update local stats
	t.mu.Lock()
	defer t.mu.Unlock()

	if success {
		t.stats.SuccessCount++
	} else {
		t.stats.FailureCount++
	}

	return nil
}

// SaveCheckpoint saves the current processing state
func (t *DatabaseResubscriptionTracker) SaveCheckpoint(subscriptionID int, msisdn string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	query := `
		UPDATE resubscription_checkpoints 
		SET processed_count = $1,
			success_count = $2,
			failure_count = $3,
			last_processed_id = $4,
			last_processed_msisdn = $5,
			updated_at = NOW()
		WHERE batch_id = $6
	`
	_, err := t.db.Exec(query,
		t.stats.ProcessedCount,
		t.stats.SuccessCount,
		t.stats.FailureCount,
		subscriptionID,
		msisdn,
		t.batchID)

	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	t.stats.LastProcessedID = subscriptionID
	t.stats.LastProcessedMSISDN = msisdn
	t.stats.UpdatedAt = time.Now()

	return nil
}

// LoadCheckpoint loads the current processing state
func (t *DatabaseResubscriptionTracker) LoadCheckpoint() (*CheckpointData, error) {
	query := `
		SELECT processed_count, success_count, failure_count, 
			   last_processed_id, last_processed_msisdn, status, started_at
		FROM resubscription_checkpoints
		WHERE batch_id = $1 AND status = 'in_progress'
	`
	var cp CheckpointData
	err := t.db.QueryRow(query, t.batchID).Scan(
		&cp.ProcessedCount, &cp.SuccessCount, &cp.FailureCount,
		&cp.LastProcessedID, &cp.LastProcessedMSISDN, &cp.Status, &cp.StartedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No checkpoint found
		}
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	// Update local stats
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.ProcessedCount = cp.ProcessedCount
	t.stats.SuccessCount = cp.SuccessCount
	t.stats.FailureCount = cp.FailureCount
	t.stats.LastProcessedID = cp.LastProcessedID
	t.stats.LastProcessedMSISDN = cp.LastProcessedMSISDN
	t.stats.Status = cp.Status
	t.stats.StartedAt = cp.StartedAt

	return &cp, nil
}

// MarkCompleted marks the batch as completed
func (t *DatabaseResubscriptionTracker) MarkCompleted() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	query := `
		UPDATE resubscription_checkpoints 
		SET status = 'completed',
			completed_at = NOW(),
			updated_at = NOW()
		WHERE batch_id = $1
	`
	_, err := t.db.Exec(query, t.batchID)
	if err != nil {
		return fmt.Errorf("failed to mark completed: %w", err)
	}

	t.stats.Status = "completed"
	now := time.Now()
	t.stats.CompletedAt = &now

	t.logger.Info("Batch marked as completed",
		zap.String("batchID", t.batchID),
		zap.Int("totalProcessed", t.stats.ProcessedCount),
		zap.Int("successCount", t.stats.SuccessCount),
		zap.Int("failureCount", t.stats.FailureCount))

	return nil
}

// GetStats returns the current processing statistics
func (t *DatabaseResubscriptionTracker) GetStats() *CheckpointData {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := *t.stats
	return &stats
}

// LogProgress logs the current processing progress
func (t *DatabaseResubscriptionTracker) LogProgress() {
	stats := t.GetStats()

	if stats.TotalCount > 0 {
		progress := float64(stats.ProcessedCount) / float64(stats.TotalCount) * 100
		successRate := float64(0)
		if stats.ProcessedCount > 0 {
			successRate = float64(stats.SuccessCount) / float64(stats.ProcessedCount) * 100
		}

		t.logger.Info("Processing progress",
			zap.String("batchID", stats.BatchID),
			zap.Int("processed", stats.ProcessedCount),
			zap.Int("total", stats.TotalCount),
			zap.Float64("progress", progress),
			zap.Int("success", stats.SuccessCount),
			zap.Int("failed", stats.FailureCount),
			zap.Float64("successRate", successRate))
	}
}

// IncrementProcessed increments the processed count
func (t *DatabaseResubscriptionTracker) IncrementProcessed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.ProcessedCount++
}
