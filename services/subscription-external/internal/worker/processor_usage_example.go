package worker

import (
	"context"
	"database/sql"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/monitoring"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"go.uber.org/zap"
)

// ExampleUsage demonstrates how to use the enhanced ResubscriptionProcessor
func ExampleUsage() {
	// Initialize dependencies
	logger, _ := zap.NewDevelopment()
	db, _ := sql.Open("postgres", "postgres://user:pass@localhost/dbname?sslmode=disable")

	// Create repository
	// Note: In production, use a fully implemented SubscriptionRepositoryInterface
	var repo repository.SubscriptionRepositoryInterface
	_ = repository.NewSubscriptionRepository(db, logger, nil) // For type reference only
	repo = nil                                                 // Set to nil for example - in production, use a properly configured repo

	// Create service (you'll need to provide the actual dependencies)
	var subscriptionService *service.SubscriptionService
	// subscriptionService = service.NewSubscriptionService(...)

	// Create monitor
	monitor := monitoring.NewChargingFailureMonitor(logger)

	// Create processor with enhanced configuration
	config := &ProcessingConfig{
		BatchSize:              1000, // Process 1000 subscriptions per batch
		MaxConcurrency:         10,   // Use 10 concurrent workers
		RetryAttempts:          3,    // Retry failed attempts 3 times
		RetryDelay:             30 * time.Second,
		ProcessingTimeout:      2 * time.Minute,
		PriorityProcessing:     true,                   // Process by priority
		SkipProcessed:          true,                   // Skip already processed subscriptions
		UpdateChargingHealth:   true,                   // Update charging health status
		CheckpointInterval:     100,                    // Save checkpoint every 100 records
		BatchID:                "batch_20241201_001",   // Unique batch identifier
		BatchDelay:             200 * time.Millisecond, // Delay between batches
		MaxQueueSize:           2000,                   // Larger queue for high throughput
		ProgressReportInterval: 15 * time.Second,       // More frequent progress updates
		ResultsBufferSize:      2000,                   // Larger buffer for results
	}

	processor := NewResubscriptionProcessor(
		repo,
		subscriptionService,
		monitor,
		logger,
		config,
	)

	// Start processing
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	err := processor.Start(ctx)
	if err != nil {
		logger.Error("Failed to start processor", zap.Error(err))
		return
	}

	// Monitor progress with enhanced reporting
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Get detailed status
				status := processor.GetDetailedStatus()
				logger.Info("Processing status", zap.Any("status", status))

				// Get processing summary
				summary := processor.GetProcessingSummary()
				logger.Info("Processing summary", zap.Any("summary", summary))
			}
		}
	}()

	// Example of dynamic configuration update
	go func() {
		time.Sleep(5 * time.Minute) // Wait 5 minutes

		// Update configuration for better performance
		newConfig := &ProcessingConfig{
			BatchSize:          2000,                   // Increase batch size
			MaxConcurrency:     15,                     // Increase concurrency
			CheckpointInterval: 200,                    // Less frequent checkpoints
			BatchDelay:         100 * time.Millisecond, // Reduce delay
		}

		if err := processor.UpdateConfig(newConfig); err != nil {
			logger.Error("Failed to update configuration", zap.Error(err))
		} else {
			logger.Info("Configuration updated successfully")
		}
	}()

	// Example of pause/resume functionality
	go func() {
		time.Sleep(10 * time.Minute) // Wait 10 minutes

		logger.Info("Pausing processing for maintenance")
		processor.Pause()

		time.Sleep(2 * time.Minute) // Pause for 2 minutes

		logger.Info("Resuming processing")
		if err := processor.Resume(ctx); err != nil {
			logger.Error("Failed to resume processing", zap.Error(err))
		}
	}()

	// Wait for completion or context cancellation
	<-ctx.Done()

	// Stop the processor
	processor.Stop()

	// Get final summary
	finalSummary := processor.GetProcessingSummary()
	logger.Info("Processing completed", zap.Any("final_summary", finalSummary))
}

// ExampleWithCustomTracker demonstrates using a custom tracker implementation
func ExampleWithCustomTracker() {
	logger, _ := zap.NewDevelopment()

	// Create a custom tracker (could be in-memory, Redis, etc.)
	customTracker := &CustomResubscriptionTracker{
		logger: logger,
		stats:  &CheckpointData{},
	}

	// Create processor without automatic tracker creation
	processor := NewResubscriptionProcessor(
		nil, // No repository needed if using custom tracker
		nil, // No service needed for this example
		nil, // No monitor needed for this example
		logger,
		&ProcessingConfig{
			BatchSize:          100,
			MaxConcurrency:     5,
			CheckpointInterval: 50,
		},
	)

	// Set the custom tracker
	processor.SetTracker(customTracker)

	// Use the processor...
}

// CustomResubscriptionTracker is an example of a custom tracker implementation
type CustomResubscriptionTracker struct {
	logger *zap.Logger
	stats  *CheckpointData
}

func (t *CustomResubscriptionTracker) InitializeBatch(totalCount int) error {
	t.stats.TotalCount = totalCount
	t.stats.Status = "in_progress"
	t.logger.Info("Custom tracker initialized", zap.Int("totalCount", totalCount))
	return nil
}

func (t *CustomResubscriptionTracker) CheckIfProcessed(msisdn string, productID int) (bool, error) {
	// Custom logic to check if processed
	return false, nil
}

func (t *CustomResubscriptionTracker) RecordAttempt(msisdn string, productID int, subscriptionID int) error {
	// Custom logic to record attempt
	return nil
}

func (t *CustomResubscriptionTracker) UpdateResult(msisdn string, productID int, success bool, errorMessage string) error {
	// Custom logic to update result
	if success {
		t.stats.SuccessCount++
	} else {
		t.stats.FailureCount++
	}
	return nil
}

func (t *CustomResubscriptionTracker) SaveCheckpoint(subscriptionID int, msisdn string) error {
	// Custom logic to save checkpoint
	t.stats.LastProcessedID = subscriptionID
	t.stats.LastProcessedMSISDN = msisdn
	return nil
}

func (t *CustomResubscriptionTracker) LoadCheckpoint() (*CheckpointData, error) {
	// Custom logic to load checkpoint
	return t.stats, nil
}

func (t *CustomResubscriptionTracker) MarkCompleted() error {
	// Custom logic to mark completed
	t.stats.Status = "completed"
	return nil
}

func (t *CustomResubscriptionTracker) GetStats() *CheckpointData {
	// Return current stats
	return t.stats
}

func (t *CustomResubscriptionTracker) LogProgress() {
	// Custom progress logging
	t.logger.Info("Custom tracker progress",
		zap.Int("processed", t.stats.ProcessedCount),
		zap.Int("success", t.stats.SuccessCount),
		zap.Int("failed", t.stats.FailureCount))
}
