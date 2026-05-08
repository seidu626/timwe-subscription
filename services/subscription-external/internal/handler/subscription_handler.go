package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/common/config"
	"go.uber.org/zap"

	"sync/atomic"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
	"github.com/valyala/fasthttp"
)

type SubscriptionHandler struct {
	service         *service.SubscriptionService
	config          *config.Config
	logger          *zap.Logger
	jobs            *BatchJobManager
	processOptinFn  func(*domain.OptinRequest) error // optional override for tests
	msisdnGenerator *utils.OptimizedMSISDNGenerator  // optimized MSISDN generator
	startTime       time.Time
}

// normalizeEntryChannels ensures proper entry channel configuration for backward compatibility
func (h *SubscriptionHandler) normalizeEntryChannels(req *domain.BackfillRequest) {
	// If EntryChannels is not set but EntryChannel is, use EntryChannel as the only channel
	if len(req.EntryChannels) == 0 && req.EntryChannel != "" {
		req.EntryChannels = []string{req.EntryChannel}
	}
	// If EntryChannels is set but EntryChannel is not, set EntryChannel to the first channel for backward compatibility
	if len(req.EntryChannels) > 0 && req.EntryChannel == "" {
		req.EntryChannel = req.EntryChannels[0]
	}
	// If neither is set, use default
	if len(req.EntryChannels) == 0 && req.EntryChannel == "" {
		req.EntryChannels = []string{"USSD"}
		req.EntryChannel = "USSD"
	}
}

func NewSubscriptionHandler(logger *zap.Logger, service *service.SubscriptionService, c *config.Config) *SubscriptionHandler {
	// Initialize the optimized MSISDN generator with Bloom Filter
	var bloomFilter *utils.MSISDNBloomFilter

	// Try to initialize Bloom Filter with Redis if available
	if c.Cache.Redis.Host != "" && c.Cache.Redis.Port != 0 {
		// Create failover Redis client for Bloom Filter
		redisClient := cached.NewFailoverRedisClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%d", c.Cache.Redis.Host, c.Cache.Redis.Port),
			DB:   c.Cache.Redis.DB,
		})

		// Test Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := redisClient.Ping(ctx); err != nil {
			logger.Warn("Redis connection failed, Bloom Filter disabled",
				zap.String("host", c.Cache.Redis.Host),
				zap.Int("port", c.Cache.Redis.Port),
				zap.Error(err))
		} else {
			// Use configurable values
			expectedItems := uint(1000000) // Default: 1M items for production scale
			falsePositiveRate := 0.01      // Default: 1% false positive rate

			// Override with configuration values if available
			if c.Application.MSISDNGenerator.BloomFilterEnabled {
				if c.Application.MSISDNGenerator.FalsePositiveRate > 0 {
					falsePositiveRate = c.Application.MSISDNGenerator.FalsePositiveRate
				}
				// Use preload batch size as a guide for expected items
				if c.Application.MSISDNGenerator.PreloadBatchSize > 0 {
					expectedItems = uint(c.Application.MSISDNGenerator.PreloadBatchSize * 100) // Scale up for production
				}
			}

			bloomFilter = utils.NewMSISDNBloomFilter(
				expectedItems,
				falsePositiveRate,
				redisClient,
				logger,
			)
			logger.Info("Bloom Filter initialized with Redis",
				zap.String("host", c.Cache.Redis.Host),
				zap.Int("port", c.Cache.Redis.Port),
				zap.String("cacheMode", string(redisClient.Mode())),
				zap.Uint("expectedItems", expectedItems),
				zap.Float64("falsePositiveRate", falsePositiveRate))
		}
	} else {
		logger.Info("Bloom Filter disabled - Redis not configured")
	}

	// Initialize the optimized MSISDN generator with configuration values
	batchSize := c.Application.MSISDNGenerator.BatchSize
	if batchSize <= 0 {
		batchSize = 1000 // Default fallback
	}
	maxConcurrent := c.Application.MSISDNGenerator.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 50 // Default fallback
	}

	msisdnGenerator := utils.NewOptimizedMSISDNGenerator(
		bloomFilter,
		service.UserBaseRepository,
		logger,
		batchSize,
		maxConcurrent,
	)

	handler := &SubscriptionHandler{
		logger:          logger,
		service:         service,
		config:          c,
		jobs:            NewBatchJobManager(),
		msisdnGenerator: msisdnGenerator,
		startTime:       time.Now(),
	}

	// Preload Bloom Filter if available (always enabled for now)
	if bloomFilter != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			if err := msisdnGenerator.PreloadBloomFilter(ctx); err != nil {
				logger.Error("Failed to preload Bloom Filter", zap.Error(err))
			} else {
				logger.Info("Bloom Filter preload completed successfully")
			}
		}()
	}

	return handler
}

// OptinHandler godoc
// @Summary Opt-in a single subscription
// @Description Opt-in a user subscription with the provided details
// @Tags Subscriptions
// @Accept  json
// @Produce  json
// @Param message body domain.OptinRequest true "Optin Request"
// @Success 200 string  "200"
// @Failure 400 {object} error "Invalid\t query parameters"
// @Failure 500 {object} error "Internal server error"
// @Router /api/v1/subscription-external [post]
func (h *SubscriptionHandler) OptinHandler(ctx *fasthttp.RequestCtx) {
	var req domain.OptinRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	process := h.service.ProcessOptin
	if h.processOptinFn != nil {
		process = h.processOptinFn
	}
	err := process(&req)
	if err != nil {
		h.logger.Error("Failed to subscribe user", zap.Any("request", req), zap.Error(err))

		// Check if it's an MTResponseError to provide specific error handling
		if mtErr, ok := err.(*domain.MTResponseError); ok {
			// Return specific error response based on the error type
			errorResponse := map[string]interface{}{
				"status":  "error",
				"message": mtErr.Message,
				"code":    mtErr.Code,
				"details": mtErr.Details,
			}

			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			if err := json.NewEncoder(ctx).Encode(errorResponse); err != nil {
				h.logger.Error("Failed to encode error response", zap.Error(err))
				ctx.Error("Failed to format error response", fasthttp.StatusInternalServerError)
				return
			}
			return
		}

		// For other types of errors, return generic error
		ctx.Error("Failed to subscribe user", fasthttp.StatusBadRequest)
		return
	}

	// Create proper JSON response
	response := domain.SubscribeResponse{
		Status:  "success",
		Message: "Successfully processed subscription",
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		ctx.Error("Failed to format response", fasthttp.StatusInternalServerError)
		return
	}
}

// BatchOptinHandler godoc
// @Summary Batch Opt-in subscriptions (async)
// @Description Enqueue a batch job and return jobId to poll status.
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param message body domain.BatchOptinRequest true "Batch Optin Request"
// @Success 202 {object} map[string]string "Accepted with jobId"
// @Failure 400 {string} string "Invalid request payload or error generating MSISDNS"
// @Router /api/v1/subscription-external/batch [post]
func (h *SubscriptionHandler) BatchOptinHandler(ctx *fasthttp.RequestCtx) {
	var req domain.BatchOptinRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	// Create job immediately and return
	jobID := uuid.New().String()
	initialTotal := len(req.MSISDNS)
	if initialTotal == 0 && req.Count > 0 {
		initialTotal = req.Count
	}
	status := h.jobs.CreateJob(jobID, initialTotal)
	totalBatchJobsCreated.Add(1)

	// Start background processing (generation/filtering happens inside)
	go h.runBatchJob(jobID, status, &req)

	ctx.SetStatusCode(fasthttp.StatusAccepted)
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(map[string]string{"jobId": jobID})
}

// Batch status endpoint
func (h *SubscriptionHandler) BatchStatusHandler(ctx *fasthttp.RequestCtx) {
	jobID := string(ctx.QueryArgs().Peek("jobId"))
	if jobID == "" {
		ctx.Error("jobId is required", fasthttp.StatusBadRequest)
		return
	}
	if st, ok := h.jobs.GetJob(jobID); ok {
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
		_ = json.NewEncoder(ctx).Encode(st)
		return
	}
	ctx.Error("job not found", fasthttp.StatusNotFound)
}

// BackfillOptinHandler godoc
// @Summary Backfill subscriptions for MSISDNs missing specified products (async)
// @Description If msisdns not provided, finds active MSISDNs missing the provided product_ids and enqueues opt-in jobs.
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param message body domain.BackfillRequest true "Backfill Request"
// @Success 202 {object} map[string]string "Accepted with jobId"
// @Failure 400 {string} string "Invalid request payload"
// @Router /api/v1/subscription-external/backfill [post]
func (h *SubscriptionHandler) BackfillOptinHandler(ctx *fasthttp.RequestCtx) {
	var req domain.BackfillRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	// Normalize entry channels configuration
	h.normalizeEntryChannels(&req)

	h.logger.Info("Backfill request received",
		zap.String("telco", req.Telco),
		zap.Strings("productIds", req.ProductIds),
		zap.String("entryChannel", req.EntryChannel),
		zap.Strings("entryChannels", req.EntryChannels),
		zap.Int("msisdnCount", len(req.MSISDNS)),
		zap.Int("startIndex", req.StartIndex),
		zap.Int("endIndex", req.EndIndex),
	)

	if len(req.ProductIds) == 0 {
		ctx.Error("product_ids is required", fasthttp.StatusBadRequest)
		return
	}
	jobID := uuid.New().String()
	initialTotal := len(req.MSISDNS)
	status := h.jobs.CreateJob(jobID, initialTotal)
	totalBatchJobsCreated.Add(1)

	go func(job string, st *BatchJobStatus, r domain.BackfillRequest) {
		h.jobs.setRunning(job)
		msisdns := r.MSISDNS
		if len(msisdns) == 0 {
			// Fetch target MSISDNs missing these products using windowing
			fetched, err := h.service.BackfillMsisdnsMissingSomeProducts(r.ProductIds, r.StartIndex, r.EndIndex)
			if err != nil {
				h.logger.Error("Failed to fetch backfill msisdns", zap.Error(err))
				st.ErrorDetails = map[string]interface{}{"error": err.Error()}
				h.jobs.setCompleted(job, true)
				return
			}
			msisdns = fetched
		} else {
			// MSISDNS provided explicitly; apply local slicing if requested
			if r.StartIndex == 0 || r.StartIndex == -1 {
				// full list
			} else if r.StartIndex > 0 {
				if r.StartIndex < len(msisdns) {
					msisdns = msisdns[r.StartIndex:]
				} else {
					msisdns = []string{}
				}
			}
			if r.EndIndex > 0 && r.EndIndex <= len(msisdns) {
				msisdns = msisdns[:r.EndIndex]
			}
		}

		// Filter against exclusion list/user base if available
		if len(msisdns) > 0 {
			filtered, err := h.service.UserBaseRepository.FilterMSISDNS(msisdns)
			if err != nil {
				h.logger.Error("Failed to filter msisdns", zap.Error(err))
				st.ErrorDetails = map[string]interface{}{"error": err.Error()}
				h.jobs.setCompleted(job, true)
				return
			}
			msisdns = filtered
		}

		st.Total = len(msisdns)

		maxWorkers := calculateOptimalWorkers(len(msisdns))
		batchSize := calculateOptimalBatchSize(len(msisdns))

		var wg sync.WaitGroup
		optinRequestChan := make(chan *domain.OptinRequest, batchSize)

		var firstErrorDetails map[string]interface{}
		var firstErrorMutex sync.Mutex
		var successCount uint64
		var errorCount uint64

		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for request := range optinRequestChan {
					if err := h.service.ProcessOptin(request); err != nil {
						firstErrorMutex.Lock()
						if firstErrorDetails == nil {
							if mtErr, ok := err.(*domain.MTResponseError); ok {
								firstErrorDetails = mtErr.Details
							} else {
								firstErrorDetails = map[string]interface{}{"error": err.Error()}
							}
						}
						firstErrorMutex.Unlock()
						atomic.AddUint64(&errorCount, 1)
						totalBatchRequestsFailed.Add(1)
					} else {
						atomic.AddUint64(&successCount, 1)
						totalBatchRequestsSucceeded.Add(1)
					}
					st.incProcessed()
					totalBatchRequestsProcessed.Add(1)
				}
			}(i)
		}

		for _, msisdn := range msisdns {
			entryChannel := r.GetNextEntryChannel()
			optinRequestChan <- &domain.OptinRequest{
				Telco:        r.Telco,
				Msisdn:       msisdn,
				EntryChannel: entryChannel,
				ProductIds:   r.ProductIds,
			}
			// Log every 100th request to avoid excessive logging
			if len(msisdns) > 100 && len(msisdns)%100 == 0 {
				h.logger.Debug("Created optin request", zap.String("msisdn", msisdn), zap.String("entryChannel", entryChannel))
			}
		}
		close(optinRequestChan)
		wg.Wait()

		st.Successful = int64(successCount)
		st.Failed = int64(errorCount)
		if errorCount > 0 && firstErrorDetails != nil {
			st.ErrorDetails = firstErrorDetails
			h.jobs.setCompleted(job, true)
		} else {
			h.jobs.setCompleted(job, false)
		}
		totalBatchJobsCompleted.Add(1)
	}(jobID, status, req)

	ctx.SetStatusCode(fasthttp.StatusAccepted)
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(map[string]string{"jobId": jobID})
}

// ResubscribeHandler godoc
// @Summary Unsubscribe then subscribe again for targeted MSISDNs (async)
// @Description If msisdns not provided, finds active MSISDNs WITH the provided product_ids using windowing and performs unsubscribe then opt-in again.
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param message body domain.BackfillRequest true "Resubscribe Request"
// @Success 202 {object} map[string]string "Accepted with jobId"
// @Failure 400 {string} string "Invalid request payload"
// @Router /api/v1/subscription-external/resubscribe [post]
func (h *SubscriptionHandler) ResubscribeHandler(ctx *fasthttp.RequestCtx) {
	var req domain.BackfillRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	// Normalize entry channels configuration
	h.normalizeEntryChannels(&req)

	h.logger.Info("Resubscribe request received",
		zap.String("telco", req.Telco),
		zap.Strings("productIds", req.ProductIds),
		zap.String("entryChannel", req.EntryChannel),
		zap.Strings("entryChannels", req.EntryChannels),
		zap.Int("msisdnCount", len(req.MSISDNS)),
		zap.Int("startIndex", req.StartIndex),
		zap.Int("endIndex", req.EndIndex),
	)

	if len(req.ProductIds) == 0 {
		ctx.Error("product_ids is required", fasthttp.StatusBadRequest)
		return
	}
	jobID := uuid.New().String()
	status := h.jobs.CreateJob(jobID, 0)
	totalBatchJobsCreated.Add(1)

	go func(job string, st *BatchJobStatus, r domain.BackfillRequest) {
		h.jobs.setRunning(job)
		msisdns := r.MSISDNS
		if len(msisdns) == 0 {
			// Fetch target MSISDNs that already have these products
			fetched, err := h.service.BackfillMsisdnsWithProducts(r.ProductIds, r.StartIndex, r.EndIndex)
			if err != nil {
				h.logger.Error("Failed to fetch resubscribe msisdns", zap.Error(err))
				st.ErrorDetails = map[string]interface{}{"error": err.Error()}
				h.jobs.setCompleted(job, true)
				return
			}
			msisdns = fetched
		} else {
			// Apply local slicing if requested
			if r.StartIndex == 0 || r.StartIndex == -1 {
				// full list
			} else if r.StartIndex > 0 {
				if r.StartIndex < len(msisdns) {
					msisdns = msisdns[r.StartIndex:]
				} else {
					msisdns = []string{}
				}
			}
			if r.EndIndex > 0 && r.EndIndex <= len(msisdns) {
				msisdns = msisdns[:r.EndIndex]
			}
		}

		// Filter against exclusion list/user base if available
		if len(msisdns) > 0 {
			filtered, err := h.service.UserBaseRepository.FilterMSISDNS(msisdns)
			if err != nil {
				h.logger.Error("Failed to filter msisdns", zap.Error(err))
				st.ErrorDetails = map[string]interface{}{"error": err.Error()}
				h.jobs.setCompleted(job, true)
				return
			}
			msisdns = filtered
		}

		//h.logger.Info("Filtered msisdns for resubscribe", zap.Any("msisdns", msisdns))
		st.Total = len(msisdns)

		maxWorkers := calculateOptimalWorkers(len(msisdns))
		batchSize := calculateOptimalBatchSize(len(msisdns))

		var wg sync.WaitGroup
		msisdnChan := make(chan string, batchSize)
		entryChannelChan := make(chan string, batchSize)

		var firstErrorDetails map[string]interface{}
		var firstErrorMutex sync.Mutex
		var successCount uint64
		var errorCount uint64

		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for msisdn := range msisdnChan {
					entryChannel := <-entryChannelChan
					if err := h.service.ResubscribeUser(msisdn, entryChannel, r.ProductIds); err != nil {
						firstErrorMutex.Lock()
						if firstErrorDetails == nil {
							firstErrorDetails = map[string]interface{}{"error": err.Error()}
						}
						firstErrorMutex.Unlock()
						atomic.AddUint64(&errorCount, 1)
						totalBatchRequestsFailed.Add(1)
					} else {
						atomic.AddUint64(&successCount, 1)
						totalBatchRequestsSucceeded.Add(1)
					}
					st.incProcessed()
					totalBatchRequestsProcessed.Add(1)
				}
			}(i)
		}

		for _, msisdn := range msisdns {
			entryChannel := r.GetNextEntryChannel()
			msisdnChan <- msisdn
			entryChannelChan <- entryChannel
			// Log every 100th request to avoid excessive logging
			if len(msisdns) > 100 && len(msisdns)%100 == 0 {
				h.logger.Debug("Created resubscribe request", zap.String("msisdn", msisdn), zap.String("entryChannel", entryChannel))
			}
		}
		close(msisdnChan)
		close(entryChannelChan)
		wg.Wait()

		st.Successful = int64(successCount)
		st.Failed = int64(errorCount)
		if errorCount > 0 && firstErrorDetails != nil {
			st.ErrorDetails = firstErrorDetails
			h.jobs.setCompleted(job, true)
		} else {
			h.jobs.setCompleted(job, false)
		}
		totalBatchJobsCompleted.Add(1)
	}(jobID, status, req)

	ctx.SetStatusCode(fasthttp.StatusAccepted)
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(map[string]string{"jobId": jobID})
}

// GetChargingFailuresHandler returns subscriptions with charging failures
// @Summary Get charging failed subscriptions
// @Description Get subscriptions with charging failures using notifications-based analysis with filtering and pagination
// @Tags ChargingFailures
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of results to return (default: 100)"
// @Param offset query int false "Number of results to skip (default: 0)"
// @Param days_threshold query int false "Filter by days since last charge notification (default: 30)"
// @Param status query string false "Filter by subscription status"
// @Param health_status query string false "Filter by charging health status"
// @Success 200 {object} map[string]interface{} "Charging failures retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/charging-failures [get]
func (h *SubscriptionHandler) GetChargingFailuresHandler(ctx *fasthttp.RequestCtx) {
	// Parse query parameters
	limit := 100 // default limit
	if limitStr := string(ctx.QueryArgs().Peek("limit")); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if offsetStr := string(ctx.QueryArgs().Peek("offset")); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}

	daysThreshold := 30 // default threshold
	if daysStr := string(ctx.QueryArgs().Peek("days_threshold")); daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil && parsed > 0 {
			daysThreshold = parsed
		}
	}

	// Create filter
	filter := repository.ChargingFailureFilter{
		Limit:         limit,
		Offset:        offset,
		DaysThreshold: daysThreshold,
	}

	// Get repository from service
	repo := h.service.GetRepository()

	// Get charging failures from repository
	chargingFailures, err := repo.FetchChargingFailedSubscriptions(filter)
	if err != nil {
		h.logger.Error("Failed to fetch charging failures", zap.Error(err))
		ctx.Error("Failed to fetch charging failures", fasthttp.StatusInternalServerError)
		return
	}

	// Get total count
	totalCount, err := repo.GetChargingFailureCount(filter)
	if err != nil {
		h.logger.Error("Failed to get charging failure count", zap.Error(err))
		ctx.Error("Failed to get charging failure count", fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":            "success",
		"message":           "Charging failures retrieved successfully",
		"implementation":    "notifications-based",
		"total_count":       totalCount,
		"returned_count":    len(chargingFailures),
		"limit":             limit,
		"offset":            offset,
		"days_threshold":    daysThreshold,
		"charging_failures": chargingFailures,
		"timestamp":         time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(response)
}

// GetChargingFailureStatsHandler returns statistics about charging failures
// @Summary Get charging failure statistics
// @Description Get comprehensive statistics about charging failures including counts, rates, and trends
// @Tags ChargingFailures
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Charging failure statistics retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/charging-failures/stats [get]
func (h *SubscriptionHandler) GetChargingFailureStatsHandler(ctx *fasthttp.RequestCtx) {
	// Get repository from service
	repo := h.service.GetRepository()

	// Get charging failure statistics
	stats, err := repo.GetChargingFailureStats()
	if err != nil {
		h.logger.Error("Failed to get charging failure stats", zap.Error(err))
		ctx.Error("Failed to get charging failure stats", fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":         "success",
		"message":        "Charging failure statistics retrieved successfully",
		"implementation": "notifications-based",
		"statistics":     stats,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(response)
}

// GetChargingFailureSummaryHandler returns a summary view of charging failures
// @Summary Get charging failure summary
// @Description Get a categorized summary of charging failures by health status and failure reasons
// @Tags ChargingFailures
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Charging failure summary retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/charging-failures/summary [get]
func (h *SubscriptionHandler) GetChargingFailureSummaryHandler(ctx *fasthttp.RequestCtx) {
	// Get repository from service
	repo := h.service.GetRepository()

	// Get charging failure summary
	summary, err := repo.GetChargingFailureSummary()
	if err != nil {
		h.logger.Error("Failed to get charging failure summary", zap.Error(err))
		ctx.Error("Failed to get charging failure summary", fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":         "success",
		"message":        "Charging failure summary retrieved successfully",
		"implementation": "notifications-based",
		"summary":        summary,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(response)
}

// GetChargingFailureByMSISDNHandler returns charging failure information for a specific MSISDN
func (h *SubscriptionHandler) GetChargingFailureByMSISDNHandler(ctx *fasthttp.RequestCtx) {
	// Parse MSISDN from query parameters
	msisdn := string(ctx.QueryArgs().Peek("msisdn"))
	if msisdn == "" {
		ctx.Error("MSISDN parameter is required", fasthttp.StatusBadRequest)
		return
	}

	// Parse product ID from query parameters
	productIDStr := string(ctx.QueryArgs().Peek("product_id"))
	productID := 0
	if productIDStr != "" {
		if parsed, err := strconv.Atoi(productIDStr); err == nil {
			productID = parsed
		}
	}

	// Get repository from service
	repo := h.service.GetRepository()

	// Get charging failure by MSISDN
	chargingFailure, err := repo.GetChargingFailureByMSISDN(msisdn, productID)
	if err != nil {
		h.logger.Error("Failed to get charging failure by MSISDN", zap.String("msisdn", msisdn), zap.Error(err))
		ctx.Error("Failed to get charging failure by MSISDN", fasthttp.StatusInternalServerError)
		return
	}

	if chargingFailure == nil {
		ctx.Error("No charging failure found for the specified MSISDN", fasthttp.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"status":           "success",
		"message":          "Charging failure retrieved successfully",
		"implementation":   "notifications-based",
		"charging_failure": chargingFailure,
		"timestamp":        time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(response)
}

// UpdateChargingHealthStatusHandler updates the charging health status for a subscription
func (h *SubscriptionHandler) UpdateChargingHealthStatusHandler(ctx *fasthttp.RequestCtx) {
	// Parse request body
	var req struct {
		SubscriptionID int    `json:"subscription_id"`
		Status         string `json:"status"`
		Reason         string `json:"reason"`
	}

	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	if req.SubscriptionID <= 0 {
		ctx.Error("Valid subscription_id is required", fasthttp.StatusBadRequest)
		return
	}

	if req.Status == "" {
		ctx.Error("Status is required", fasthttp.StatusBadRequest)
		return
	}

	// Get repository from service
	repo := h.service.GetRepository()

	// Update charging health status
	err := repo.UpdateChargingHealthStatus(req.SubscriptionID, req.Status, req.Reason)
	if err != nil {
		h.logger.Error("Failed to update charging health status", zap.Int("subscription_id", req.SubscriptionID), zap.Error(err))
		ctx.Error("Failed to update charging health status", fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":          "success",
		"message":         "Charging health status updated successfully",
		"implementation":  "notifications-based",
		"subscription_id": req.SubscriptionID,
		"health_status":   req.Status,
		"reason":          req.Reason,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(response)
}

// MarkChargingFailureAsProcessedHandler marks a charging failure as processed
func (h *SubscriptionHandler) MarkChargingFailureAsProcessedHandler(ctx *fasthttp.RequestCtx) {
	// Parse request body
	var req struct {
		SubscriptionID int    `json:"subscription_id"`
		Status         string `json:"status"`
	}

	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	if req.SubscriptionID <= 0 {
		ctx.Error("Valid subscription_id is required", fasthttp.StatusBadRequest)
		return
	}

	if req.Status == "" {
		ctx.Error("Status is required", fasthttp.StatusBadRequest)
		return
	}

	// Get repository from service
	repo := h.service.GetRepository()

	// Mark charging failure as processed
	err := repo.MarkChargingFailureAsProcessed(req.SubscriptionID, req.Status)
	if err != nil {
		h.logger.Error("Failed to mark charging failure as processed", zap.Int("subscription_id", req.SubscriptionID), zap.Error(err))
		ctx.Error("Failed to mark charging failure as processed", fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":           "success",
		"message":          "Charging failure marked as processed successfully",
		"implementation":   "notifications-based",
		"subscription_id":  req.SubscriptionID,
		"processed_status": req.Status,
		"timestamp":        time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(response)
}

// GetBatchProgressHandler returns progress for a specific batch
func (h *SubscriptionHandler) GetBatchProgressHandler(ctx *fasthttp.RequestCtx) {
	args := ctx.QueryArgs()
	batchID := string(args.Peek("batch_id"))

	if batchID == "" {
		ctx.Error("batch_id is required", fasthttp.StatusBadRequest)
		return
	}

	// TODO: Implement batch progress tracking using proper service layer
	// For now, return a placeholder response
	response := map[string]interface{}{
		"batch_id": batchID,
		"status":   "not_implemented",
		"message":  "Batch progress tracking not yet implemented",
	}

	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(response)
}

// StopBatchHandler stops a running batch
func (h *SubscriptionHandler) StopBatchHandler(ctx *fasthttp.RequestCtx) {
	var req struct {
		BatchID string `json:"batch_id"`
		Reason  string `json:"reason"`
	}

	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request", fasthttp.StatusBadRequest)
		return
	}

	if req.BatchID == "" {
		ctx.Error("batch_id is required", fasthttp.StatusBadRequest)
		return
	}

	// TODO: Implement batch stopping using proper service layer
	// For now, return a placeholder response
	response := map[string]interface{}{
		"batch_id": req.BatchID,
		"status":   "not_implemented",
		"message":  "Batch stopping not yet implemented",
		"reason":   req.Reason,
	}

	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(response)
}

func (h *SubscriptionHandler) runBatchJob(jobID string, st *BatchJobStatus, req *domain.BatchOptinRequest) {
	h.logger.Info("Starting batch job", zap.String("jobId", jobID), zap.Int("totalRequests", len(req.MSISDNS)), zap.Int("requestedCount", req.Count))

	// Create context with timeout for the entire batch job
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// If no MSISDNs provided but count is specified, generate them using the optimized generator
	var msisdns []string
	var err error

	if len(req.MSISDNS) == 0 && req.Count > 0 {
		h.logger.Info("No MSISDNs provided, generating using optimized generator",
			zap.String("jobId", jobID),
			zap.Int("count", req.Count),
			zap.String("telco", req.Telco))

		// Use the optimized MSISDN generator to create the requested number of MSISDNs
		msisdns, err = h.msisdnGenerator.GenerateBatchMSISDNSOptimized(ctx, req.Telco, req.Count, h.config)
		if err != nil {
			h.logger.Error("Failed to generate MSISDNs using optimized generator",
				zap.String("jobId", jobID),
				zap.Error(err),
				zap.String("telco", req.Telco),
				zap.Int("count", req.Count))

			// Set error details and mark job as failed
			st.ErrorDetails = map[string]interface{}{
				"error": fmt.Sprintf("MSISDN generation failed: %v", err),
				"telco": req.Telco,
				"count": req.Count,
			}
			h.jobs.setCompleted(jobID, true)
			return
		}

		h.logger.Info("Successfully generated MSISDNs using optimized generator",
			zap.String("jobId", jobID),
			zap.Int("generated", len(msisdns)),
			zap.String("telco", req.Telco))

		// Update the request with generated MSISDNs for consistency
		req.MSISDNS = msisdns
	} else {
		// Use provided MSISDNs
		msisdns = req.MSISDNS
	}

	// Update total count based on actual MSISDNs available
	st.Total = len(msisdns)
	h.logger.Info("Batch job MSISDNs ready", zap.String("jobId", jobID), zap.Int("totalMSISDNs", len(msisdns)))

	// Concurrency parameters
	maxWorkers := calculateOptimalWorkers(len(msisdns))
	batchSize := calculateOptimalBatchSize(len(msisdns))
	h.logger.Info("Batch processing parameters", zap.String("jobId", jobID), zap.Int("maxWorkers", maxWorkers), zap.Int("batchSize", batchSize))

	var wg sync.WaitGroup
	optinRequestChan := make(chan *domain.OptinRequest, batchSize)

	// First error tracking
	var firstErrorDetails map[string]interface{}
	var firstErrorMutex sync.Mutex

	var successCount uint64
	var errorCount uint64

	// Create worker context with cancellation
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			h.logger.Debug("Worker started", zap.String("jobId", jobID), zap.Int("workerId", workerID))
			var errorBatch []error
			errorBatchSize := 10
			errorBatchTicker := time.NewTicker(5 * time.Second)
			defer errorBatchTicker.Stop()

			for {
				select {
				case request, ok := <-optinRequestChan:
					if !ok {
						// Channel closed, worker should exit
						return
					}

					// Process request with context timeout
					if err := h.service.ProcessOptin(request); err != nil {
						firstErrorMutex.Lock()
						if firstErrorDetails == nil {
							if mtErr, ok := err.(*domain.MTResponseError); ok {
								firstErrorDetails = mtErr.Details
							} else {
								firstErrorDetails = map[string]interface{}{"error": err.Error()}
							}
						}
						firstErrorMutex.Unlock()
						errorBatch = append(errorBatch, err)
						if len(errorBatch) >= errorBatchSize {
							h.logger.Error("Batch of subscription failures",
								zap.String("jobId", jobID),
								zap.Int("workerId", workerID),
								zap.Int("count", len(errorBatch)),
								zap.String("sampleError", errorBatch[0].Error()))
							errorBatch = errorBatch[:0]
						}
						atomic.AddUint64(&errorCount, 1)
						totalBatchRequestsFailed.Add(1)
					} else {
						atomic.AddUint64(&successCount, 1)
						totalBatchRequestsSucceeded.Add(1)
					}
					st.incProcessed()
					totalBatchRequestsProcessed.Add(1)

				case <-workerCtx.Done():
					// Context cancelled, worker should exit
					h.logger.Debug("Worker cancelled", zap.String("jobId", jobID), zap.Int("workerId", workerID))
					return
				}
			}

			if len(errorBatch) > 0 {
				h.logger.Error("Final batch of subscription failures",
					zap.String("jobId", jobID),
					zap.Int("workerId", workerID),
					zap.Int("count", len(errorBatch)),
					zap.String("sampleError", errorBatch[0].Error()))
			}
			h.logger.Debug("Worker finished", zap.String("jobId", jobID), zap.Int("workerId", workerID))
		}(i)
	}

	h.logger.Info("Starting to feed requests", zap.String("jobId", jobID), zap.Int("totalRequests", len(msisdns)))
	go func() {
		for i, msisdn := range msisdns {
			optinRequestChan <- &domain.OptinRequest{
				Telco:        req.Telco,
				Msisdn:       msisdn,
				EntryChannel: req.EntryChannel,
				ProductIds:   req.ProductIds,
			}
			if i%1000 == 0 {
				h.logger.Debug("Fed requests", zap.String("jobId", jobID), zap.Int("fed", i+1))
			}
		}
		close(optinRequestChan)
		h.logger.Info("Finished feeding requests", zap.String("jobId", jobID))
	}()

	h.logger.Info("Waiting for workers to complete", zap.String("jobId", jobID))
	wg.Wait()

	st.Successful = int64(successCount)
	st.Failed = int64(errorCount)
	if errorCount > 0 && firstErrorDetails != nil {
		st.ErrorDetails = firstErrorDetails
		h.jobs.setCompleted(jobID, true)
	} else {
		h.jobs.setCompleted(jobID, false)
	}
	totalBatchJobsCompleted.Add(1)
	h.logger.Info("Batch job completed", zap.String("jobId", jobID), zap.Int64("successful", st.Successful), zap.Int64("failed", st.Failed))
}

// calculateOptimalWorkers determines the optimal number of workers based on request volume
func calculateOptimalWorkers(requestCount int) int {
	// Base configuration for different volume tiers
	switch {
	case requestCount <= 100:
		return 5 // Small batches
	case requestCount <= 1000:
		return 20 // Medium batches
	case requestCount <= 5000:
		return 50 // Large batches
	case requestCount <= 10000:
		return 100 // Very large batches
	default:
		return 200 // Massive batches (10k+)
	}
}

// calculateOptimalBatchSize determines the optimal batch size for channel buffering
func calculateOptimalBatchSize(requestCount int) int {
	// Ensure we don't create channels that are too large
	maxBufferSize := 10000

	switch {
	case requestCount <= 100:
		return requestCount // Small batches - buffer all requests
	case requestCount <= 1000:
		return 500 // Medium batches
	case requestCount <= 5000:
		return 1000 // Large batches
	case requestCount <= 10000:
		return 2000 // Very large batches
	default:
		return maxBufferSize // Cap at reasonable size
	}
}

// getPerformanceRecommendations provides performance tuning recommendations
func (h *SubscriptionHandler) getPerformanceRecommendations(metrics map[string]interface{}) []string {
	var recommendations []string

	// Check throughput performance
	if throughput, ok := metrics["throughput_msisdns_per_second"].(float64); ok {
		if throughput < 500 {
			recommendations = append(recommendations, "Consider increasing batch size for better throughput")
		} else if throughput > 2000 {
			recommendations = append(recommendations, "Performance is excellent - consider reducing resources if needed")
		}
	}

	// Check Bloom Filter efficiency
	if hitRate, ok := metrics["bloom_hit_rate"].(float64); ok {
		if hitRate < 0.7 {
			recommendations = append(recommendations, "Bloom Filter hit rate is low - consider preloading more data")
		}
	}

	// Check efficiency
	if efficiency, ok := metrics["efficiency_percentage"].(float64); ok {
		if efficiency < 80 {
			recommendations = append(recommendations, "Generation efficiency is low - review validation logic")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Performance is optimal - no tuning needed")
	}

	return recommendations
}

// HealthCheckHandler provides system health and performance metrics
func (h *SubscriptionHandler) HealthCheckHandler(ctx *fasthttp.RequestCtx) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// Add MSISDN generator metrics if available
	if h.msisdnGenerator != nil {
		// Get detailed performance metrics
		metrics := h.msisdnGenerator.GetDetailedStats()
		health["msisdn_generator"] = metrics

		// Add performance tuning recommendations
		health["performance_tuning"] = map[string]interface{}{
			"recommendations":     h.getPerformanceRecommendations(metrics),
			"auto_tuning_enabled": true,
		}
	}

	// Add system metrics
	health["system"] = map[string]interface{}{
		"uptime": time.Since(h.startTime).String(),
		"jobs": map[string]interface{}{
			"total": len(h.jobs.jobs),
		},
		"configuration": map[string]interface{}{
			"redis_enabled":        h.config.Cache.Redis.Host != "" && h.config.Cache.Redis.Port != 0,
			"bloom_filter_enabled": true, // Always enabled if Redis is available
			"batch_size":           1000, // Hardcoded for now
			"max_concurrent":       50,   // Hardcoded for now
		},
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	if err := json.NewEncoder(ctx).Encode(health); err != nil {
		h.logger.Error("Failed to encode health response", zap.Error(err))
		ctx.Error("Failed to format health response", fasthttp.StatusInternalServerError)
		return
	}
}
