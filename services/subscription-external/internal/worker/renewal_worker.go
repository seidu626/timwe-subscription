package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"go.uber.org/zap"
)

// ProcessingStats tracks processing statistics
type ProcessingStats struct {
	Total     int
	Processed int
	Renewed   int
	Churned   int
	Failed    int
	Skipped   int
	mu        sync.Mutex
}

// RenewalWorker handles scheduled renewal processing
type RenewalWorker struct {
	renewalService service.RenewalServiceInterface
	repo           repository.SubscriptionRepositoryInterface
	productRepo    *repository.ProductRepository
	logger         *zap.Logger
	config         *domain.RenewalConfig
	isRunning      bool
	mu             sync.RWMutex
	stats          *ProcessingStats
}

// NewRenewalWorker creates a new renewal worker
func NewRenewalWorker(
	renewalService service.RenewalServiceInterface,
	repo repository.SubscriptionRepositoryInterface,
	productRepo *repository.ProductRepository,
	logger *zap.Logger,
	config *domain.RenewalConfig,
) *RenewalWorker {
	return &RenewalWorker{
		renewalService: renewalService,
		repo:           repo,
		productRepo:    productRepo,
		logger:         logger,
		config:         config,
		stats:          &ProcessingStats{},
	}
}

// Start begins the renewal worker
func (w *RenewalWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.isRunning {
		w.mu.Unlock()
		return fmt.Errorf("renewal worker already running")
	}
	w.isRunning = true
	w.mu.Unlock()

	w.logger.Info("Starting renewal worker")

	// Run based on schedule
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Renewal worker stopped")
			w.mu.Lock()
			w.isRunning = false
			w.mu.Unlock()
			return nil

		case <-ticker.C:
			// Check if it's time to run
			if w.shouldRunNow() {
				go w.ProcessRenewals(ctx)
			}
		}
	}
}

// shouldRunNow checks if current time matches scheduled run time
func (w *RenewalWorker) shouldRunNow() bool {
	now := time.Now()
	
	// Parse scheduled time
	scheduledTime, err := time.Parse("15:04", w.config.Worker.DailyRunTime)
	if err != nil {
		w.logger.Error("Invalid daily run time configuration", zap.Error(err))
		return false
	}

	// Check if we're at the scheduled time (within the same minute)
	return now.Hour() == scheduledTime.Hour() &&
		now.Minute() == scheduledTime.Minute()
}

// ProcessRenewals is the main renewal processing function
func (w *RenewalWorker) ProcessRenewals(ctx context.Context) {
	startTime := time.Now()
	w.logger.Info("Starting daily renewal processing")

	// Reset statistics
	w.stats = &ProcessingStats{}

	// Get subscriptions needing renewal
	subscriptions, err := w.repo.GetSubscriptionsNeedingRenewal(7, 1000) // 7 days threshold, 1000 limit
	if err != nil {
		w.logger.Error("Failed to get subscriptions for renewal", zap.Error(err))
		return
	}

	w.logger.Info("Found subscriptions for renewal evaluation",
		zap.Int("count", len(subscriptions)))

	w.stats.Total = len(subscriptions)

	// Process in batches
	batchSize := w.config.OptOutOptIn.BatchSize
	if batchSize == 0 {
		batchSize = 50 // Default batch size
	}

	for i := 0; i < len(subscriptions); i += batchSize {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			w.logger.Info("Renewal processing cancelled")
			return
		default:
		}

		end := i + batchSize
		if end > len(subscriptions) {
			end = len(subscriptions)
		}

		batch := subscriptions[i:end]
		w.processBatch(ctx, batch)

		// Rate limiting between batches
		if i+batchSize < len(subscriptions) {
			time.Sleep(time.Duration(w.config.OptOutOptIn.BatchDelayMs) * time.Millisecond)
		}
	}

	// Log final statistics
	w.logger.Info("Renewal processing completed",
		zap.Int("total", w.stats.Total),
		zap.Int("renewed", w.stats.Renewed),
		zap.Int("churned", w.stats.Churned),
		zap.Int("failed", w.stats.Failed),
		zap.Int("skipped", w.stats.Skipped),
		zap.Duration("duration", time.Since(startTime)))

	// Save metrics
	w.saveMetrics()
}

// processBatch processes a batch of subscriptions
func (w *RenewalWorker) processBatch(ctx context.Context, subscriptions []*domain.SubscriptionWithRenewalInfo) {
	var wg sync.WaitGroup
	
	// Create semaphore for concurrency control
	maxConcurrent := w.config.OptOutOptIn.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = 5 // Default
	}
	semaphore := make(chan struct{}, maxConcurrent)

	for _, sub := range subscriptions {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(sub *domain.SubscriptionWithRenewalInfo) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			// Process with timeout
			processCtx, cancel := context.WithTimeout(ctx, w.config.Worker.TimeoutPerRenewal)
			defer cancel()

			w.processSubscription(processCtx, sub)
		}(sub)
	}

	wg.Wait()
}

// processSubscription handles a single subscription renewal
func (w *RenewalWorker) processSubscription(ctx context.Context, sub *domain.SubscriptionWithRenewalInfo) {
	w.stats.mu.Lock()
	w.stats.Processed++
	w.stats.mu.Unlock()

	// Evaluate churn policy
	action := w.renewalService.EvaluateChurnPolicy(ctx, sub.UserIdentifier, sub.ProductId)

	w.logger.Debug("Processing subscription",
		zap.String("msisdn", sub.UserIdentifier),
		zap.String("productId", sub.ProductId),
		zap.String("action", string(action)))

	switch action {
	case domain.ActionAttemptRenewal:
		if err := w.attemptRenewal(ctx, sub); err != nil {
			w.logger.Error("Renewal failed",
				zap.String("msisdn", sub.UserIdentifier),
				zap.Error(err))
			w.stats.mu.Lock()
			w.stats.Failed++
			w.stats.mu.Unlock()
		} else {
			w.stats.mu.Lock()
			w.stats.Renewed++
			w.stats.mu.Unlock()
		}

	case domain.ActionChurn:
		if err := w.renewalService.ChurnSubscription(ctx, sub.UserIdentifier, sub.ProductId, "PAYMENT_FAILURE"); err != nil {
			w.logger.Error("Failed to churn subscription",
				zap.String("msisdn", sub.UserIdentifier),
				zap.Error(err))
			w.stats.mu.Lock()
			w.stats.Failed++
			w.stats.mu.Unlock()
		} else {
			w.stats.mu.Lock()
			w.stats.Churned++
			w.stats.mu.Unlock()
		}

	case domain.ActionGracePeriod:
		w.logger.Debug("Subscription in grace period",
			zap.String("msisdn", sub.UserIdentifier))
		w.stats.mu.Lock()
		w.stats.Skipped++
		w.stats.mu.Unlock()

	default:
		w.stats.mu.Lock()
		w.stats.Skipped++
		w.stats.mu.Unlock()
	}
}

// attemptRenewal performs the opt-out/opt-in renewal cycle
func (w *RenewalWorker) attemptRenewal(ctx context.Context, sub *domain.SubscriptionWithRenewalInfo) error {
	// Get product details
	product, err := w.productRepo.GetProduct(sub.ProductId)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	// Determine entry channel
	entryChannel := "WEB"
	if sub.EntryChannel != nil && *sub.EntryChannel != "" {
		entryChannel = *sub.EntryChannel
	}

	// Perform renewal using opt-out/opt-in strategy
	response, err := w.renewalService.SendRenewalRequest(ctx, sub.UserIdentifier, product, entryChannel)
	if err != nil {
		return fmt.Errorf("renewal request failed: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("renewal failed: %s", response.Error)
	}

	// Update renewal attempt count
	w.incrementRenewalAttempt(sub.UserIdentifier, sub.ProductId)

	return nil
}

// incrementRenewalAttempt updates the renewal attempt count
func (w *RenewalWorker) incrementRenewalAttempt(msisdn string, productID string) {
	if err := w.repo.IncrementRenewalAttempt(msisdn, productID); err != nil {
		w.logger.Error("Failed to increment renewal attempt",
			zap.String("msisdn", msisdn),
			zap.String("productId", productID),
			zap.Error(err))
	}
}

// saveMetrics saves processing metrics
func (w *RenewalWorker) saveMetrics() {
	metrics := &domain.RenewalMetrics{
		TotalProcessed:       int64(w.stats.Total),
		SuccessfulRenewals:   int64(w.stats.Renewed),
		FailedRenewals:       int64(w.stats.Failed),
		ChurnedSubscriptions: int64(w.stats.Churned),
		LastRunTime:          time.Now(),
	}

	if w.stats.Total > 0 {
		metrics.SuccessRate = float64(w.stats.Renewed) / float64(w.stats.Total) * 100
	}

	// Save metrics to database
	if err := w.repo.SaveRenewalMetrics(metrics); err != nil {
		w.logger.Error("Failed to save renewal metrics", zap.Error(err))
	}
}

// ProcessPriorityRetryQueue processes failed opt-ins that need immediate retry
func (w *RenewalWorker) ProcessPriorityRetryQueue(ctx context.Context) error {
	w.logger.Info("Processing priority retry queue")

	// Get items due for retry
	items, err := w.repo.GetDuePriorityRetryItems(100)
	if err != nil {
		return fmt.Errorf("failed to get retry items: %w", err)
	}

	if len(items) == 0 {
		w.logger.Debug("No items in priority retry queue")
		return nil
	}

	w.logger.Info("Found items in priority retry queue", zap.Int("count", len(items)))

	for _, item := range items {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Process retry
		if err := w.processRetryItem(ctx, item); err != nil {
			w.logger.Error("Failed to process retry item",
				zap.String("msisdn", item.MSISDN),
				zap.Error(err))
			
			// Update retry count and next retry time
			w.updateRetryItem(item, false)
		} else {
			// Mark as processed
			w.updateRetryItem(item, true)
		}
	}

	return nil
}

// processRetryItem attempts to resubscribe a failed opt-in
func (w *RenewalWorker) processRetryItem(ctx context.Context, item *domain.PriorityRetryQueue) error {
	// Get product details
	product, err := w.productRepo.GetProduct(item.ProductID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	// Attempt opt-in only (user is already opted out)
	cycle := &domain.RenewalCycle{
		MSISDN:    item.MSISDN,
		ProductID: item.ProductID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Get renewal service as concrete type to access OptInForRenewal
	if rs, ok := w.renewalService.(*service.RenewalService); ok {
		err = rs.OptInForRenewal(ctx, item.MSISDN, product, "RETRY", cycle)
		if err != nil {
			return fmt.Errorf("retry opt-in failed: %w", err)
		}
	} else {
		return fmt.Errorf("renewal service type assertion failed")
	}

	w.logger.Info("Successfully resubscribed user from retry queue",
		zap.String("msisdn", item.MSISDN),
		zap.String("productId", item.ProductID))

	return nil
}

// updateRetryItem updates the retry item status
func (w *RenewalWorker) updateRetryItem(item *domain.PriorityRetryQueue, success bool) {
	now := time.Now()
	item.LastAttemptAt = &now
	item.RetryCount++

	if success {
		item.Status = "completed"
	} else {
		// Calculate next retry with exponential backoff
		backoffMinutes := 5 * (1 << uint(item.RetryCount)) // 5, 10, 20, 40, etc.
		if backoffMinutes > 1440 {                         // Cap at 24 hours
			backoffMinutes = 1440
		}
		nextRetry := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)
		item.NextRetryAt = &nextRetry

		// Mark as failed if max retries exceeded
		if item.RetryCount >= 10 {
			item.Status = "failed"
		}
	}

	item.UpdatedAt = time.Now()

	if err := w.repo.UpdatePriorityRetryItem(item); err != nil {
		w.logger.Error("Failed to update retry item",
			zap.String("msisdn", item.MSISDN),
			zap.Error(err))
	}
}

// GetStatus returns the current status of the worker
func (w *RenewalWorker) GetStatus() map[string]interface{} {
	w.mu.RLock()
	isRunning := w.isRunning
	w.mu.RUnlock()

	w.stats.mu.Lock()
	stats := map[string]interface{}{
		"isRunning": isRunning,
		"stats": map[string]int{
			"total":     w.stats.Total,
			"processed": w.stats.Processed,
			"renewed":   w.stats.Renewed,
			"churned":   w.stats.Churned,
			"failed":    w.stats.Failed,
			"skipped":   w.stats.Skipped,
		},
	}
	w.stats.mu.Unlock()

	return stats
}

// IsRunning returns whether the worker is currently running
func (w *RenewalWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// Stop stops the renewal worker
func (w *RenewalWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.isRunning = false
	w.logger.Info("Renewal worker stopped by request")
}

// GetMetrics returns the current processing metrics
func (w *RenewalWorker) GetMetrics() *ProcessingStats {
	w.stats.mu.Lock()
	defer w.stats.mu.Unlock()
	return &ProcessingStats{
		Total:     w.stats.Total,
		Processed: w.stats.Processed,
		Renewed:   w.stats.Renewed,
		Churned:   w.stats.Churned,
		Failed:    w.stats.Failed,
		Skipped:   w.stats.Skipped,
	}
}

// HealthCheck performs a health check on the renewal worker
func (w *RenewalWorker) HealthCheck() map[string]interface{} {
	w.mu.RLock()
	isRunning := w.isRunning
	w.mu.RUnlock()

	health := map[string]interface{}{
		"status":     "healthy",
		"is_running": isRunning,
	}

	// Check if all dependencies are available
	if w.renewalService == nil {
		health["status"] = "degraded"
		health["renewal_service"] = "unavailable"
	} else {
		health["renewal_service"] = "available"
	}

	if w.repo == nil {
		health["status"] = "degraded"
		health["repository"] = "unavailable"
	} else {
		health["repository"] = "available"
	}

	if w.productRepo == nil {
		health["status"] = "degraded"
		health["product_repository"] = "unavailable"
	} else {
		health["product_repository"] = "available"
	}

	return health
}
