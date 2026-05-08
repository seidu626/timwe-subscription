package worker

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"bytes"

	"github.com/seidu626/subscription-manager/subscription-external/internal/monitoring"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
	"go.uber.org/zap"
)

// ResubscriptionStatus represents the status of a resubscription attempt
type ResubscriptionStatus string

const (
	StatusPending    ResubscriptionStatus = "pending"
	StatusProcessing ResubscriptionStatus = "processing"
	StatusSuccess    ResubscriptionStatus = "success"
	StatusFailed     ResubscriptionStatus = "failed"
	StatusSkipped    ResubscriptionStatus = "skipped"
	StatusCompleted  ResubscriptionStatus = "completed"
)

// ResubscriptionResult represents the result of a resubscription attempt
type ResubscriptionResult struct {
	SubscriptionID int64                `json:"subscription_id"`
	MSISDN         string               `json:"msisdn"`
	ProductID      int                  `json:"product_id"`
	Status         ResubscriptionStatus `json:"status"`
	AttemptedAt    time.Time            `json:"attempted_at"`
	CompletedAt    *time.Time           `json:"completed_at,omitempty"`
	Error          string               `json:"error,omitempty"`
	RetryCount     int                  `json:"retry_count"`
	Priority       int                  `json:"priority"`
	ProcessingTime time.Duration        `json:"processing_time,omitempty"`
}

// ResubscriptionStats tracks resubscription processing statistics
type ResubscriptionStats struct {
	TotalProcessed    int64         `json:"total_processed"`
	Successful        int64         `json:"successful"`
	Failed            int64         `json:"failed"`
	Skipped           int64         `json:"skipped"`
	SuccessRate       float64       `json:"success_rate"`
	AverageTime       time.Duration `json:"average_time"`
	StartTime         time.Time     `json:"start_time"`
	LastProcessed     time.Time     `json:"last_processed"`
	CurrentBatch      int           `json:"current_batch"`
	TotalBatches      int           `json:"total_batches"`
	EstimatedComplete time.Time     `json:"estimated_complete"`
}

// ProcessingConfig defines processing behavior
type ProcessingConfig struct {
	BatchSize              int           `json:"batch_size"`               // Number of subscriptions per batch
	MaxConcurrency         int           `json:"max_concurrency"`          // Maximum concurrent workers
	RetryAttempts          int           `json:"retry_attempts"`           // Maximum retry attempts
	RetryDelay             time.Duration `json:"retry_delay"`              // Delay between retries
	ProcessingTimeout      time.Duration `json:"processing_timeout"`       // Timeout per subscription
	PriorityProcessing     bool          `json:"priority_processing"`      // Process by priority order
	SkipProcessed          bool          `json:"skip_processed"`           // Skip already processed
	UpdateChargingHealth   bool          `json:"update_charging_health"`   // Update charging health status
	CheckpointInterval     int           `json:"checkpoint_interval"`      // Save checkpoint every N records
	BatchID                string        `json:"batch_id"`                 // Unique batch identifier
	BatchDelay             time.Duration `json:"batch_delay"`              // Delay between batches
	MaxQueueSize           int           `json:"max_queue_size"`           // Maximum items in processing queue
	ProgressReportInterval time.Duration `json:"progress_report_interval"` // How often to report progress
	ResultsBufferSize      int           `json:"results_buffer_size"`      // Buffer size for results
}

// ResubscriptionProcessor handles batch processing of charging failed subscriptions
type ResubscriptionProcessor struct {
	repo         repository.SubscriptionRepositoryInterface
	service      *service.SubscriptionService
	monitor      *monitoring.ChargingFailureMonitor
	logger       *zap.Logger
	config       *ProcessingConfig
	stats        *ResubscriptionStats
	tracker      ResubscriptionTracker
	results      []*ResubscriptionResult
	mu           sync.RWMutex
	isRunning    bool
	stopChan     chan struct{}
	progressChan chan *ResubscriptionStats
	resultsChan  chan *ResubscriptionResult
}

// NewResubscriptionProcessor creates a new processor instance
func NewResubscriptionProcessor(
	repo repository.SubscriptionRepositoryInterface,
	service *service.SubscriptionService,
	monitor *monitoring.ChargingFailureMonitor,
	logger *zap.Logger,
	config *ProcessingConfig,
) *ResubscriptionProcessor {

	if config == nil {
		config = &ProcessingConfig{
			BatchSize:              100,
			MaxConcurrency:         5,
			RetryAttempts:          3,
			RetryDelay:             30 * time.Second,
			ProcessingTimeout:      2 * time.Minute,
			PriorityProcessing:     true,
			SkipProcessed:          true,
			UpdateChargingHealth:   true,
			CheckpointInterval:     100,
			BatchID:                fmt.Sprintf("batch_%d", time.Now().Unix()),
			BatchDelay:             100 * time.Millisecond,
			MaxQueueSize:           1000,
			ProgressReportInterval: 10 * time.Second,
			ResultsBufferSize:      1000,
		}
	}

	// Create tracker if not provided
	var tracker ResubscriptionTracker
	if repo != nil {
		// Access database through repository using DBGetter interface
		if dbGetter, ok := repo.(repository.DBGetter); ok {
			tracker = NewDatabaseResubscriptionTracker(
				dbGetter.GetDB(),
				config.BatchID,
				config.CheckpointInterval,
				logger,
			)
		} else {
			logger.Warn("Repository type not supported for tracker - checkpointing will be disabled")
		}
	}

	p := &ResubscriptionProcessor{
		repo:         repo,
		service:      service,
		monitor:      monitor,
		logger:       logger,
		config:       config,
		stats:        &ResubscriptionStats{StartTime: time.Now()},
		tracker:      tracker,
		results:      make([]*ResubscriptionResult, 0),
		stopChan:     make(chan struct{}),
		progressChan: make(chan *ResubscriptionStats, config.MaxQueueSize),
		resultsChan:  make(chan *ResubscriptionResult, config.ResultsBufferSize),
	}

	return p
}

// SetTracker sets the tracker after creation (useful for dependency injection)
func (p *ResubscriptionProcessor) SetTracker(tracker ResubscriptionTracker) {
	p.tracker = tracker
}

// GetStatus returns the current processing status
func (p *ResubscriptionProcessor) GetStatus() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.isRunning {
		return "running"
	}
	return "stopped"
}

// Start begins the resubscription processing
func (p *ResubscriptionProcessor) Start(ctx context.Context) error {
	if p.isRunning {
		return fmt.Errorf("processor is already running")
	}

	p.isRunning = true
	p.logger.Info("Starting resubscription processor", zap.Any("config", p.config))

	// Start progress reporting
	go p.progressReporter(ctx)

	// Start results collector
	go p.resultsCollector(ctx)

	// Start processing
	go p.processChargingFailures(ctx)

	return nil
}

// Stop stops the processing
func (p *ResubscriptionProcessor) Stop() {
	if !p.isRunning {
		return
	}

	p.logger.Info("Stopping resubscription processor")
	close(p.stopChan)
	p.isRunning = false
}

// IsRunning returns the current running status
func (p *ResubscriptionProcessor) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isRunning
}

// Pause pauses the processing temporarily
func (p *ResubscriptionProcessor) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		p.isRunning = false
		p.logger.Info("Processing paused")
	}
}

// Resume resumes the processing after a pause
func (p *ResubscriptionProcessor) Resume(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		p.isRunning = true
		p.logger.Info("Processing resumed")

		// Restart the processing goroutines
		go p.progressReporter(ctx)
		go p.resultsCollector(ctx)
		go p.processChargingFailures(ctx)

		return nil
	}

	return fmt.Errorf("processor is already running")
}

// GetStats returns current processing statistics
func (p *ResubscriptionProcessor) GetStats() *ResubscriptionStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := *p.stats
	stats.LastProcessed = p.stats.LastProcessed

	// Calculate estimated completion time
	if stats.TotalProcessed > 0 && stats.CurrentBatch > 0 {
		elapsed := time.Since(stats.StartTime)
		avgPerBatch := elapsed / time.Duration(stats.CurrentBatch)
		remainingBatches := stats.TotalBatches - stats.CurrentBatch
		estimatedRemaining := avgPerBatch * time.Duration(remainingBatches)
		stats.EstimatedComplete = time.Now().Add(estimatedRemaining)
	}

	return &stats
}

// GetConfig returns the current processing configuration
func (p *ResubscriptionProcessor) GetConfig() *ProcessingConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// UpdateConfig updates the processing configuration
func (p *ResubscriptionProcessor) UpdateConfig(newConfig *ProcessingConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if newConfig == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Validate the new configuration
	if newConfig.BatchSize < 1 || newConfig.BatchSize > 10000 {
		return fmt.Errorf("invalid batch size: %d (must be between 1 and 10000)", newConfig.BatchSize)
	}
	if newConfig.MaxConcurrency < 1 || newConfig.MaxConcurrency > 100 {
		return fmt.Errorf("invalid max concurrency: %d (must be between 1 and 100)", newConfig.MaxConcurrency)
	}
	if newConfig.CheckpointInterval < 1 {
		return fmt.Errorf("invalid checkpoint interval: %d (must be greater than 0)", newConfig.CheckpointInterval)
	}

	// Update the configuration
	p.config = newConfig

	p.logger.Info("Configuration updated",
		zap.Int("batch_size", newConfig.BatchSize),
		zap.Int("max_concurrency", newConfig.MaxConcurrency),
		zap.Int("checkpoint_interval", newConfig.CheckpointInterval))

	return nil
}

// GetResults returns processing results (optionally filtered)
func (p *ResubscriptionProcessor) GetResults(status ResubscriptionStatus, limit int) []*ResubscriptionResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit <= 0 || limit > len(p.results) {
		limit = len(p.results)
	}

	filtered := make([]*ResubscriptionResult, 0, limit)
	count := 0

	for i := len(p.results) - 1; i >= 0 && count < limit; i-- {
		result := p.results[i]
		if status == "" || result.Status == status {
			filtered = append(filtered, result)
			count++
		}
	}

	return filtered
}

// ClearResults clears all processing results
func (p *ResubscriptionProcessor) ClearResults() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.results = make([]*ResubscriptionResult, 0)
	p.logger.Info("Processing results cleared")
}

// ResetStats resets all processing statistics
func (p *ResubscriptionProcessor) ResetStats() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats = &ResubscriptionStats{StartTime: time.Now()}
	p.logger.Info("Processing statistics reset")
}

// GetProcessingSummary returns a summary of the current processing run
func (p *ResubscriptionProcessor) GetProcessingSummary() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	summary := map[string]interface{}{
		"total_processed": p.stats.TotalProcessed,
		"successful":      p.stats.Successful,
		"failed":          p.stats.Failed,
		"skipped":         p.stats.Skipped,
		"success_rate":    p.stats.SuccessRate,
		"average_time":    p.stats.AverageTime.String(),
		"start_time":      p.stats.StartTime,
		"last_processed":  p.stats.LastProcessed,
		"current_batch":   p.stats.CurrentBatch,
		"total_batches":   p.stats.TotalBatches,
	}

	if p.stats.TotalProcessed > 0 {
		elapsed := time.Since(p.stats.StartTime)
		summary["elapsed_time"] = elapsed.String()
		summary["processing_rate"] = float64(p.stats.TotalProcessed) / elapsed.Seconds()
	}

	return summary
}

// processChargingFailures is the main processing loop
func (p *ResubscriptionProcessor) processChargingFailures(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("PANIC RECOVERED in processChargingFailures",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, ctx)
			}

			// Mark processor as stopped due to panic
			p.mu.Lock()
			p.isRunning = false
			p.mu.Unlock()
		}
	}()

	batchNum := 0
	offset := 0

	// Load existing checkpoint if available
	if p.tracker != nil {
		checkpoint, err := p.tracker.LoadCheckpoint()
		if err != nil {
			p.logger.Error("Failed to load checkpoint", zap.Error(err))
		} else if checkpoint != nil {
			p.logger.Info("Resuming from checkpoint",
				zap.String("batchID", checkpoint.BatchID),
				zap.Int("processed", checkpoint.ProcessedCount),
				zap.Int("lastID", checkpoint.LastProcessedID))
			offset = checkpoint.LastProcessedID
		}
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Processing stopped due to context cancellation")
			return
		case <-p.stopChan:
			p.logger.Info("Processing stopped by user")
			return
		default:
			// Continue processing
		}

		// Fetch batch of charging failures
		filter := repository.ChargingFailureFilter{
			Limit:            p.config.BatchSize,
			Offset:           offset,
			ExcludeProcessed: p.config.SkipProcessed,
		}

		subscriptions, err := p.repo.FetchChargingFailedSubscriptions(filter)
		if err != nil {
			p.logger.Error("Failed to fetch charging failures", zap.Error(err))
			time.Sleep(10 * time.Second) // Wait before retry
			continue
		}

		if len(subscriptions) == 0 {
			p.logger.Info("No more charging failures to process")
			break
		}

		batchNum++
		p.updateStats(func(stats *ResubscriptionStats) {
			stats.CurrentBatch = batchNum
			stats.TotalBatches = (int(p.stats.TotalProcessed) + len(subscriptions) + p.config.BatchSize - 1) / p.config.BatchSize
		})

		// Initialize tracker for this batch if not already done
		if p.tracker != nil && batchNum == 1 {
			if err := p.tracker.InitializeBatch(len(subscriptions)); err != nil {
				p.logger.Error("Failed to initialize tracker", zap.Error(err))
			}
		}

		p.logger.Info("Processing batch",
			zap.Int("batch", batchNum),
			zap.Int("count", len(subscriptions)),
			zap.Int("offset", offset))

		// Process batch with concurrency control
		p.processBatch(ctx, subscriptions, batchNum)

		offset += len(subscriptions)
		p.updateStats(func(stats *ResubscriptionStats) {
			stats.TotalProcessed += int64(len(subscriptions))
		})

		// Small delay between batches to prevent overwhelming the system
		time.Sleep(p.config.BatchDelay)
	}

	// Mark batch as completed
	if p.tracker != nil {
		if err := p.tracker.MarkCompleted(); err != nil {
			p.logger.Error("Failed to mark batch completed", zap.Error(err))
		}
	}

	p.logger.Info("Processing completed", zap.Int("total_batches", batchNum))
}

// processBatch processes a batch of subscriptions
func (p *ResubscriptionProcessor) processBatch(ctx context.Context, subscriptions []repository.ChargingFailedSubscription, batchNum int) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("PANIC RECOVERED in processBatch",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.Int("batch_num", batchNum),
				zap.Int("subscription_count", len(subscriptions)),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, ctx)
			}
		}
	}()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, p.config.MaxConcurrency)

	for _, subscription := range subscriptions {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		default:
			// Continue processing
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(sub repository.ChargingFailedSubscription) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			// Add panic recovery for individual goroutines
			defer func() {
				if r := recover(); r != nil {
					p.logger.Error("PANIC RECOVERED in subscription processing goroutine",
						zap.Any("panic_value", r),
						zap.String("panic_type", fmt.Sprintf("%T", r)),
						zap.String("msisdn", sub.MSISDN),
						zap.Int("product_id", sub.ProductID),
						zap.Int("batch_num", batchNum),
						zap.String("timestamp", time.Now().Format(time.RFC3339)),
					)

					// Use global panic handler if available
					if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
						panicHandler.HandlePanic(r, ctx)
					}

					// Record failed result due to panic
					result := &ResubscriptionResult{
						SubscriptionID: int64(sub.ID),
						MSISDN:         sub.MSISDN,
						ProductID:      sub.ProductID,
						Status:         StatusFailed,
						AttemptedAt:    time.Now(),
						CompletedAt:    &time.Time{},
						Error:          fmt.Sprintf("Panic recovered: %v", r),
						RetryCount:     0,
						Priority:       p.calculatePriority(&sub),
					}
					p.recordResult(result)
				}
			}()

			p.processSubscription(ctx, &sub, batchNum)
		}(subscription)
	}

	wg.Wait()
}

// processSubscription processes a single subscription
func (p *ResubscriptionProcessor) processSubscription(ctx context.Context, subscription *repository.ChargingFailedSubscription, batchNum int) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("PANIC RECOVERED in processSubscription",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("msisdn", subscription.MSISDN),
				zap.Int("product_id", subscription.ProductID),
				zap.Int("batch_num", batchNum),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, ctx)
			}
		}
	}()

	startTime := time.Now()
	result := &ResubscriptionResult{
		SubscriptionID: int64(subscription.ID),
		MSISDN:         subscription.MSISDN,
		ProductID:      subscription.ProductID,
		Status:         StatusPending,
		AttemptedAt:    time.Now(),
		RetryCount:     0,
		Priority:       p.calculatePriority(subscription),
	}

	// Check if already processed using tracker
	if p.config.SkipProcessed && p.tracker != nil {
		processed, err := p.tracker.CheckIfProcessed(subscription.MSISDN, subscription.ProductID)
		if err != nil {
			p.logger.Error("Failed to check if processed", zap.Error(err))
		} else if processed {
			result.Status = StatusSkipped
			result.CompletedAt = &startTime
			result.Error = "Already processed"
			p.recordResult(result)
			return
		}
	}

	// Record attempt in tracker
	if p.tracker != nil {
		if err := p.tracker.RecordAttempt(subscription.MSISDN, subscription.ProductID, subscription.ID); err != nil {
			p.logger.Error("Failed to record attempt", zap.Error(err))
		}
	}

	// Process with retry logic
	for attempt := 0; attempt <= p.config.RetryAttempts; attempt++ {
		result.RetryCount = attempt
		result.Status = StatusProcessing

		// Create context with timeout
		processCtx, cancel := context.WithTimeout(ctx, p.config.ProcessingTimeout)

		success, err := p.attemptResubscription(processCtx, subscription)
		cancel()

		if success {
			result.Status = StatusSuccess
			completedAt := time.Now()
			result.CompletedAt = &completedAt

			// Update tracker with success
			if p.tracker != nil {
				if err := p.tracker.UpdateResult(subscription.MSISDN, subscription.ProductID, true, ""); err != nil {
					p.logger.Error("Failed to update tracker with success", zap.Error(err))
				}
			}
			break
		} else if attempt == p.config.RetryAttempts {
			result.Status = StatusFailed
			result.Error = err.Error()

			// Update tracker with failure
			if p.tracker != nil {
				if err := p.tracker.UpdateResult(subscription.MSISDN, subscription.ProductID, false, err.Error()); err != nil {
					p.logger.Error("Failed to update tracker with failure", zap.Error(err))
				}
			}
		} else {
			// Wait before retry
			select {
			case <-ctx.Done():
				result.Status = StatusFailed
				result.Error = "Processing cancelled"
				break
			case <-p.stopChan:
				result.Status = StatusFailed
				result.Error = "Processing stopped"
				break
			case <-time.After(p.config.RetryDelay):
				continue
			}
		}
	}

	result.ProcessingTime = time.Since(startTime)
	p.recordResult(result)

	// Update charging health status if configured
	if p.config.UpdateChargingHealth && result.Status == StatusSuccess {
		// Update charging health status using the new tracking table
		p.updateChargingHealth(int64(subscription.ID), "resubscribed")
	}

	// Save checkpoint periodically
	if p.tracker != nil && p.config.CheckpointInterval > 0 {
		if p.stats.TotalProcessed%int64(p.config.CheckpointInterval) == 0 {
			if err := p.tracker.SaveCheckpoint(subscription.ID, subscription.MSISDN); err != nil {
				p.logger.Error("Failed to save checkpoint", zap.Error(err))
			}
			p.tracker.LogProgress()
		}
	}

	// Update monitor metrics
	p.updateMonitorMetrics()
}

// attemptResubscription attempts to resubscribe a single subscription
func (p *ResubscriptionProcessor) attemptResubscription(ctx context.Context, subscription *repository.ChargingFailedSubscription) (bool, error) {
	if p.service == nil {
		return false, fmt.Errorf("subscription service not available")
	}

	// ResubscribeUser handles unsubscribe internally, so we don't need to call SendOptout separately
	// It will: 1) Unsubscribe from the product, 2) Re-subscribe to the product
	entryChannel := "SMS" // Default entry channel, could be made configurable

	p.logger.Info("Starting resubscription process",
		zap.String("msisdn", subscription.MSISDN),
		zap.Int64("subscription_id", int64(subscription.ID)),
		zap.Int("product_id", subscription.ProductID),
		zap.String("entry_channel", entryChannel))

	// Call ResubscribeUser which handles both unsubscribe and resubscribe internally
	resubscribeErr := p.service.ResubscribeUser(subscription.MSISDN, entryChannel, []string{fmt.Sprintf("%d", subscription.ProductID)})
	if resubscribeErr != nil {
		// Categorize resubscribe error for better handling
		errorType, shouldRetry := p.categorizeExternalAPIError(resubscribeErr)

		p.logger.Error("Resubscribe failed",
			zap.String("msisdn", subscription.MSISDN),
			zap.Int64("subscription_id", int64(subscription.ID)),
			zap.String("error_type", errorType),
			zap.Bool("should_retry", shouldRetry),
			zap.Error(resubscribeErr))

		return false, fmt.Errorf("failed to resubscribe: %w", resubscribeErr)
	}

	p.logger.Info("Successfully completed resubscription process",
		zap.String("msisdn", subscription.MSISDN),
		zap.Int64("subscription_id", int64(subscription.ID)),
		zap.Int("product_id", subscription.ProductID))

	return true, nil
}

// categorizeExternalAPIError categorizes external API errors for better handling
func (p *ResubscriptionProcessor) categorizeExternalAPIError(err error) (string, bool) {
	if err == nil {
		return "none", false
	}

	errStr := err.Error()

	// Check for specific error patterns from TIMWE system
	switch {
	case strings.Contains(errStr, "ArrayIndexOutOfBoundsException"):
		return "cache_error", true // External system cache issue, retry later
	case strings.Contains(errStr, "500") || strings.Contains(errStr, "Internal Server Error"):
		return "external_system_error", true // External system issue, retry later
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return "timeout_error", true // Network timeout, retry
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no route to host"):
		return "network_error", true // Network issue, retry
	case strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests"):
		return "rate_limit_error", false // Rate limited, don't retry immediately
	case strings.Contains(errStr, "authentication") || strings.Contains(errStr, "unauthorized"):
		return "auth_error", false // Auth issue, don't retry
	case strings.Contains(errStr, "validation") || strings.Contains(errStr, "bad request"):
		return "validation_error", false // Bad request, don't retry
	case strings.Contains(errStr, "OPTIN_CONFIG_NOT_FOUND"):
		return "config_error", true // Configuration issue, retry with different channel
	case strings.Contains(errStr, "product not found") || strings.Contains(errStr, "ProductId"):
		return "product_error", false // Product doesn't exist, don't retry
	case strings.Contains(errStr, "user not found") || strings.Contains(errStr, "MSISDN"):
		return "user_error", false // User doesn't exist, don't retry
	case strings.Contains(errStr, "already subscribed") || strings.Contains(errStr, "already active"):
		return "duplicate_error", false // Already subscribed, don't retry
	case strings.Contains(errStr, "circuit breaker"):
		return "circuit_breaker_error", true // Circuit breaker open, retry later
	default:
		return "unknown_error", true // Unknown error, retry with caution
	}
}

// shouldRetryExternalAPI determines if an external API call should be retried based on error type and retry count
func (p *ResubscriptionProcessor) shouldRetryExternalAPI(errorType string, retryCount int, lastErrorTime time.Time) bool {
	// Don't retry if we've already tried too many times
	if retryCount >= 3 {
		return false
	}

	// Don't retry certain error types
	switch errorType {
	case "auth_error", "validation_error":
		return false
	case "rate_limit_error":
		// For rate limits, wait longer before retrying
		return time.Since(lastErrorTime) > 5*time.Minute
	}

	// For retryable errors, implement exponential backoff
	backoffDuration := time.Duration(retryCount*retryCount) * time.Second
	return time.Since(lastErrorTime) > backoffDuration
}

// getExternalAPIBackoffDuration calculates backoff duration for external API retries
func (p *ResubscriptionProcessor) getExternalAPIBackoffDuration(retryCount int) time.Duration {
	// Exponential backoff: 1s, 4s, 9s, 16s, etc.
	baseDelay := time.Duration(retryCount*retryCount) * time.Second

	// Add some jitter to prevent thundering herd
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond

	return baseDelay + jitter
}

// calculatePriority calculates processing priority for a subscription
func (p *ResubscriptionProcessor) calculatePriority(subscription *repository.ChargingFailedSubscription) int {
	priority := 0

	// Higher priority for never charged (revenue opportunity)
	if subscription.ChargingHealthStatus == "NEVER_CHARGED" {
		priority += 100
	}

	// Higher priority for recent subscriptions
	daysSinceSubscription := subscription.DaysWithoutCharge
	if daysSinceSubscription < 7 {
		priority += 50
	} else if daysSinceSubscription < 30 {
		priority += 30
	} else if daysSinceSubscription < 90 {
		priority += 10
	}

	return priority
}

// updateChargingHealth updates the charging health status in the database
func (p *ResubscriptionProcessor) updateChargingHealth(subscriptionID int64, status string) {
	if p.repo == nil {
		p.logger.Warn("Repository not available, cannot update charging health status",
			zap.Int64("subscription_id", subscriptionID),
			zap.String("status", status))
		return
	}

	// Update the subscription charging health status using the repository method
	err := p.repo.UpdateChargingHealthStatus(int(subscriptionID), status, "resubscription_processor")
	if err != nil {
		p.logger.Error("Failed to update charging health status",
			zap.Int64("subscription_id", subscriptionID),
			zap.String("status", status),
			zap.Error(err))
	} else {
		p.logger.Info("Successfully updated charging health status",
			zap.Int64("subscription_id", subscriptionID),
			zap.String("status", status))
	}
}

// recordResult records a processing result
func (p *ResubscriptionProcessor) recordResult(result *ResubscriptionResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.results = append(p.results, result)

	// Update statistics
	switch result.Status {
	case StatusSuccess:
		p.stats.Successful++
	case StatusFailed:
		p.stats.Failed++
	case StatusSkipped:
		p.stats.Skipped++
	}

	p.stats.LastProcessed = time.Now()

	// Calculate success rate
	if p.stats.TotalProcessed > 0 {
		p.stats.SuccessRate = float64(p.stats.Successful) / float64(p.stats.TotalProcessed) * 100
	}

	// Calculate average processing time
	if p.stats.Successful > 0 {
		totalTime := p.stats.AverageTime * time.Duration(p.stats.Successful-1)
		totalTime += result.ProcessingTime
		p.stats.AverageTime = totalTime / time.Duration(p.stats.Successful)
	}

	// Send result to channel
	select {
	case p.resultsChan <- result:
	default:
		p.logger.Warn("Results channel full, dropping result")
	}
}

// updateStats updates statistics with a function
func (p *ResubscriptionProcessor) updateStats(updateFn func(*ResubscriptionStats)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	updateFn(p.stats)
}

// updateMonitorMetrics updates the monitoring system
func (p *ResubscriptionProcessor) updateMonitorMetrics() {
	stats := p.GetStats()

	metrics := &monitoring.ChargingFailureMetrics{
		TotalSubscriptions:    stats.TotalProcessed,
		ChargingFailures:      stats.TotalProcessed, // This will be updated by actual data
		FailureRate:           100.0,                // Placeholder
		ProcessingQueue:       0,                    // Will be updated based on actual queue
		ProcessedToday:        stats.Successful,
		SuccessRate:           stats.SuccessRate,
		LastUpdated:           time.Now(),
		ProcessingStatus:      "running",
		AverageProcessingTime: stats.AverageTime.Minutes(),
	}

	p.monitor.UpdateMetrics(metrics)
}

// progressReporter reports progress periodically
func (p *ResubscriptionProcessor) progressReporter(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("PANIC RECOVERED in progressReporter",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, ctx)
			}
		}
	}()

	interval := p.config.ProgressReportInterval
	if interval == 0 {
		interval = 10 * time.Second // Default fallback
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			stats := p.GetStats()
			select {
			case p.progressChan <- stats:
			default:
				// Channel full, skip this update
			}
		}
	}
}

// resultsCollector collects results for reporting
func (p *ResubscriptionProcessor) resultsCollector(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("PANIC RECOVERED in resultsCollector",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, ctx)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case result := <-p.resultsChan:
			// Process result (could send to external systems, update databases, etc.)
			p.logger.Debug("Result collected",
				zap.Int64("subscription_id", result.SubscriptionID),
				zap.String("status", string(result.Status)))
		}
	}
}

// GetProgressChannel returns the progress channel for monitoring
func (p *ResubscriptionProcessor) GetProgressChannel() <-chan *ResubscriptionStats {
	return p.progressChan
}

// GetResultsChannel returns the results channel for monitoring
func (p *ResubscriptionProcessor) GetResultsChannel() <-chan *ResubscriptionResult {
	return p.resultsChan
}

// GetDetailedStatus returns comprehensive processing status information
func (p *ResubscriptionProcessor) GetDetailedStatus() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := map[string]interface{}{
		"is_running":         p.isRunning,
		"start_time":         p.stats.StartTime,
		"last_processed":     p.stats.LastProcessed,
		"current_batch":      p.stats.CurrentBatch,
		"total_batches":      p.stats.TotalBatches,
		"estimated_complete": p.stats.EstimatedComplete,
	}

	// Add tracker information if available
	if p.tracker != nil {
		trackerStats := p.tracker.GetStats()
		status["tracker"] = map[string]interface{}{
			"batch_id":        trackerStats.BatchID,
			"total_count":     trackerStats.TotalCount,
			"processed_count": trackerStats.ProcessedCount,
			"success_count":   trackerStats.SuccessCount,
			"failure_count":   trackerStats.FailureCount,
			"status":          trackerStats.Status,
			"started_at":      trackerStats.StartedAt,
			"updated_at":      trackerStats.UpdatedAt,
			"completed_at":    trackerStats.CompletedAt,
		}
	}

	// Add configuration information
	status["config"] = map[string]interface{}{
		"batch_size":             p.config.BatchSize,
		"max_concurrency":        p.config.MaxConcurrency,
		"retry_attempts":         p.config.RetryAttempts,
		"checkpoint_interval":    p.config.CheckpointInterval,
		"priority_processing":    p.config.PriorityProcessing,
		"skip_processed":         p.config.SkipProcessed,
		"update_charging_health": p.config.UpdateChargingHealth,
	}

	return status
}

// GracefulShutdown performs a graceful shutdown with cleanup
func (p *ResubscriptionProcessor) GracefulShutdown(ctx context.Context, timeout time.Duration) error {
	p.logger.Info("Starting graceful shutdown", zap.Duration("timeout", timeout))

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Stop accepting new work
	p.Stop()

	// Wait for current processing to complete
	shutdownChan := make(chan struct{})
	go func() {
		// Wait for all workers to complete
		for p.isRunning {
			time.Sleep(100 * time.Millisecond)
		}
		close(shutdownChan)
	}()

	// Wait for shutdown or timeout
	select {
	case <-shutdownChan:
		p.logger.Info("All processing completed, proceeding with cleanup")
	case <-shutdownCtx.Done():
		p.logger.Warn("Shutdown timeout reached, forcing cleanup")
	}

	// Save final checkpoint if tracker is available
	if p.tracker != nil {
		if err := p.tracker.SaveCheckpoint(0, ""); err != nil {
			p.logger.Error("Failed to save final checkpoint", zap.Error(err))
		} else {
			p.logger.Info("Final checkpoint saved")
		}
	}

	// Clear channels
	close(p.progressChan)
	close(p.resultsChan)

	// Log final statistics
	finalStats := p.GetStats()
	p.logger.Info("Graceful shutdown completed",
		zap.Int64("total_processed", finalStats.TotalProcessed),
		zap.Int64("successful", finalStats.Successful),
		zap.Int64("failed", finalStats.Failed),
		zap.Int64("skipped", finalStats.Skipped))

	return nil
}

// ExportResults exports processing results in the specified format
func (p *ResubscriptionProcessor) ExportResults(format string, status ResubscriptionStatus) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Get filtered results
	results := p.GetResults(status, 0) // 0 means no limit

	switch format {
	case "json":
		return json.Marshal(results)
	case "csv":
		return p.exportResultsToCSV(results)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportResultsToCSV exports results to CSV format
func (p *ResubscriptionProcessor) exportResultsToCSV(results []*ResubscriptionResult) ([]byte, error) {
	var buf bytes.Buffer

	// Write CSV header
	writer := csv.NewWriter(&buf)
	defer writer.Flush()

	header := []string{
		"SubscriptionID", "MSISDN", "ProductID", "Status", "AttemptedAt",
		"CompletedAt", "Error", "RetryCount", "Priority", "ProcessingTime",
	}

	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, result := range results {
		completedAt := ""
		if result.CompletedAt != nil {
			completedAt = result.CompletedAt.Format(time.RFC3339)
		}

		row := []string{
			fmt.Sprintf("%d", result.SubscriptionID),
			result.MSISDN,
			fmt.Sprintf("%d", result.ProductID),
			string(result.Status),
			result.AttemptedAt.Format(time.RFC3339),
			completedAt,
			result.Error,
			fmt.Sprintf("%d", result.RetryCount),
			fmt.Sprintf("%d", result.Priority),
			result.ProcessingTime.String(),
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// GetResultsByTimeRange returns results within a specific time range
func (p *ResubscriptionProcessor) GetResultsByTimeRange(start, end time.Time, status ResubscriptionStatus) []*ResubscriptionResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var filtered []*ResubscriptionResult

	for _, result := range p.results {
		// Check time range
		if result.AttemptedAt.Before(start) || result.AttemptedAt.After(end) {
			continue
		}

		// Check status filter
		if status != "" && result.Status != status {
			continue
		}

		filtered = append(filtered, result)
	}

	return filtered
}
