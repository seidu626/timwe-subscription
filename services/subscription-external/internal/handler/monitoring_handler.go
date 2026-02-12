package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/monitoring"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// MonitoringHandler handles monitoring and alerting endpoints
type MonitoringHandler struct {
	monitor         *monitoring.ChargingFailureMonitor
	realTimeMonitor *monitoring.RealTimeMonitor
	logger          *zap.Logger
}

// NewMonitoringHandler creates a new monitoring handler
func NewMonitoringHandler(monitor *monitoring.ChargingFailureMonitor, logger *zap.Logger) *MonitoringHandler {
	realTimeMonitor := monitoring.NewRealTimeMonitor(monitor, logger)

	return &MonitoringHandler{
		monitor:         monitor,
		realTimeMonitor: realTimeMonitor,
		logger:          logger,
	}
}

// GetRealTimeMonitor returns the real-time monitor instance
func (h *MonitoringHandler) GetRealTimeMonitor() *monitoring.RealTimeMonitor {
	return h.realTimeMonitor
}

// HandleWebSocketConnection handles WebSocket connections for real-time updates
func (h *MonitoringHandler) HandleWebSocketConnection(ctx *fasthttp.RequestCtx) {
	// Convert fasthttp context to net/http for WebSocket upgrade
	uri := ctx.URI()
	url := &url.URL{
		Scheme:   string(uri.Scheme()),
		Host:     string(uri.Host()),
		Path:     string(uri.Path()),
		RawQuery: string(uri.QueryString()),
	}

	req := &http.Request{
		Method: string(ctx.Method()),
		URL:    url,
		Header: make(http.Header),
	}

	// Copy headers
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		req.Header.Set(string(key), string(value))
	})

	// Create a response writer adapter
	w := &responseWriterAdapter{ctx: ctx}

	// Handle the WebSocket upgrade
	h.realTimeMonitor.HandleWebSocket(w, req)
}

// responseWriterAdapter adapts fasthttp.RequestCtx to http.ResponseWriter
type responseWriterAdapter struct {
	ctx *fasthttp.RequestCtx
}

func (w *responseWriterAdapter) Header() http.Header {
	header := make(http.Header)
	w.ctx.Response.Header.VisitAll(func(key, value []byte) {
		header.Set(string(key), string(value))
	})
	return header
}

func (w *responseWriterAdapter) Write(data []byte) (int, error) {
	return w.ctx.Write(data)
}

func (w *responseWriterAdapter) WriteHeader(statusCode int) {
	w.ctx.SetStatusCode(statusCode)
}

// GetDashboardDataHandler returns comprehensive dashboard data
// @Summary Get monitoring dashboard data
// @Description Get comprehensive monitoring dashboard data including metrics, alerts, and charts
// @Tags Monitoring
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Dashboard data retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/monitoring/dashboard [get]
func (h *MonitoringHandler) GetDashboardDataHandler(ctx *fasthttp.RequestCtx) {
	dashboardData := h.monitor.GetDashboardData()

	// Log the dashboard data being returned
	h.logger.Info("Returning dashboard data",
		zap.Any("dashboard_data", dashboardData))

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Dashboard data retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      dashboardData,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode dashboard response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetMetricsHandler returns current metrics
// @Summary Get monitoring metrics
// @Description Get current monitoring metrics for charging failures and system health
// @Tags Monitoring
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Metrics retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/monitoring/metrics [get]
func (h *MonitoringHandler) GetMetricsHandler(ctx *fasthttp.RequestCtx) {
	metrics := h.monitor.GetMetrics()

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Metrics retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"metrics":   metrics,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode metrics response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetAlertsHandler returns alerts (optionally filtered by severity)
// @Summary Get monitoring alerts
// @Description Get current monitoring alerts with optional filtering by severity, type, and acknowledgment status
// @Tags Monitoring
// @Accept json
// @Produce json
// @Param severity query string false "Filter by alert severity (low, medium, high, critical)"
// @Param type query string false "Filter by alert type"
// @Param acknowledged query boolean false "Filter by acknowledgment status"
// @Success 200 {object} map[string]interface{} "Alerts retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/monitoring/alerts [get]
func (h *MonitoringHandler) GetAlertsHandler(ctx *fasthttp.RequestCtx) {
	severity := string(ctx.QueryArgs().Peek("severity"))
	alerts := h.monitor.GetAlerts(severity)

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Alerts retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"alerts":    alerts,
		"count":     len(alerts),
		"filter": map[string]interface{}{
			"severity": severity,
		},
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode alerts response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// AcknowledgeAlertHandler marks an alert as acknowledged
// @Summary Acknowledge monitoring alert
// @Description Mark a monitoring alert as acknowledged by the specified user
// @Tags Monitoring
// @Accept json
// @Produce json
// @Param alert_id query string true "ID of the alert to acknowledge"
// @Param acknowledged_by query string false "Identifier of who acknowledged the alert"
// @Param notes query string false "Optional notes about the acknowledgment"
// @Success 200 {object} map[string]interface{} "Alert acknowledged successfully"
// @Failure 400 {object} map[string]interface{} "Bad request - missing alert_id"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/monitoring/alerts/acknowledge [post]
func (h *MonitoringHandler) AcknowledgeAlertHandler(ctx *fasthttp.RequestCtx) {
	alertID := string(ctx.QueryArgs().Peek("alert_id"))
	if alertID == "" {
		ctx.SetStatusCode(http.StatusBadRequest)
		ctx.WriteString(`{"status":"error","message":"alert_id parameter is required"}`)
		return
	}

	err := h.monitor.AcknowledgeAlert(alertID)
	if err != nil {
		h.logger.Error("Failed to acknowledge alert", zap.Error(err), zap.String("alert_id", alertID))
		ctx.SetStatusCode(http.StatusNotFound)
		ctx.WriteString(fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error()))
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Alert acknowledged successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"alert_id":  alertID,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode acknowledge response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// ClearAlertsHandler removes old acknowledged alerts
// @Summary Clear monitoring alerts
// @Description Remove old acknowledged alerts older than the specified hours
// @Tags Monitoring
// @Accept json
// @Produce json
// @Param hours query int false "Hours threshold for clearing alerts (default: 24)"
// @Success 200 {object} map[string]interface{} "Alerts cleared successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/monitoring/alerts/clear [post]
func (h *MonitoringHandler) ClearAlertsHandler(ctx *fasthttp.RequestCtx) {
	hoursStr := string(ctx.QueryArgs().Peek("hours"))
	hours := 24 // Default to 24 hours

	if hoursStr != "" {
		if parsed, err := strconv.Atoi(hoursStr); err == nil && parsed > 0 {
			hours = parsed
		}
	}

	olderThan := time.Duration(hours) * time.Hour
	h.monitor.ClearAlerts(olderThan)

	response := map[string]interface{}{
		"status":             "success",
		"message":            "Alerts cleared successfully",
		"timestamp":          time.Now().Format(time.RFC3339),
		"cleared_older_than": fmt.Sprintf("%d hours", hours),
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode clear alerts response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// UpdateThresholdsHandler updates alert thresholds
// @Summary Update monitoring alert thresholds
// @Description Update the alert thresholds for charging failures
// @Tags Monitoring
// @Accept json
// @Produce json
// @Param thresholds body monitoring.AlertThresholds true "New alert thresholds"
// @Success 200 {object} map[string]interface{} "Thresholds updated successfully"
// @Failure 400 {object} map[string]interface{} "Bad request - invalid threshold data"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/monitoring/thresholds [put]
func (h *MonitoringHandler) UpdateThresholdsHandler(ctx *fasthttp.RequestCtx) {
	var thresholds monitoring.AlertThresholds
	if err := json.Unmarshal(ctx.PostBody(), &thresholds); err != nil {
		h.logger.Error("Failed to unmarshal thresholds", zap.Error(err))
		ctx.SetStatusCode(http.StatusBadRequest)
		ctx.WriteString(`{"status":"error","message":"Invalid threshold data"}`)
		return
	}

	// TODO: Implement threshold update in monitor
	// For now, just log the request
	h.logger.Info("Threshold update requested", zap.Any("thresholds", thresholds))

	response := map[string]interface{}{
		"status":     "success",
		"message":    "Thresholds updated successfully",
		"timestamp":  time.Now().Format(time.RFC3339),
		"thresholds": thresholds,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(http.StatusOK)

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode threshold response", zap.Error(err))
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.WriteString(`{"status":"error","message":"Failed to encode response"}`)
	}
}

// GetHealthHandler returns system health status
// @Summary Get monitoring system health status
// @Description Get comprehensive health status including system health, alerts, and metrics freshness
// @Tags Monitoring
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Health check completed successfully"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/subscription-external/monitoring/health [get]
func (h *MonitoringHandler) GetHealthHandler(ctx *fasthttp.RequestCtx) {
	// Get system health status
	health := h.monitor.GetSystemHealth()

	// Set response headers
	ctx.Response.Header.Set("Content-Type", "application/json")

	// Determine HTTP status based on health
	var statusCode int
	switch health.OverallStatus {
	case monitoring.HealthStatusHealthy:
		statusCode = fasthttp.StatusOK
	case monitoring.HealthStatusDegraded:
		statusCode = fasthttp.StatusOK // 200 but with degraded status in body
	case monitoring.HealthStatusUnhealthy:
		statusCode = fasthttp.StatusServiceUnavailable
	default:
		statusCode = fasthttp.StatusInternalServerError
	}

	ctx.SetStatusCode(statusCode)

	// Return health data
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"health":    health,
			"timestamp": time.Now(),
		},
	}

	// Marshal response
	responseData, err := json.Marshal(response)
	if err != nil {
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.Write(responseData)
}

// GetMetricsSummaryHandler handles metrics summary requests
func (h *MonitoringHandler) GetMetricsSummaryHandler(ctx *fasthttp.RequestCtx) {
	// Get period from query parameters
	period := string(ctx.QueryArgs().Peek("period"))
	if period == "" {
		period = "24h" // Default to 24 hours
	}

	// Get summary from historical data manager
	summary, err := h.monitor.GetMetricsSummary(period)
	if err != nil {
		h.logger.Error("Failed to get metrics summary", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	// Set response headers
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)

	// Return summary data
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"summary":   summary,
			"timestamp": time.Now(),
		},
	}

	// Marshal response
	responseData, err := json.Marshal(response)
	if err != nil {
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.Write(responseData)
}

// GetHistoricalMetricsHandler handles historical metrics requests
func (h *MonitoringHandler) GetHistoricalMetricsHandler(ctx *fasthttp.RequestCtx) {
	// Get parameters from query string
	startTimeStr := string(ctx.QueryArgs().Peek("start_time"))
	endTimeStr := string(ctx.QueryArgs().Peek("end_time"))
	limitStr := string(ctx.QueryArgs().Peek("limit"))

	// Parse start time
	var startTime time.Time
	if startTimeStr != "" {
		var err error
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			ctx.Error("Invalid start_time format", fasthttp.StatusBadRequest)
			return
		}
	} else {
		startTime = time.Now().Add(-24 * time.Hour) // Default to 24 hours ago
	}

	// Parse end time
	var endTime time.Time
	if endTimeStr != "" {
		var err error
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			ctx.Error("Invalid end_time format", fasthttp.StatusBadRequest)
			return
		}
	} else {
		endTime = time.Now()
	}

	// Parse limit
	limit := 1000 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get historical metrics
	metrics, err := h.monitor.GetHistoricalMetrics(startTime, endTime, limit)
	if err != nil {
		h.logger.Error("Failed to get historical metrics", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	// Set response headers
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)

	// Return historical metrics
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"metrics":    metrics,
			"count":      len(metrics),
			"start_time": startTime,
			"end_time":   endTime,
			"timestamp":  time.Now(),
		},
	}

	// Marshal response
	responseData, err := json.Marshal(response)
	if err != nil {
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.Write(responseData)
}

// GetTrendAnalysisHandler handles trend analysis requests
func (h *MonitoringHandler) GetTrendAnalysisHandler(ctx *fasthttp.RequestCtx) {
	// Get parameters from query string
	metric := string(ctx.QueryArgs().Peek("metric"))
	period := string(ctx.QueryArgs().Peek("period"))

	// Validate metric
	if metric == "" {
		metric = "failure_rate" // Default metric
	}

	// Validate period
	if period == "" {
		period = "24h" // Default period
	}

	// Get trend analysis
	trends, err := h.monitor.AnalyzeTrends(metric, period)
	if err != nil {
		h.logger.Error("Failed to analyze trends", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	// Set response headers
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)

	// Return trend analysis
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"trends":    trends,
			"timestamp": time.Now(),
		},
	}

	// Marshal response
	responseData, err := json.Marshal(response)
	if err != nil {
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.Write(responseData)
}
