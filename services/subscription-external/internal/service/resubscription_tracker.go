// resubscription_tracker.go
// New file: internal/service/resubscription_tracker.go

package service

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ResubscriptionTracker handles tracking and deduplication of resubscription attempts
type ResubscriptionTracker struct {
	db              *sql.DB
	logger          *zap.Logger
	batchID         string
	checkpointMutex sync.Mutex
	stats           *ProcessingStats
}

// ProcessingStats tracks real-time processing statistics
type ProcessingStats struct {
	TotalCount     int64
	ProcessedCount int64
	SuccessCount   int64
	FailureCount   int64
	SkippedCount   int64
	StartTime      time.Time
	LastCheckpoint time.Time
}

// CheckpointData represents a processing checkpoint
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

// NewResubscriptionTracker creates a new tracker instance
func NewResubscriptionTracker(db *sql.DB, logger *zap.Logger, batchID string) *ResubscriptionTracker {
	if batchID == "" {
		batchID = uuid.New().String()
	}

	return &ResubscriptionTracker{
		db:      db,
		logger:  logger,
		batchID: batchID,
		stats: &ProcessingStats{
			StartTime:      time.Now(),
			LastCheckpoint: time.Now(),
		},
	}
}

// InitializeBatch creates a new processing batch
func (rt *ResubscriptionTracker) InitializeBatch(totalCount int) error {
	query := `
        INSERT INTO resubscription_checkpoints 
        (batch_id, total_count, status, started_at)
        VALUES ($1, $2, 'in_progress', NOW())
        ON CONFLICT (batch_id) DO UPDATE
        SET total_count = EXCLUDED.total_count,
            status = 'in_progress',
            started_at = NOW()
    `

	_, err := rt.db.Exec(query, rt.batchID, totalCount)
	if err != nil {
		return fmt.Errorf("failed to initialize batch: %w", err)
	}

	atomic.StoreInt64(&rt.stats.TotalCount, int64(totalCount))
	rt.logger.Info("Initialized resubscription batch",
		zap.String("batchID", rt.batchID),
		zap.Int("totalCount", totalCount))

	return nil
}

// CheckIfProcessed checks if an MSISDN has already been processed
func (rt *ResubscriptionTracker) CheckIfProcessed(msisdn string, productID int) (bool, error) {
	query := `
        SELECT EXISTS(
            SELECT 1 FROM resubscription_tracking
            WHERE msisdn = $1 
            AND product_id = $2
            AND process_batch_id = $3
            AND resubscribe_status IN ('success', 'in_progress')
            AND created_at > NOW() - INTERVAL '24 hours'
        )
    `

	var exists bool
	err := rt.db.QueryRow(query, msisdn, productID, rt.batchID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if processed: %w", err)
	}

	return exists, nil
}

// RecordAttempt records a resubscription attempt
func (rt *ResubscriptionTracker) RecordAttempt(msisdn string, productID int, subscriptionID int) error {
	query := `
        INSERT INTO resubscription_tracking 
        (subscription_id, msisdn, product_id, process_batch_id, resubscribe_status, created_at)
        VALUES ($1, $2, $3, $4, 'in_progress', NOW())
        ON CONFLICT (subscription_id, process_batch_id) 
        DO UPDATE SET 
            attempt_number = resubscription_tracking.attempt_number + 1,
            updated_at = NOW()
    `

	_, err := rt.db.Exec(query, subscriptionID, msisdn, productID, rt.batchID)
	if err != nil {
		return fmt.Errorf("failed to record attempt: %w", err)
	}

	return nil
}

// UpdateResult updates the result of a resubscription attempt
func (rt *ResubscriptionTracker) UpdateResult(msisdn string, productID int, success bool, errorMsg string) error {
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
        AND resubscribe_status = 'in_progress'
    `

	_, err := rt.db.Exec(query, status, errorMsg, msisdn, productID, rt.batchID)
	if err != nil {
		return fmt.Errorf("failed to update result: %w", err)
	}

	// Update stats
	if success {
		atomic.AddInt64(&rt.stats.SuccessCount, 1)
	} else {
		atomic.AddInt64(&rt.stats.FailureCount, 1)
	}
	atomic.AddInt64(&rt.stats.ProcessedCount, 1)

	return nil
}

// SaveCheckpoint saves the current processing state
func (rt *ResubscriptionTracker) SaveCheckpoint(lastProcessedID int, lastMSISDN string) error {
	rt.checkpointMutex.Lock()
	defer rt.checkpointMutex.Unlock()

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

	_, err := rt.db.Exec(query,
		atomic.LoadInt64(&rt.stats.ProcessedCount),
		atomic.LoadInt64(&rt.stats.SuccessCount),
		atomic.LoadInt64(&rt.stats.FailureCount),
		lastProcessedID,
		lastMSISDN,
		rt.batchID,
	)

	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	rt.stats.LastCheckpoint = time.Now()
	return nil
}

// LoadCheckpoint loads a previous checkpoint for recovery
func (rt *ResubscriptionTracker) LoadCheckpoint() (*CheckpointData, error) {
	query := `
        SELECT batch_id, total_count, processed_count, success_count, failure_count,
               last_processed_id, last_processed_msisdn, status, started_at, updated_at
        FROM resubscription_checkpoints
        WHERE batch_id = $1
    `

	var cp CheckpointData
	err := rt.db.QueryRow(query, rt.batchID).Scan(
		&cp.BatchID,
		&cp.TotalCount,
		&cp.ProcessedCount,
		&cp.SuccessCount,
		&cp.FailureCount,
		&cp.LastProcessedID,
		&cp.LastProcessedMSISDN,
		&cp.Status,
		&cp.StartedAt,
		&cp.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	// Restore stats from checkpoint
	atomic.StoreInt64(&rt.stats.TotalCount, int64(cp.TotalCount))
	atomic.StoreInt64(&rt.stats.ProcessedCount, int64(cp.ProcessedCount))
	atomic.StoreInt64(&rt.stats.SuccessCount, int64(cp.SuccessCount))
	atomic.StoreInt64(&rt.stats.FailureCount, int64(cp.FailureCount))
	rt.stats.StartTime = cp.StartedAt
	rt.stats.LastCheckpoint = cp.UpdatedAt

	return &cp, nil
}

// MarkCompleted marks the batch as completed
func (rt *ResubscriptionTracker) MarkCompleted() error {
	query := `
        UPDATE resubscription_checkpoints
        SET status = 'completed',
            completed_at = NOW(),
            updated_at = NOW()
        WHERE batch_id = $1
    `

	_, err := rt.db.Exec(query, rt.batchID)
	if err != nil {
		return fmt.Errorf("failed to mark completed: %w", err)
	}

	return nil
}

// GetStats returns current processing statistics
func (rt *ResubscriptionTracker) GetStats() ProcessingStats {
	return ProcessingStats{
		TotalCount:     atomic.LoadInt64(&rt.stats.TotalCount),
		ProcessedCount: atomic.LoadInt64(&rt.stats.ProcessedCount),
		SuccessCount:   atomic.LoadInt64(&rt.stats.SuccessCount),
		FailureCount:   atomic.LoadInt64(&rt.stats.FailureCount),
		SkippedCount:   atomic.LoadInt64(&rt.stats.SkippedCount),
		StartTime:      rt.stats.StartTime,
		LastCheckpoint: rt.stats.LastCheckpoint,
	}
}

// CalculateProgress calculates current progress and estimates
func (rt *ResubscriptionTracker) CalculateProgress() map[string]interface{} {
	stats := rt.GetStats()
	elapsed := time.Since(stats.StartTime).Seconds()

	processedCount := float64(stats.ProcessedCount)
	totalCount := float64(stats.TotalCount)

	if processedCount == 0 {
		return map[string]interface{}{
			"progress_pct":              0,
			"rate_per_second":           0,
			"estimated_remaining_hours": "unknown",
		}
	}

	progressPct := (processedCount / totalCount) * 100
	ratePerSecond := processedCount / elapsed
	remainingCount := totalCount - processedCount
	estimatedSecondsRemaining := remainingCount / ratePerSecond

	return map[string]interface{}{
		"batch_id":                  rt.batchID,
		"total_count":               stats.TotalCount,
		"processed_count":           stats.ProcessedCount,
		"success_count":             stats.SuccessCount,
		"failure_count":             stats.FailureCount,
		"skipped_count":             stats.SkippedCount,
		"progress_pct":              fmt.Sprintf("%.2f", progressPct),
		"rate_per_second":           fmt.Sprintf("%.2f", ratePerSecond),
		"elapsed_hours":             fmt.Sprintf("%.2f", elapsed/3600),
		"estimated_remaining_hours": fmt.Sprintf("%.2f", estimatedSecondsRemaining/3600),
		"error_rate_pct":            fmt.Sprintf("%.2f", (float64(stats.FailureCount)/processedCount)*100),
	}
}

// LogProgress logs current progress
func (rt *ResubscriptionTracker) LogProgress() {
	progress := rt.CalculateProgress()
	rt.logger.Info("Resubscription progress",
		zap.Any("progress", progress))
}
