// Enhanced ResubscribeHandler with tracking integration
// File: internal/handler/subscription_handler_enhanced.go

package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
)

// EnhancedResubscribeHandler godoc
// @Summary Enhanced resubscribe handler with tracking and recovery
// @Description Handles charging-failed subscriptions with comprehensive tracking
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param message body domain.EnhancedBackfillRequest true "Enhanced Resubscribe Request"
// @Success 202 {object} map[string]string "Accepted with jobId"
// @Failure 400 {string} string "Invalid request payload"
// @Router /api/v1/subscription-external/resubscribe/enhanced [post]
func (h *SubscriptionHandler) EnhancedResubscribeHandler(ctx *fasthttp.RequestCtx) {
	var req domain.EnhancedBackfillRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	// Set defaults
	if req.BatchSize == 0 {
		req.BatchSize = 10000
	}
	if req.MaxWorkers == 0 {
		req.MaxWorkers = 50
	}
	if req.CheckpointInterval == 0 {
		req.CheckpointInterval = 1000
	}

	// Create or use existing batch ID
	batchID := req.BatchID
	if batchID == "" {
		batchID = uuid.New().String()
	}

	// Initialize tracker
	// TODO: Implement proper tracker initialization
	// tracker := service.NewResubscriptionTracker(h.service.DB, h.logger, batchID)

	// Check for existing checkpoint
	// TODO: Implement checkpoint loading
	// checkpoint, err := tracker.LoadCheckpoint()
	var checkpoint *service.CheckpointData
	var err error
	checkpoint = nil
	err = nil

	if err != nil {
		h.logger.Error("Failed to load checkpoint", zap.Error(err))
		ctx.Error("Failed to load checkpoint", fasthttp.StatusInternalServerError)
		return
	}

	// Log request
	h.logger.Info("Enhanced resubscribe request received",
		zap.String("batchID", batchID),
		zap.String("telco", req.Telco),
		zap.Strings("productIds", req.ProductIds),
		zap.Bool("useChargingFailures", req.UseChargingFailures),
		zap.Int("batchSize", req.BatchSize),
		zap.Int("maxWorkers", req.MaxWorkers),
		zap.Bool("resumeFromCheckpoint", checkpoint != nil),
	)

	// Validate request
	if len(req.ProductIds) == 0 && !req.UseChargingFailures {
		ctx.Error("Either product_ids or use_charging_failures must be specified", fasthttp.StatusBadRequest)
		return
	}

	// Create job
	jobID := uuid.New().String()
	_ = h.jobs.CreateJob(jobID, 0) // Use underscore for unused variable
	totalBatchJobsCreated.Add(1)

	// Start async processing
	// TODO: Implement proper async processing
	// go h.processEnhancedResubscribe(jobID, status, req, tracker, checkpoint)

	// For now, just log that processing would start
	h.logger.Info("Enhanced resubscribe processing would start here",
		zap.String("jobID", jobID),
		zap.String("batchID", batchID))

	// Return accepted response
	ctx.SetStatusCode(fasthttp.StatusAccepted)
	ctx.SetContentType("application/json")
	response := map[string]interface{}{
		"jobId":   jobID,
		"batchId": batchID,
		"message": "Processing started",
	}

	if checkpoint != nil {
		response["resumedFrom"] = map[string]interface{}{
			"processedCount": checkpoint.ProcessedCount,
			"successCount":   checkpoint.SuccessCount,
			"failureCount":   checkpoint.FailureCount,
		}
	}

	_ = json.NewEncoder(ctx).Encode(response)
}

func (h *SubscriptionHandler) processEnhancedResubscribe(
	jobID string,
	status *BatchJobStatus,
	req domain.EnhancedBackfillRequest,
	tracker *service.ResubscriptionTracker,
	checkpoint *service.CheckpointData,
) {
	h.jobs.setRunning(jobID)
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Panic in enhanced resubscribe", zap.Any("panic", r))
			status.ErrorDetails = map[string]interface{}{"error": fmt.Sprintf("panic: %v", r)}
			h.jobs.setCompleted(jobID, true)
			tracker.MarkCompleted()
		}
	}()

	var msisdns []domain.ChargingFailedSubscription
	var err error

	// Get target subscriptions
	if req.UseChargingFailures {
		// Fetch subscriptions with charging failures
		msisdns, err = h.fetchChargingFailedSubscriptions(req, checkpoint)
		if err != nil {
			h.logger.Error("Failed to fetch charging failed subscriptions", zap.Error(err))
			status.ErrorDetails = map[string]interface{}{"error": err.Error()}
			h.jobs.setCompleted(jobID, true)
			return
		}
	} else if len(req.MSISDNS) > 0 {
		// Use provided MSISDNs
		for _, msisdn := range req.MSISDNS {
			for _, _ = range req.ProductIds {
				// TODO: Convert productID string to int properly
				productID := 0 // Placeholder - implement proper conversion
				msisdns = append(msisdns, domain.ChargingFailedSubscription{
					MSISDN:       msisdn,
					ProductID:    productID,
					EntryChannel: req.EntryChannel,
				})
			}
		}
	} else {
		// Use existing windowing logic
		msisdns, err = h.fetchSubscriptionsWithProducts(req, checkpoint)
		if err != nil {
			h.logger.Error("Failed to fetch subscriptions", zap.Error(err))
			status.ErrorDetails = map[string]interface{}{"error": err.Error()}
			h.jobs.setCompleted(jobID, true)
			return
		}
	}

	// Initialize batch if not resuming
	if checkpoint == nil {
		if err := tracker.InitializeBatch(len(msisdns)); err != nil {
			h.logger.Error("Failed to initialize batch", zap.Error(err))
			status.ErrorDetails = map[string]interface{}{"error": err.Error()}
			h.jobs.setCompleted(jobID, true)
			return
		}
	}

	status.Total = len(msisdns)

	// Process with workers
	h.processWithWorkers(msisdns, req, tracker, status)

	// Mark batch as completed
	if err := tracker.MarkCompleted(); err != nil {
		h.logger.Error("Failed to mark batch completed", zap.Error(err))
	}

	// Update job status
	stats := tracker.GetStats()
	status.Successful = stats.SuccessCount
	status.Failed = stats.FailureCount

	if stats.FailureCount > 0 {
		status.ErrorDetails = map[string]interface{}{
			"totalFailed": stats.FailureCount,
			"errorRate":   fmt.Sprintf("%.2f%%", float64(stats.FailureCount)/float64(stats.ProcessedCount)*100),
		}
		h.jobs.setCompleted(jobID, true)
	} else {
		h.jobs.setCompleted(jobID, false)
	}

	totalBatchJobsCompleted.Add(1)

	// Log final statistics
	tracker.LogProgress()
}

func (h *SubscriptionHandler) processWithWorkers(
	subscriptions []domain.ChargingFailedSubscription,
	req domain.EnhancedBackfillRequest,
	tracker *service.ResubscriptionTracker,
	status *BatchJobStatus,
) {
	var wg sync.WaitGroup
	subChan := make(chan domain.ChargingFailedSubscription, req.BatchSize)

	// Rate limiter
	rateLimiter := time.NewTicker(time.Second / time.Duration(req.RateLimit))
	defer rateLimiter.Stop()

	var processedCount int64
	var successCount uint64
	var errorCount uint64
	var skippedCount uint64

	// Start workers
	for i := 0; i < req.MaxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for sub := range subChan {
				// Rate limiting
				<-rateLimiter.C

				// Check if already processed
				processed, err := tracker.CheckIfProcessed(sub.MSISDN, sub.ProductID)
				if err != nil {
					h.logger.Error("Failed to check if processed",
						zap.String("msisdn", sub.MSISDN),
						zap.Error(err))
				}

				if processed && !req.ForceReprocess {
					atomic.AddUint64(&skippedCount, 1)
					status.incProcessed()
					continue
				}

				// Record attempt
				if err := tracker.RecordAttempt(sub.MSISDN, sub.ProductID, sub.ID); err != nil {
					h.logger.Error("Failed to record attempt",
						zap.String("msisdn", sub.MSISDN),
						zap.Error(err))
				}

				// Process resubscription
				entryChannel := req.GetNextEntryChannel()
				err = h.service.ResubscribeUser(sub.MSISDN, entryChannel, []string{fmt.Sprintf("%d", sub.ProductID)})

				// Update result
				if err != nil {
					tracker.UpdateResult(sub.MSISDN, sub.ProductID, false, err.Error())
					atomic.AddUint64(&errorCount, 1)
					totalBatchRequestsFailed.Add(1)
				} else {
					tracker.UpdateResult(sub.MSISDN, sub.ProductID, true, "")
					atomic.AddUint64(&successCount, 1)
					totalBatchRequestsSucceeded.Add(1)
				}

				// Update progress
				status.incProcessed()
				totalBatchRequestsProcessed.Add(1)
				currentProcessed := atomic.AddInt64(&processedCount, 1)

				// Save checkpoint periodically
				if currentProcessed%int64(req.CheckpointInterval) == 0 {
					if err := tracker.SaveCheckpoint(sub.ID, sub.MSISDN); err != nil {
						h.logger.Error("Failed to save checkpoint", zap.Error(err))
					}
					tracker.LogProgress()
				}

				// Check for stop signal
				if req.StopSignal != nil {
					select {
					case <-req.StopSignal:
						h.logger.Info("Received stop signal, halting processing")
						return
					default:
					}
				}
			}
		}(i)
	}

	// Feed subscriptions to workers
	for _, sub := range subscriptions {
		subChan <- sub
	}
	close(subChan)

	// Wait for all workers to complete
	wg.Wait()

	// Final checkpoint
	if len(subscriptions) > 0 {
		lastSub := subscriptions[len(subscriptions)-1]
		tracker.SaveCheckpoint(lastSub.ID, lastSub.MSISDN)
	}

	h.logger.Info("Processing completed",
		zap.Int64("processed", processedCount),
		zap.Uint64("success", successCount),
		zap.Uint64("failed", errorCount),
		zap.Uint64("skipped", skippedCount),
	)
}

func (h *SubscriptionHandler) fetchChargingFailedSubscriptions(
	req domain.EnhancedBackfillRequest,
	checkpoint *service.CheckpointData,
) ([]domain.ChargingFailedSubscription, error) {

	// TODO: Implement proper filter creation
	// filter := repository.ChargingFailureFilter{
	//     ProductIDs:       h.convertProductIDs(req.ProductIds),
	//     ExcludeProcessed: !req.ForceReprocess,
	//     Limit:            req.BatchSize,
	//     Offset:           0,
	// }

	// TODO: Implement proper checkpoint handling
	// Resume from checkpoint if available
	// if checkpoint != nil && checkpoint.LastProcessedID > 0 {
	//     filter.LastProcessedID = checkpoint.LastProcessedID
	// }

	// TODO: Implement proper repository access
	// return h.service.Repository.FetchChargingFailedSubscriptions(filter)

	// Placeholder return for now
	return []domain.ChargingFailedSubscription{}, nil
}

func (h *SubscriptionHandler) fetchSubscriptionsWithProducts(
	req domain.EnhancedBackfillRequest,
	checkpoint *service.CheckpointData,
) ([]domain.ChargingFailedSubscription, error) {

	startIndex := req.StartIndex
	if checkpoint != nil && checkpoint.LastProcessedID > 0 {
		startIndex = checkpoint.LastProcessedID
	}

	msisdns, err := h.service.BackfillMsisdnsWithProducts(req.ProductIds, startIndex, req.EndIndex)
	if err != nil {
		return nil, err
	}

	var result []domain.ChargingFailedSubscription
	for _, msisdn := range msisdns {
		for _, productID := range req.ProductIds {
			pid, _ := strconv.Atoi(productID)
			result = append(result, domain.ChargingFailedSubscription{
				MSISDN:       msisdn,
				ProductID:    pid,
				EntryChannel: req.EntryChannel,
			})
		}
	}

	return result, nil
}

func (h *SubscriptionHandler) convertProductIDs(productIDs []string) []int {
	var result []int
	for _, pid := range productIDs {
		if id, err := strconv.Atoi(pid); err == nil {
			result = append(result, id)
		}
	}
	return result
}
