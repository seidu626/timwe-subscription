package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/worker"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// WorkerHandler handles batch processing operations
type WorkerHandler struct {
	processor *worker.ResubscriptionProcessor
	logger    *zap.Logger
}

// NewWorkerHandler creates a new worker handler
func NewWorkerHandler(processor *worker.ResubscriptionProcessor, logger *zap.Logger) *WorkerHandler {
	return &WorkerHandler{
		processor: processor,
		logger:    logger,
	}
}

// StartProcessingHandler starts the batch processing
// @Summary Start batch processing worker
// @Description Start the batch processing worker with optional configuration overrides
// @Tags Worker
// @Accept json
// @Produce json
// @Param config body worker.ProcessingConfig false "Optional processing configuration overrides"
// @Success 200 {object} map[string]interface{} "Batch processing started successfully"
// @Failure 409 {object} map[string]interface{} "Conflict - Processor already running"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/start [post]
func (h *WorkerHandler) StartProcessingHandler(ctx *fasthttp.RequestCtx) {
	if h.processor.IsRunning() {
		ctx.SetStatusCode(http.StatusConflict)
		ctx.WriteString(`{"status":"error","message":"Processor is already running"}`)
		return
	}

	// Parse configuration from request body
	var config worker.ProcessingConfig
	if len(ctx.PostBody()) > 0 {
		if err := json.Unmarshal(ctx.PostBody(), &config); err != nil {
			h.logger.Error("Failed to unmarshal processing config", zap.Error(err))
			ctx.SetStatusCode(http.StatusBadRequest)
			ctx.WriteString(`{"status":"error","message":"Invalid configuration data"}`)
			return
		}
	}

	// Start processing in background
	go func() {
		processCtx := context.Background()
		if err := h.processor.Start(processCtx); err != nil {
			h.logger.Error("Failed to start processor", zap.Error(err))
		}
	}()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Batch processing started successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"config":    config,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode start response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// StopProcessingHandler stops the batch processing
// @Summary Stop batch processing worker
// @Description Stop the currently running batch processing worker
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Batch processing stopped successfully"
// @Failure 409 {object} map[string]interface{} "Conflict - Processor not running"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/stop [post]
func (h *WorkerHandler) StopProcessingHandler(ctx *fasthttp.RequestCtx) {
	if !h.processor.IsRunning() {
		ctx.SetStatusCode(http.StatusConflict)
		ctx.WriteString(`{"status":"error","message":"Processor is not running"}`)
		return
	}

	h.processor.Stop()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Batch processing stopped successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode stop response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetProcessingStatusHandler returns the current processing status
// @Summary Get worker processing status
// @Description Get the current status of the batch processing worker
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Processing status retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/status [get]
func (h *WorkerHandler) GetProcessingStatusHandler(ctx *fasthttp.RequestCtx) {
	status := map[string]interface{}{
		"is_running": h.processor.IsRunning(),
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if h.processor.IsRunning() {
		stats := h.processor.GetStats()
		status["stats"] = stats
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Processing status retrieved successfully",
		"data":    status,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode status response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetProcessingStatsHandler returns detailed processing statistics
// @Summary Get worker processing statistics
// @Description Get detailed statistics about the batch processing operations
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Processing statistics retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/stats [get]
func (h *WorkerHandler) GetProcessingStatsHandler(ctx *fasthttp.RequestCtx) {
	stats := h.processor.GetStats()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Processing statistics retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"stats":     stats,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode stats response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetProcessingResultsHandler returns processing results (optionally filtered)
// @Summary Get worker processing results
// @Description Get processing results with optional filtering by status and pagination
// @Tags Worker
// @Accept json
// @Produce json
// @Param status query string false "Filter by result status (pending, processing, success, failed, skipped, completed)"
// @Param limit query int false "Maximum number of results to return (default: 100)"
// @Success 200 {object} map[string]interface{} "Processing results retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/results [get]
func (h *WorkerHandler) GetProcessingResultsHandler(ctx *fasthttp.RequestCtx) {
	status := string(ctx.QueryArgs().Peek("status"))
	limitStr := string(ctx.QueryArgs().Peek("limit"))

	limit := 100 // Default limit
	if limitStr != "" {
		if parsed, err := json.Marshal(limitStr); err == nil {
			limit = int(parsed[0])
		}
	}

	// Convert string status to ResubscriptionStatus
	var resubscriptionStatus worker.ResubscriptionStatus
	if status != "" {
		resubscriptionStatus = worker.ResubscriptionStatus(status)
	}

	results := h.processor.GetResults(resubscriptionStatus, limit)

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Processing results retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"results":   results,
		"count":     len(results),
		"filter": map[string]interface{}{
			"status": status,
			"limit":  limit,
		},
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode results response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetProcessingProgressHandler returns processing progress information
// @Summary Get worker processing progress
// @Description Get detailed progress information about the current batch processing operation
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Processing progress retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/progress [get]
func (h *WorkerHandler) GetProcessingProgressHandler(ctx *fasthttp.RequestCtx) {
	if !h.processor.IsRunning() {
		ctx.SetStatusCode(http.StatusConflict)
		ctx.WriteString(`{"status":"error","message":"Processor is not running"}`)
		return
	}

	// Get current stats
	stats := h.processor.GetStats()

	// Calculate progress percentage
	progress := 0.0
	if stats.TotalBatches > 0 {
		progress = float64(stats.CurrentBatch) / float64(stats.TotalBatches) * 100
	}

	// Calculate estimated time remaining
	estimatedRemaining := "unknown"
	if stats.EstimatedComplete.After(time.Now()) {
		estimatedRemaining = time.Until(stats.EstimatedComplete).String()
	}

	progressData := map[string]interface{}{
		"current_batch":       stats.CurrentBatch,
		"total_batches":       stats.TotalBatches,
		"progress_percentage": progress,
		"total_processed":     stats.TotalProcessed,
		"successful":          stats.Successful,
		"failed":              stats.Failed,
		"skipped":             stats.Skipped,
		"success_rate":        stats.SuccessRate,
		"average_time":        stats.AverageTime.String(),
		"start_time":          stats.StartTime.Format(time.RFC3339),
		"last_processed":      stats.LastProcessed.Format(time.RFC3339),
		"estimated_complete":  stats.EstimatedComplete.Format(time.RFC3339),
		"estimated_remaining": estimatedRemaining,
		"processing_status":   "running",
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Processing progress retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"progress":  progressData,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode progress response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetProcessingConfigHandler returns the current processing configuration
// @Summary Get worker processing configuration
// @Description Get the current configuration settings for the batch processing worker
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Processing configuration retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/config [get]
func (h *WorkerHandler) GetProcessingConfigHandler(ctx *fasthttp.RequestCtx) {
	config := h.processor.GetConfig()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Processing configuration retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"config":    config,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode config response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// UpdateProcessingConfigHandler updates the processing configuration
// @Summary Update worker processing configuration
// @Description Update the configuration settings for the batch processing worker
// @Tags Worker
// @Accept json
// @Produce json
// @Param config body worker.ProcessingConfig true "New processing configuration"
// @Success 200 {object} map[string]interface{} "Processing configuration updated successfully"
// @Failure 400 {object} map[string]interface{} "Bad request - invalid configuration data"
// @Failure 409 {object} map[string]interface{} "Conflict - Cannot update config while processor is running"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/config [post]
func (h *WorkerHandler) UpdateProcessingConfigHandler(ctx *fasthttp.RequestCtx) {
	if h.processor.IsRunning() {
		ctx.SetStatusCode(http.StatusConflict)
		ctx.WriteString(`{"status":"error","message":"Cannot update config while processor is running"}`)
		return
	}

	var config worker.ProcessingConfig
	if err := json.Unmarshal(ctx.PostBody(), &config); err != nil {
		h.logger.Error("Failed to unmarshal config", zap.Error(err))
		ctx.SetStatusCode(http.StatusBadRequest)
		ctx.WriteString(`{"status":"error","message":"Invalid configuration data"}`)
		return
	}

	// Update the processor configuration
	if err := h.processor.UpdateConfig(&config); err != nil {
		h.logger.Error("Failed to update processor config", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to update configuration"}`)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Processing configuration updated successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"config":    config,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode config update response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// PauseProcessingHandler pauses the batch processing
// @Summary Pause batch processing worker
// @Description Pause the currently running batch processing worker
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Batch processing paused successfully"
// @Failure 409 {object} map[string]interface{} "Conflict - Processor not running"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/pause [post]
func (h *WorkerHandler) PauseProcessingHandler(ctx *fasthttp.RequestCtx) {
	if !h.processor.IsRunning() {
		ctx.SetStatusCode(http.StatusConflict)
		ctx.WriteString(`{"status":"error","message":"Processor is not running"}`)
		return
	}

	h.processor.Pause()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Batch processing paused successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode pause response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// ResumeProcessingHandler resumes the batch processing
// @Summary Resume batch processing worker
// @Description Resume the paused batch processing worker
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Batch processing resumed successfully"
// @Failure 409 {object} map[string]interface{} "Conflict - Processor already running"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/resume [post]
func (h *WorkerHandler) ResumeProcessingHandler(ctx *fasthttp.RequestCtx) {
	if h.processor.IsRunning() {
		ctx.SetStatusCode(http.StatusConflict)
		ctx.WriteString(`{"status":"error","message":"Processor is already running"}`)
		return
	}

	processCtx := context.Background()
	if err := h.processor.Resume(processCtx); err != nil {
		h.logger.Error("Failed to resume processor", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to resume processing"}`)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Batch processing resumed successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode resume response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetDetailedStatusHandler returns comprehensive processing status information
// @Summary Get detailed worker status
// @Description Get comprehensive status information including tracker details and configuration
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Detailed status retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/detailed-status [get]
func (h *WorkerHandler) GetDetailedStatusHandler(ctx *fasthttp.RequestCtx) {
	detailedStatus := h.processor.GetDetailedStatus()

	response := map[string]interface{}{
		"status":         "success",
		"message":        "Detailed status retrieved successfully",
		"timestamp":      time.Now().Format(time.RFC3339),
		"detailedStatus": detailedStatus,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode detailed status response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetProcessingSummaryHandler returns a summary of the current processing run
// @Summary Get processing summary
// @Description Get a summary of the current processing run with rates and timing
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Processing summary retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/summary [get]
func (h *WorkerHandler) GetProcessingSummaryHandler(ctx *fasthttp.RequestCtx) {
	summary := h.processor.GetProcessingSummary()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Processing summary retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"summary":   summary,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode summary response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// ExportResultsHandler exports processing results in various formats
// @Summary Export processing results
// @Description Export processing results in JSON or CSV format with optional filtering
// @Tags Worker
// @Accept json
// @Produce json
// @Param format query string true "Export format (json, csv)"
// @Param status query string false "Filter by result status"
// @Success 200 {object} map[string]interface{} "Results exported successfully"
// @Failure 400 {object} map[string]interface{} "Bad request - invalid format"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/export [get]
func (h *WorkerHandler) ExportResultsHandler(ctx *fasthttp.RequestCtx) {
	format := string(ctx.QueryArgs().Peek("format"))
	status := string(ctx.QueryArgs().Peek("status"))

	if format == "" {
		format = "json" // Default to JSON
	}

	// Convert string status to ResubscriptionStatus
	var resubscriptionStatus worker.ResubscriptionStatus
	if status != "" {
		resubscriptionStatus = worker.ResubscriptionStatus(status)
	}

	// Export results
	exportedData, err := h.processor.ExportResults(format, resubscriptionStatus)
	if err != nil {
		h.logger.Error("Failed to export results", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to export results"}`)
		return
	}

	// Set appropriate content type and headers
	switch format {
	case "json":
		ctx.SetContentType("application/json")
	case "csv":
		ctx.SetContentType("text/csv")
		ctx.Response.Header.Set("Content-Disposition", "attachment; filename=processing_results.csv")
	default:
		ctx.SetContentType("application/octet-stream")
	}

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write(exportedData)
}

// GetResultsByTimeRangeHandler returns results within a specific time range
// @Summary Get results by time range
// @Description Get processing results within a specific time range with optional status filtering
// @Tags Worker
// @Accept json
// @Produce json
// @Param start query string true "Start time (RFC3339 format)"
// @Param end query string true "End time (RFC3339 format)"
// @Param status query string false "Filter by result status"
// @Success 200 {object} map[string]interface{} "Time range results retrieved successfully"
// @Failure 400 {object} map[string]interface{} "Bad request - invalid time format"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/results/time-range [get]
func (h *WorkerHandler) GetResultsByTimeRangeHandler(ctx *fasthttp.RequestCtx) {
	startStr := string(ctx.QueryArgs().Peek("start"))
	endStr := string(ctx.QueryArgs().Peek("end"))
	status := string(ctx.QueryArgs().Peek("status"))

	if startStr == "" || endStr == "" {
		ctx.SetStatusCode(http.StatusBadRequest)
		ctx.WriteString(`{"status":"error","message":"Start and end time parameters are required"}`)
		return
	}

	// Parse time parameters
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		ctx.SetStatusCode(http.StatusBadRequest)
		ctx.WriteString(`{"status":"error","message":"Invalid start time format"}`)
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		ctx.SetStatusCode(http.StatusBadRequest)
		ctx.WriteString(`{"status":"error","message":"Invalid end time format"}`)
		return
	}

	// Convert string status to ResubscriptionStatus
	var resubscriptionStatus worker.ResubscriptionStatus
	if status != "" {
		resubscriptionStatus = worker.ResubscriptionStatus(status)
	}

	// Get results by time range
	results := h.processor.GetResultsByTimeRange(start, end, resubscriptionStatus)

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Time range results retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"results":   results,
		"count":     len(results),
		"filter": map[string]interface{}{
			"start_time": start.Format(time.RFC3339),
			"end_time":   end.Format(time.RFC3339),
			"status":     status,
		},
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode time range results response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// ClearResultsHandler clears all processing results
// @Summary Clear processing results
// @Description Clear all processing results and reset statistics
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Results cleared successfully"
// @Failure 409 {object} map[string]interface{} "Conflict - Cannot clear results while processor is running"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/results/clear [post]
func (h *WorkerHandler) ClearResultsHandler(ctx *fasthttp.RequestCtx) {
	if h.processor.IsRunning() {
		ctx.SetStatusCode(http.StatusConflict)
		ctx.WriteString(`{"status":"error","message":"Cannot clear results while processor is running"}`)
		return
	}

	h.processor.ClearResults()
	h.processor.ResetStats()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Results and statistics cleared successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode clear results response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GracefulShutdownHandler performs a graceful shutdown of the processor
// @Summary Graceful shutdown
// @Description Perform a graceful shutdown with cleanup and final checkpoint
// @Tags Worker
// @Accept json
// @Produce json
// @Param timeout query int false "Shutdown timeout in seconds (default: 300)"
// @Success 200 {object} map[string]interface{} "Graceful shutdown completed successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/worker/shutdown [post]
func (h *WorkerHandler) GracefulShutdownHandler(ctx *fasthttp.RequestCtx) {
	timeoutStr := string(ctx.QueryArgs().Peek("timeout"))

	timeout := 5 * time.Minute // Default timeout
	if timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr + "s"); err == nil {
			timeout = parsed
		}
	}

	processCtx := context.Background()
	if err := h.processor.GracefulShutdown(processCtx, timeout); err != nil {
		h.logger.Error("Failed to perform graceful shutdown", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to perform graceful shutdown"}`)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Graceful shutdown completed successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"timeout":   timeout.String(),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode shutdown response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}
