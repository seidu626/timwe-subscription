package handler

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	totalBatchJobsCreated       atomic.Int64
	totalBatchJobsCompleted     atomic.Int64
	totalBatchRequestsProcessed atomic.Int64
	totalBatchRequestsSucceeded atomic.Int64
	totalBatchRequestsFailed    atomic.Int64
	startTime                   = time.Now()
)

// HealthCheck godoc
// @Summary Enhanced health check endpoint
// @Description Check if the service is healthy and running with detailed metrics
// @Tags System
// @Accept json
// @Produce application/json
// @Success 200 {object} map[string]interface{} "Health status with metrics"
// @Router /health [get]
func HealthCheck(ctx *fasthttp.RequestCtx) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
		"uptime":    time.Since(startTime).String(),
		"system": map[string]interface{}{
			"batch_jobs": map[string]interface{}{
				"total_created":   totalBatchJobsCreated.Load(),
				"total_completed": totalBatchJobsCompleted.Load(),
			},
			"batch_requests": map[string]interface{}{
				"total_processed": totalBatchRequestsProcessed.Load(),
				"total_succeeded": totalBatchRequestsSucceeded.Load(),
				"total_failed":    totalBatchRequestsFailed.Load(),
			},
		},
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	if err := json.NewEncoder(ctx).Encode(health); err != nil {
		ctx.Error("Failed to encode health response", fasthttp.StatusInternalServerError)
		return
	}
}

// MetricsHandler godoc
// @Summary Metrics endpoint
// @Description Get Prometheus metrics for the service
// @Tags System
// @Accept json
// @Produce text/plain
// @Success 200 {string} string "Prometheus metrics"
// @Router /metrics [get]
func MetricsHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("text/plain")
	metrics := fmt.Sprintf(`# HELP batch_jobs_created Total number of batch jobs created
# TYPE batch_jobs_created counter
batch_jobs_created %d
# HELP batch_jobs_completed Total number of batch jobs completed
# TYPE batch_jobs_completed counter
batch_jobs_completed %d
# HELP batch_requests_processed Total number of subscription requests processed by batch jobs
# TYPE batch_requests_processed counter
batch_requests_processed %d
# HELP batch_requests_succeeded Total number of successful subscription requests in batch jobs
# TYPE batch_requests_succeeded counter
batch_requests_succeeded %d
# HELP batch_requests_failed Total number of failed subscription requests in batch jobs
# TYPE batch_requests_failed counter
batch_requests_failed %d
`,
		totalBatchJobsCreated.Load(),
		totalBatchJobsCompleted.Load(),
		totalBatchRequestsProcessed.Load(),
		totalBatchRequestsSucceeded.Load(),
		totalBatchRequestsFailed.Load(),
	)
	ctx.SetBodyString(metrics)
}
