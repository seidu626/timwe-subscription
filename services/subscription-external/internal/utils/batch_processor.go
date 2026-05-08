package utils

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// BatchProcessor handles batch operations with enhanced error handling
type BatchProcessor struct {
	logger         *zap.Logger
	config         *BatchConfig
	metrics        *BatchMetrics
	errorCollector *ErrorCollector
	retryQueue     chan BatchItem
	stopChan       chan struct{}
	wg             sync.WaitGroup
	isRunning      int32
}

// BatchConfig contains configuration for batch processing
type BatchConfig struct {
	BatchSize           int
	MaxConcurrency      int
	RetryAttempts       int
	RetryDelay          time.Duration
	PartialSuccessRatio float64 // Minimum success ratio to consider batch partially successful
	ErrorBatchSize      int     // Size of error batches for logging
	ProcessingTimeout   time.Duration
	EnableRetryQueue    bool
}

// BatchMetrics tracks batch processing statistics
type BatchMetrics struct {
	TotalProcessed    int64
	TotalSucceeded    int64
	TotalFailed       int64
	TotalRetried      int64
	BatchesProcessed  int64
	BatchesFailed     int64
	AverageLatency    time.Duration
	LastProcessedTime time.Time
	mu                sync.RWMutex
}

// BatchItem represents an item to be processed in a batch
type BatchItem interface {
	GetID() string
	GetRetryCount() int
	IncrementRetryCount()
	IsRetryable() bool
}

// BatchResult contains the result of batch processing
type BatchResult struct {
	TotalItems      int
	SuccessfulItems int
	FailedItems     int
	Errors          []error
	ProcessingTime  time.Duration
	PartialSuccess  bool
}

// ErrorCollector collects and manages batch processing errors
type ErrorCollector struct {
	errors     []error
	errorBatch []error
	batchSize  int
	logger     *zap.Logger
	mu         sync.Mutex
}

// DefaultBatchConfig returns sensible defaults for batch processing
func DefaultBatchConfig() *BatchConfig {
	return &BatchConfig{
		BatchSize:           100, // Reduced from larger sizes for better error handling
		MaxConcurrency:      10,  // Controlled concurrency
		RetryAttempts:       3,
		RetryDelay:          2 * time.Second,
		PartialSuccessRatio: 0.7, // 70% success rate for partial success
		ErrorBatchSize:      10,  // Log errors in batches of 10
		ProcessingTimeout:   5 * time.Minute,
		EnableRetryQueue:    true,
	}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(logger *zap.Logger, config *BatchConfig) *BatchProcessor {
	if config == nil {
		config = DefaultBatchConfig()
	}

	errorCollector := &ErrorCollector{
		errors:    make([]error, 0),
		batchSize: config.ErrorBatchSize,
		logger:    logger,
	}

	processor := &BatchProcessor{
		logger:         logger,
		config:         config,
		metrics:        &BatchMetrics{},
		errorCollector: errorCollector,
		retryQueue:     make(chan BatchItem, config.BatchSize*2), // Buffer for retry queue
		stopChan:       make(chan struct{}),
	}

	// Start background goroutines
	processor.startBackgroundWorkers()

	return processor
}

// startBackgroundWorkers starts background processing goroutines
func (p *BatchProcessor) startBackgroundWorkers() {
	// Start retry queue processor
	if p.config.EnableRetryQueue {
		p.wg.Add(1)
		go p.retryQueueProcessor(context.Background())
	}

	// Start metrics updater
	p.wg.Add(1)
	go p.metricsUpdater()
}

// metricsUpdater periodically updates metrics
func (p *BatchProcessor) metricsUpdater() {
	defer p.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			// Create a dummy result for periodic metrics update
			dummyResult := &BatchResult{
				TotalItems:      0,
				SuccessfulItems: 0,
				FailedItems:     0,
				ProcessingTime:  0,
			}
			p.updateMetrics(dummyResult)
		}
	}
}

// Start starts the batch processor
func (bp *BatchProcessor) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&bp.isRunning, 0, 1) {
		return fmt.Errorf("batch processor is already running")
	}

	bp.logger.Info("Starting batch processor",
		zap.Int("batch_size", bp.config.BatchSize),
		zap.Int("max_concurrency", bp.config.MaxConcurrency),
		zap.Bool("retry_queue_enabled", bp.config.EnableRetryQueue))

	// Start retry queue processor if enabled
	if bp.config.EnableRetryQueue {
		bp.wg.Add(1)
		go bp.retryQueueProcessor(ctx)
	}

	return nil
}

// Stop stops the batch processor
func (bp *BatchProcessor) Stop() error {
	if !atomic.CompareAndSwapInt32(&bp.isRunning, 1, 0) {
		return fmt.Errorf("batch processor is not running")
	}

	bp.logger.Info("Stopping batch processor")
	close(bp.stopChan)
	bp.wg.Wait()

	if bp.retryQueue != nil {
		close(bp.retryQueue)
	}

	return nil
}

// ProcessBatch processes a batch of items with enhanced error handling
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, items []BatchItem, processor func(BatchItem) error) *BatchResult {
	startTime := time.Now()

	result := &BatchResult{
		TotalItems: len(items),
		Errors:     make([]error, 0),
	}

	if len(items) == 0 {
		return result
	}

	bp.logger.Info("Processing batch",
		zap.Int("total_items", len(items)),
		zap.Int("batch_size", bp.config.BatchSize))

	// Process items in smaller batches
	batches := bp.createBatches(items)

	for i, batch := range batches {
		batchResult := bp.processSingleBatch(ctx, batch, processor, i+1, len(batches))

		// Aggregate results
		result.SuccessfulItems += batchResult.SuccessfulItems
		result.FailedItems += batchResult.FailedItems
		result.Errors = append(result.Errors, batchResult.Errors...)

		// Check if we should continue processing
		if bp.shouldStopProcessing(ctx, result) {
			break
		}
	}

	result.ProcessingTime = time.Since(startTime)

	// Determine if batch was partially successful
	successRatio := float64(result.SuccessfulItems) / float64(result.TotalItems)
	result.PartialSuccess = successRatio >= bp.config.PartialSuccessRatio && result.SuccessfulItems > 0

	// Update metrics
	bp.updateMetrics(result)

	bp.logger.Info("Batch processing completed",
		zap.Int("total_items", result.TotalItems),
		zap.Int("successful_items", result.SuccessfulItems),
		zap.Int("failed_items", result.FailedItems),
		zap.Float64("success_ratio", successRatio),
		zap.Bool("partial_success", result.PartialSuccess),
		zap.Duration("processing_time", result.ProcessingTime))

	return result
}

// createBatches splits items into smaller batches
func (bp *BatchProcessor) createBatches(items []BatchItem) [][]BatchItem {
	var batches [][]BatchItem
	batchSize := bp.config.BatchSize

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// processSingleBatch processes a single batch with concurrency control
func (bp *BatchProcessor) processSingleBatch(ctx context.Context, items []BatchItem, processor func(BatchItem) error, batchNum, totalBatches int) *BatchResult {
	result := &BatchResult{
		TotalItems: len(items),
		Errors:     make([]error, 0),
	}

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, bp.config.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	bp.logger.Debug("Processing single batch",
		zap.Int("batch_num", batchNum),
		zap.Int("total_batches", totalBatches),
		zap.Int("items_in_batch", len(items)))

	for _, item := range items {
		wg.Add(1)
		go func(item BatchItem) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process item with timeout
			itemCtx, cancel := context.WithTimeout(ctx, bp.config.ProcessingTimeout)
			defer cancel()

			err := bp.processItemWithRetry(itemCtx, item, processor)

			mu.Lock()
			if err != nil {
				result.FailedItems++
				result.Errors = append(result.Errors, err)
				bp.errorCollector.AddError(err)

				// Add to retry queue if retryable and queue is enabled
				if bp.config.EnableRetryQueue && item.IsRetryable() && item.GetRetryCount() < bp.config.RetryAttempts {
					select {
					case bp.retryQueue <- item:
						bp.logger.Debug("Added item to retry queue", zap.String("item_id", item.GetID()))
					default:
						bp.logger.Warn("Retry queue is full, dropping item", zap.String("item_id", item.GetID()))
					}
				}
			} else {
				result.SuccessfulItems++
			}
			mu.Unlock()
		}(item)
	}

	wg.Wait()
	return result
}

// processItemWithRetry processes a single item with retry logic
func (bp *BatchProcessor) processItemWithRetry(ctx context.Context, item BatchItem, processor func(BatchItem) error) error {
	var lastErr error

	for attempt := 0; attempt <= bp.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(bp.config.RetryDelay):
			}

			item.IncrementRetryCount()
			atomic.AddInt64(&bp.metrics.TotalRetried, 1)
		}

		err := processor(item)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !item.IsRetryable() {
			break
		}

		bp.logger.Debug("Retrying item processing",
			zap.String("item_id", item.GetID()),
			zap.Int("attempt", attempt+1),
			zap.Error(err))
	}

	return fmt.Errorf("failed after %d attempts: %w", bp.config.RetryAttempts+1, lastErr)
}

// retryQueueProcessor processes items from the retry queue
func (bp *BatchProcessor) retryQueueProcessor(ctx context.Context) {
	defer bp.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Process retry queue every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-bp.stopChan:
			return
		case <-ticker.C:
			bp.processRetryQueue(ctx)
		}
	}
}

// processRetryQueue processes items in the retry queue
func (bp *BatchProcessor) processRetryQueue(ctx context.Context) {
	var retryItems []BatchItem

	// Collect items from retry queue
	for {
		select {
		case item := <-bp.retryQueue:
			retryItems = append(retryItems, item)
			if len(retryItems) >= bp.config.BatchSize {
				break
			}
		default:
			break
		}
	}

	if len(retryItems) == 0 {
		return
	}

	bp.logger.Info("Processing retry queue",
		zap.Int("retry_items", len(retryItems)))

	// Process retry items (this would need a processor function passed in)
	// For now, just log that we would process them
	for _, item := range retryItems {
		bp.logger.Debug("Would retry item", zap.String("item_id", item.GetID()))
	}
}

// shouldStopProcessing determines if batch processing should stop
func (bp *BatchProcessor) shouldStopProcessing(ctx context.Context, result *BatchResult) bool {
	// Stop if context is cancelled
	if ctx.Err() != nil {
		return true
	}

	// Stop if error rate is too high (optional logic)
	if result.TotalItems > 0 {
		errorRate := float64(result.FailedItems) / float64(result.TotalItems)
		if errorRate > 0.9 { // Stop if more than 90% errors
			bp.logger.Warn("Stopping batch processing due to high error rate",
				zap.Float64("error_rate", errorRate))
			return true
		}
	}

	return false
}

// updateMetrics updates batch processing metrics
func (bp *BatchProcessor) updateMetrics(result *BatchResult) {
	bp.metrics.mu.Lock()
	defer bp.metrics.mu.Unlock()

	bp.metrics.TotalProcessed += int64(result.TotalItems)
	bp.metrics.TotalSucceeded += int64(result.SuccessfulItems)
	bp.metrics.TotalFailed += int64(result.FailedItems)
	bp.metrics.BatchesProcessed++

	if result.FailedItems > result.SuccessfulItems {
		bp.metrics.BatchesFailed++
	}

	bp.metrics.LastProcessedTime = time.Now()

	// Update average latency (simple moving average)
	if bp.metrics.BatchesProcessed == 1 {
		bp.metrics.AverageLatency = result.ProcessingTime
	} else {
		bp.metrics.AverageLatency = (bp.metrics.AverageLatency + result.ProcessingTime) / 2
	}
}

// GetMetrics returns current batch processing metrics
func (bp *BatchProcessor) GetMetrics() *BatchMetrics {
	bp.metrics.mu.RLock()
	defer bp.metrics.mu.RUnlock()

	// Return a copy to avoid race conditions
	return &BatchMetrics{
		TotalProcessed:    bp.metrics.TotalProcessed,
		TotalSucceeded:    bp.metrics.TotalSucceeded,
		TotalFailed:       bp.metrics.TotalFailed,
		TotalRetried:      bp.metrics.TotalRetried,
		BatchesProcessed:  bp.metrics.BatchesProcessed,
		BatchesFailed:     bp.metrics.BatchesFailed,
		AverageLatency:    bp.metrics.AverageLatency,
		LastProcessedTime: bp.metrics.LastProcessedTime,
	}
}

// AddError adds an error to the error collector
func (ec *ErrorCollector) AddError(err error) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errors = append(ec.errors, err)
	ec.errorBatch = append(ec.errorBatch, err)

	// Log error batch when it reaches the configured size
	if len(ec.errorBatch) >= ec.batchSize {
		ec.logger.Error("Batch of processing errors",
			zap.Int("error_count", len(ec.errorBatch)),
			zap.String("sample_error", ec.errorBatch[0].Error()))

		// Clear the error batch
		ec.errorBatch = ec.errorBatch[:0]
	}
}

// GetErrors returns all collected errors
func (ec *ErrorCollector) GetErrors() []error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	// Return a copy
	errors := make([]error, len(ec.errors))
	copy(errors, ec.errors)
	return errors
}

// ClearErrors clears all collected errors
func (ec *ErrorCollector) ClearErrors() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errors = ec.errors[:0]
	ec.errorBatch = ec.errorBatch[:0]
}
