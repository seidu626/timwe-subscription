package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/seidu626/subscription-manager/subscription-external/internal/worker"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// @Summary Start renewal worker
// @Description Starts the automated renewal worker that processes subscription renewals
// @Tags Renewal Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Worker started successfully"
// @Failure 409 {string} string "Worker already running"
// @Failure 503 {string} string "Worker not available"
// @Failure 500 {string} string "Failed to start worker"
// @Router /api/v1/renewal/worker/start [post]

// ManualRenewalRequest represents a manual renewal request
// @Description Request structure for manual renewal
type ManualRenewalRequest struct {
	MSISDN    string `json:"msisdn" example:"1234567890" binding:"required"`   // Mobile number
	ProductID string `json:"product_id" example:"PROD_001" binding:"required"` // Product identifier
	Channel   string `json:"channel" example:"API"`                            // Entry channel
}

// RenewalWorkerStatus represents the status of the renewal worker
// @Description Status and metrics of the renewal worker
type RenewalWorkerStatus struct {
	Running bool                   `json:"running" example:"true"` // Whether the worker is running
	Metrics *domain.RenewalMetrics `json:"metrics"`                // Worker metrics
}

// RenewalHealthResponse represents the health status of the renewal system
// @Description Health status of the renewal system
type RenewalHealthResponse struct {
	WorkerRunning bool                   `json:"worker_running" example:"true"`            // Worker status
	WorkerHealth  map[string]interface{} `json:"worker_health"`                            // Worker health details
	Timestamp     time.Time              `json:"timestamp" example:"2024-01-01T00:00:00Z"` // Health check timestamp
}

// ChurnEvaluationResponse represents the result of a churn evaluation
// @Description Result of a churn evaluation operation
type ChurnEvaluationResponse struct {
	Status    string `json:"status" example:"success"`                     // Operation status
	Message   string `json:"message" example:"Churn evaluation completed"` // Result message
	Processed int    `json:"processed" example:"150"`                      // Number of subscriptions processed
	Churned   int    `json:"churned" example:"25"`                         // Number of subscriptions churned
	Renewed   int    `json:"renewed" example:"125"`                        // Number of subscriptions renewed
}

// RenewalHandler handles renewal-related HTTP requests
type RenewalHandler struct {
	renewalService *service.RenewalService
	renewalWorker  *worker.RenewalWorker
	logger         *zap.Logger
}

// NewRenewalHandler creates a new renewal handler
func NewRenewalHandler(renewalService *service.RenewalService, renewalWorker *worker.RenewalWorker, logger *zap.Logger) *RenewalHandler {
	return &RenewalHandler{
		renewalService: renewalService,
		renewalWorker:  renewalWorker,
		logger:         logger,
	}
}

// StartRenewalWorker starts the renewal worker
// @Summary Start renewal worker
// @Description Starts the automated renewal worker that processes subscription renewals
// @Tags Renewal Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Worker started successfully"
// @Failure 409 {string} string "Worker already running"
// @Failure 503 {string} string "Worker not available"
// @Failure 500 {string} string "Failed to start worker"
// @Router /api/v1/renewal/worker/start [post]
func (h *RenewalHandler) StartRenewalWorker(ctx *fasthttp.RequestCtx) {
	h.logger.Info("Starting renewal worker via API request")

	if h.renewalWorker == nil {
		h.logger.Error("Renewal worker not initialized")
		ctx.Error("Renewal worker not available", fasthttp.StatusServiceUnavailable)
		return
	}

	if h.renewalWorker.IsRunning() {
		ctx.Error("Renewal worker is already running", fasthttp.StatusConflict)
		return
	}

	go func() {
		if err := h.renewalWorker.Start(context.Background()); err != nil {
			h.logger.Error("Failed to start renewal worker", zap.Error(err))
		}
	}()

	// Wait a moment for the worker to start
	time.Sleep(100 * time.Millisecond)

	if h.renewalWorker.IsRunning() {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/json")
		response := map[string]interface{}{
			"status":  "success",
			"message": "Renewal worker started successfully",
			"running": true,
		}
		json.NewEncoder(ctx).Encode(response)
	} else {
		ctx.Error("Failed to start renewal worker", fasthttp.StatusInternalServerError)
	}
}

// StopRenewalWorker stops the renewal worker
// @Summary Stop renewal worker
// @Description Stops the automated renewal worker
// @Tags Renewal Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Worker stopped successfully"
// @Failure 409 {string} string "Worker not running"
// @Failure 503 {string} string "Worker not available"
// @Failure 500 {string} string "Failed to stop worker"
// @Router /api/v1/renewal/worker/stop [post]
func (h *RenewalHandler) StopRenewalWorker(ctx *fasthttp.RequestCtx) {
	h.logger.Info("Stopping renewal worker via API request")

	if h.renewalWorker == nil {
		h.logger.Error("Renewal worker not initialized")
		ctx.Error("Renewal worker not available", fasthttp.StatusServiceUnavailable)
		return
	}

	if !h.renewalWorker.IsRunning() {
		ctx.Error("Renewal worker is not running", fasthttp.StatusConflict)
		return
	}

	h.renewalWorker.Stop()

	// Wait a moment for the worker to stop
	time.Sleep(100 * time.Millisecond)

	if !h.renewalWorker.IsRunning() {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/json")
		response := map[string]interface{}{
			"status":  "success",
			"message": "Renewal worker stopped successfully",
			"running": false,
		}
		json.NewEncoder(ctx).Encode(response)
	} else {
		ctx.Error("Failed to stop renewal worker", fasthttp.StatusInternalServerError)
	}
}

// GetRenewalWorkerStatus returns the current status of the renewal worker
// @Summary Get renewal worker status
// @Description Returns the current status and metrics of the renewal worker
// @Tags Renewal Worker
// @Produce json
// @Success 200 {object} RenewalWorkerStatus "Worker status and metrics"
// @Failure 503 {string} string "Worker not available"
// @Router /api/v1/renewal/worker/status [get]
func (h *RenewalHandler) GetRenewalWorkerStatus(ctx *fasthttp.RequestCtx) {
	if h.renewalWorker == nil {
		ctx.Error("Renewal worker not available", fasthttp.StatusServiceUnavailable)
		return
	}

	status := map[string]interface{}{
		"running": h.renewalWorker.IsRunning(),
		"metrics": h.renewalWorker.GetMetrics(),
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(status)
}

// GetRenewalStatistics returns renewal statistics
// @Summary Get renewal statistics
// @Description Returns renewal statistics for the specified number of days
// @Tags Renewal Monitoring
// @Produce json
// @Param days query int false "Number of days to look back (default: 30)" default(30)
// @Success 200 {object} domain.RenewalMetrics "Renewal statistics"
// @Failure 500 {string} string "Failed to get statistics"
// @Router /api/v1/renewal/statistics [get]
func (h *RenewalHandler) GetRenewalStatistics(ctx *fasthttp.RequestCtx) {
	// Parse query parameters
	daysStr := string(ctx.QueryArgs().Peek("days"))
	days := 30 // default to 30 days
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	stats, err := h.renewalService.GetRenewalStatistics(context.Background(), days*24) // Convert days to hours
	if err != nil {
		h.logger.Error("Failed to get renewal statistics", zap.Error(err))
		ctx.Error("Failed to get renewal statistics", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(stats)
}

// GetChurnCandidates returns subscriptions that are candidates for churn
// @Summary Get churn candidates
// @Description Returns subscriptions that are candidates for churn based on policy
// @Tags Renewal Monitoring
// @Produce json
// @Param limit query int false "Maximum number of candidates to return (default: 100)" default(100)
// @Success 200 {array} domain.SubscriptionWithRenewalInfo "Churn candidates"
// @Failure 500 {string} string "Failed to get churn candidates"
// @Router /api/v1/renewal/churn-candidates [get]
func (h *RenewalHandler) GetChurnCandidates(ctx *fasthttp.RequestCtx) {
	// Parse query parameters
	limitStr := string(ctx.QueryArgs().Peek("limit"))
	limit := 100 // default to 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	candidates, err := h.renewalService.GetChurnCandidates(context.Background(), 30, 3, limit) // 30 days, 3 attempts, limit
	if err != nil {
		h.logger.Error("Failed to get churn candidates", zap.Error(err))
		ctx.Error("Failed to get churn candidates", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(candidates)
}

// ProcessPriorityRetryQueue processes the priority retry queue
// @Summary Process priority retry queue
// @Description Processes the priority retry queue for failed opt-ins
// @Tags Renewal Operations
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Queue processed successfully"
// @Failure 503 {string} string "Worker not available"
// @Failure 500 {string} string "Failed to process queue"
// @Router /api/v1/renewal/priority-retry/process [post]
func (h *RenewalHandler) ProcessPriorityRetryQueue(ctx *fasthttp.RequestCtx) {
	h.logger.Info("Processing priority retry queue via API request")

	if h.renewalWorker == nil {
		h.logger.Error("Renewal worker not available")
		ctx.Error("Renewal worker not available", fasthttp.StatusServiceUnavailable)
		return
	}

	// Process the queue
	err := h.renewalWorker.ProcessPriorityRetryQueue(context.Background())
	if err != nil {
		h.logger.Error("Failed to process priority retry queue", zap.Error(err))
		ctx.Error("Failed to process priority retry queue", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	response := map[string]interface{}{
		"status":  "success",
		"message": "Priority retry queue processed successfully",
	}
	json.NewEncoder(ctx).Encode(response)
}

// GetRenewalCycles returns renewal cycles with optional filtering
// @Summary Get renewal cycles
// @Description Returns renewal cycles with optional filtering by status
// @Tags Renewal Monitoring
// @Produce json
// @Param status query string false "Filter by status"
// @Param limit query int false "Maximum number of cycles to return (default: 100)" default(100)
// @Success 200 {array} domain.RenewalCycle "Renewal cycles"
// @Failure 501 {string} string "Method not implemented"
// @Router /api/v1/renewal/cycles [get]
func (h *RenewalHandler) GetRenewalCycles(ctx *fasthttp.RequestCtx) {
	// This method is not implemented in the service yet
	ctx.Error("Method not implemented", fasthttp.StatusNotImplemented)
}

// GetPriorityRetryQueue returns items in the priority retry queue
// @Summary Get priority retry queue
// @Description Returns items currently in the priority retry queue
// @Tags Renewal Monitoring
// @Produce json
// @Success 200 {array} domain.PriorityRetryQueue "Priority retry queue items"
// @Failure 501 {string} string "Method not implemented"
// @Router /api/v1/renewal/priority-retry [get]
func (h *RenewalHandler) GetPriorityRetryQueue(ctx *fasthttp.RequestCtx) {
	// This method is not implemented in the service yet
	ctx.Error("Method not implemented", fasthttp.StatusNotImplemented)
}

// ManualRenewal triggers a manual renewal for a specific subscription
// @Summary Manual renewal
// @Description Triggers a manual renewal for a specific subscription
// @Tags Renewal Operations
// @Accept json
// @Produce json
// @Param request body ManualRenewalRequest true "Renewal request"
// @Success 200 {object} domain.RenewalResponse "Renewal response"
// @Failure 400 {string} string "Invalid request body or missing fields"
// @Failure 404 {string} string "Product not found"
// @Failure 500 {string} string "Failed to process renewal"
// @Router /api/v1/renewal/manual [post]
func (h *RenewalHandler) ManualRenewal(ctx *fasthttp.RequestCtx) {
	var request struct {
		MSISDN    string `json:"msisdn"`
		ProductID string `json:"product_id"`
		Channel   string `json:"channel"`
	}

	if err := json.Unmarshal(ctx.PostBody(), &request); err != nil {
		h.logger.Error("Failed to parse manual renewal request", zap.Error(err))
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	// Validate request
	if request.MSISDN == "" || request.ProductID == "" {
		ctx.Error("Missing required fields: msisdn, product_id", fasthttp.StatusBadRequest)
		return
	}

	if request.Channel == "" {
		request.Channel = "API"
	}

	// Get product details first
	product, err := h.renewalService.GetProduct(context.Background(), request.ProductID)
	if err != nil {
		h.logger.Error("Failed to get product", zap.Error(err))
		ctx.Error("Failed to get product", fasthttp.StatusInternalServerError)
		return
	}

	if product == nil {
		ctx.Error("Product not found", fasthttp.StatusNotFound)
		return
	}

	// Process the renewal
	response, err := h.renewalService.SendRenewalRequest(context.Background(), request.MSISDN, product, request.Channel)
	if err != nil {
		h.logger.Error("Failed to process manual renewal", zap.Error(err))
		ctx.Error("Failed to process renewal", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(response)
}

// GetRenewalHealth returns the health status of the renewal system
// @Summary Get renewal system health
// @Description Returns the health status of the renewal system
// @Tags Renewal Monitoring
// @Produce json
// @Success 200 {object} RenewalHealthResponse "System health status"
// @Failure 503 {string} string "Worker not available"
// @Router /api/v1/renewal/health [get]
func (h *RenewalHandler) GetRenewalHealth(ctx *fasthttp.RequestCtx) {
	if h.renewalWorker == nil {
		ctx.Error("Renewal worker not available", fasthttp.StatusServiceUnavailable)
		return
	}

	health := map[string]interface{}{
		"worker_running": h.renewalWorker.IsRunning(),
		"worker_health":  h.renewalWorker.HealthCheck(),
		"timestamp":      time.Now().UTC(),
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(health)
}

// ForceChurnEvaluation forces a churn evaluation for all subscriptions
// @Summary Force churn evaluation
// @Description Forces a churn evaluation for all subscriptions based on policy
// @Tags Renewal Operations
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of subscriptions to evaluate (default: 1000)" default(1000)
// @Success 200 {object} ChurnEvaluationResponse "Churn evaluation completed"
// @Failure 500 {string} string "Failed to evaluate churn candidates"
// @Router /api/v1/renewal/force-churn-evaluation [post]
func (h *RenewalHandler) ForceChurnEvaluation(ctx *fasthttp.RequestCtx) {
	h.logger.Info("Forcing churn evaluation via API request")

	// Parse query parameters
	limitStr := string(ctx.QueryArgs().Peek("limit"))
	limit := 1000 // default to 1000
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Get churn candidates
	candidates, err := h.renewalService.GetChurnCandidates(context.Background(), 30, 3, limit)
	if err != nil {
		h.logger.Error("Failed to get churn candidates", zap.Error(err))
		ctx.Error("Failed to get churn candidates", fasthttp.StatusInternalServerError)
		return
	}

	// Process each candidate
	processed := 0
	churned := 0
	renewed := 0

	for _, candidate := range candidates {
		action := h.renewalService.EvaluateChurnPolicy(context.Background(), candidate.UserIdentifier, candidate.ProductId)

		processed++

		switch action {
		case domain.ActionChurn:
			if err := h.renewalService.ChurnSubscription(context.Background(), candidate.UserIdentifier, candidate.ProductId, "API forced evaluation"); err != nil {
				h.logger.Error("Failed to churn subscription",
					zap.String("user_identifier", candidate.UserIdentifier),
					zap.Error(err))
			} else {
				churned++
			}
		case domain.ActionAttemptRenewal:
			// Get product details
			product, err := h.renewalService.GetProduct(context.Background(), candidate.ProductId)
			if err != nil {
				h.logger.Error("Failed to get product for renewal",
					zap.String("product_id", candidate.ProductId),
					zap.Error(err))
				continue
			}

			if product == nil {
				h.logger.Error("Product not found for renewal",
					zap.String("product_id", candidate.ProductId))
				continue
			}

			// Send renewal request
			if _, err := h.renewalService.SendRenewalRequest(context.Background(), candidate.UserIdentifier, product, "API"); err != nil {
				h.logger.Error("Failed to send renewal request",
					zap.String("user_identifier", candidate.UserIdentifier),
					zap.Error(err))
			} else {
				renewed++
			}
		}
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	response := map[string]interface{}{
		"status":    "success",
		"message":   "Churn evaluation completed",
		"processed": processed,
		"churned":   churned,
		"renewed":   renewed,
	}
	json.NewEncoder(ctx).Encode(response)
}
