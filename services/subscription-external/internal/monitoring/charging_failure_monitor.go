package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ChargingFailureMetrics represents key metrics for monitoring
type ChargingFailureMetrics struct {
	TotalSubscriptions    int64                  `json:"total_subscriptions"`
	ChargingFailures      int64                  `json:"charging_failures"`
	FailureRate           float64                `json:"failure_rate"`
	NeverCharged          int64                  `json:"never_charged"`
	StaleCharges          int64                  `json:"stale_charges"`
	ChargingRecent        int64                  `json:"charging_recent"`
	ChargingDelayed       int64                  `json:"charging_delayed"`
	ChargingStale         int64                  `json:"charging_stale"`
	ProcessingQueue       int64                  `json:"processing_queue"`
	ProcessedToday        int64                  `json:"processed_today"`
	SuccessRate           float64                `json:"success_rate"`
	LastUpdated           time.Time              `json:"last_updated"`
	ProcessingStatus      string                 `json:"processing_status"`
	AverageProcessingTime float64                `json:"average_processing_time"`
	Metadata              map[string]interface{} `json:"metadata"`
}

// AlertThresholds defines when alerts should be triggered
type AlertThresholds struct {
	HighFailureRate    float64 `json:"high_failure_rate"`   // Alert if > 80%
	LowSuccessRate     float64 `json:"low_success_rate"`    // Alert if < 60%
	HighQueueSize      int64   `json:"high_queue_size"`     // Alert if > 10000
	ProcessingDelay    float64 `json:"processing_delay"`    // Alert if > 5 minutes
	DatabaseErrors     int64   `json:"database_errors"`     // Alert if > 10 in 1 hour
	ServiceUnavailable bool    `json:"service_unavailable"` // Alert if service down
}

// Alert represents a monitoring alert
type Alert struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Severity     string                 `json:"severity"` // low, medium, high, critical
	Message      string                 `json:"message"`
	Metric       string                 `json:"metric"`
	Value        interface{}            `json:"value"`
	Threshold    interface{}            `json:"threshold"`
	Timestamp    time.Time              `json:"timestamp"`
	Acknowledged bool                   `json:"acknowledged"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ChargingFailureMonitor handles monitoring and alerting for charging failures
type ChargingFailureMonitor struct {
	metrics         *ChargingFailureMetrics
	thresholds      *AlertThresholds
	alerts          []*Alert
	mu              sync.RWMutex
	logger          *zap.Logger
	alertChan       chan *Alert
	stopChan        chan struct{}
	isRunning       bool
	realTimeMonitor *RealTimeMonitor     // Reference to real-time monitor for broadcasting
	healthMonitor   *SystemHealthMonitor // Reference to system health monitor
}

// NewChargingFailureMonitor creates a new monitoring instance
func NewChargingFailureMonitor(logger *zap.Logger) *ChargingFailureMonitor {
	return &ChargingFailureMonitor{
		metrics: &ChargingFailureMetrics{
			TotalSubscriptions:    0,
			ChargingFailures:      0,
			FailureRate:           0.0,
			NeverCharged:          0,
			StaleCharges:          0,
			ChargingRecent:        0,
			ChargingDelayed:       0,
			ChargingStale:         0,
			ProcessingQueue:       0,
			ProcessedToday:        0,
			SuccessRate:           100.0,
			LastUpdated:           time.Now(),
			ProcessingStatus:      "idle",
			AverageProcessingTime: 0.0,
			Metadata:              make(map[string]interface{}),
		},
		thresholds: &AlertThresholds{
			HighFailureRate:    80.0,
			LowSuccessRate:     60.0,
			HighQueueSize:      10000,
			ProcessingDelay:    5.0, // minutes
			DatabaseErrors:     10,
			ServiceUnavailable: false,
		},
		alerts:    make([]*Alert, 0),
		logger:    logger,
		alertChan: make(chan *Alert, 100),
		stopChan:  make(chan struct{}),
	}
}

// Start begins the monitoring process
func (m *ChargingFailureMonitor) Start(ctx context.Context) error {
	if m.isRunning {
		return fmt.Errorf("monitor is already running")
	}

	m.isRunning = true
	m.logger.Info("Starting charging failure monitor")

	// Generate sample data for testing
	// m.generateSampleMetrics()
	// m.generateSampleAlerts()

	// Start monitoring goroutine
	go m.monitoringLoop(ctx)

	// Start alert processing goroutine
	go m.alertProcessor(ctx)

	return nil
}

// Stop stops the monitoring process
func (m *ChargingFailureMonitor) Stop() {
	if !m.isRunning {
		return
	}

	m.logger.Info("Stopping charging failure monitor")
	close(m.stopChan)
	m.isRunning = false
}

// SetRealTimeMonitor sets the real-time monitor reference for broadcasting
func (m *ChargingFailureMonitor) SetRealTimeMonitor(rtm *RealTimeMonitor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.realTimeMonitor = rtm
}

// UpdateMetrics updates the metrics with new data
func (m *ChargingFailureMonitor) UpdateMetrics(metrics *ChargingFailureMetrics) {
	m.logger.Info("UpdateMetrics: Starting update process")

	m.mu.Lock()
	m.logger.Info("UpdateMetrics: Acquired lock")

	// Debug logging
	m.logger.Info("UpdateMetrics called",
		zap.Int64("total_subscriptions", metrics.TotalSubscriptions),
		zap.Int64("charging_failures", metrics.ChargingFailures),
		zap.Float64("failure_rate", metrics.FailureRate),
		zap.Int64("never_charged", metrics.NeverCharged))

	// Store the old metrics for comparison
	oldMetrics := m.metrics
	if oldMetrics != nil {
		m.logger.Info("UpdateMetrics: Previous metrics existed",
			zap.Int64("old_total_subscriptions", oldMetrics.TotalSubscriptions),
			zap.Int64("old_charging_failures", oldMetrics.ChargingFailures))
	} else {
		m.logger.Info("UpdateMetrics: No previous metrics existed")
	}

	// Update the metrics
	m.metrics = metrics
	m.logger.Info("UpdateMetrics: Metrics pointer updated")

	// Verify the update worked
	if m.metrics != nil {
		m.logger.Info("UpdateMetrics: Verification - metrics stored successfully",
			zap.Int64("stored_total_subscriptions", m.metrics.TotalSubscriptions),
			zap.Int64("stored_charging_failures", m.metrics.ChargingFailures),
			zap.Float64("stored_failure_rate", m.metrics.FailureRate),
			zap.Int64("stored_never_charged", m.metrics.NeverCharged))
	} else {
		m.logger.Error("UpdateMetrics: CRITICAL ERROR - metrics is nil after update!")
	}

	// Broadcast real-time update if real-time monitor is available
	if m.realTimeMonitor != nil && m.realTimeMonitor.IsRunning() {
		m.realTimeMonitor.BroadcastMetricsUpdate(m.metrics)
	}

	m.mu.Unlock()
	m.logger.Info("UpdateMetrics: Released lock")

	// Check for threshold violations and generate alerts
	m.logger.Info("UpdateMetrics: Calling checkThresholds")
	m.checkThresholds()
	m.logger.Info("UpdateMetrics: Update process completed")
}

// UpdateProcessingMetrics updates only processing-related metrics
func (m *ChargingFailureMonitor) UpdateProcessingMetrics(processingQueue, processedToday int64, successRate, averageProcessingTime float64, processingStatus string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.metrics != nil {
		m.metrics.ProcessingQueue = processingQueue
		m.metrics.ProcessedToday = processedToday
		m.metrics.SuccessRate = successRate
		m.metrics.AverageProcessingTime = averageProcessingTime
		m.metrics.ProcessingStatus = processingStatus
		m.metrics.LastUpdated = time.Now()
	}
}

// GetMetrics returns the current metrics
func (m *ChargingFailureMonitor) GetMetrics() *ChargingFailureMetrics {
	m.logger.Info("GetMetrics: Starting retrieval")

	m.mu.RLock()
	m.logger.Info("GetMetrics: Acquired read lock")

	// Debug logging
	if m.metrics != nil {
		m.logger.Info("GetMetrics called, returning metrics",
			zap.Int64("total_subscriptions", m.metrics.TotalSubscriptions),
			zap.Int64("charging_failures", m.metrics.ChargingFailures),
			zap.Float64("failure_rate", m.metrics.FailureRate),
			zap.Int64("never_charged", m.metrics.NeverCharged))
	} else {
		m.logger.Warn("GetMetrics called, but metrics is nil")
	}

	result := m.metrics
	m.mu.RUnlock()
	m.logger.Info("GetMetrics: Released read lock, returning result")

	return result
}

// GetAlerts returns all alerts (optionally filtered by severity)
func (m *ChargingFailureMonitor) GetAlerts(severity string) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if severity == "" {
		return m.alerts
	}

	var filtered []*Alert
	for _, alert := range m.alerts {
		if alert.Severity == severity {
			filtered = append(filtered, alert)
		}
	}

	return filtered
}

// AcknowledgeAlert marks an alert as acknowledged
func (m *ChargingFailureMonitor) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			m.logger.Info("Alert acknowledged", zap.String("alert_id", alertID))
			return nil
		}
	}

	return fmt.Errorf("alert not found: %s", alertID)
}

// ClearAlerts removes old acknowledged alerts
func (m *ChargingFailureMonitor) ClearAlerts(olderThan time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var activeAlerts []*Alert
	cutoff := time.Now().Add(-olderThan)

	for _, alert := range m.alerts {
		if !alert.Acknowledged || alert.Timestamp.After(cutoff) {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	m.alerts = activeAlerts
	m.logger.Info("Cleared old alerts", zap.Int("remaining", len(m.alerts)))
}

// monitoringLoop runs the main monitoring logic
func (m *ChargingFailureMonitor) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// alertProcessor handles alert distribution
func (m *ChargingFailureMonitor) alertProcessor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case alert := <-m.alertChan:
			m.processAlert(alert)
		}
	}
}

// performHealthCheck performs system health checks
func (m *ChargingFailureMonitor) performHealthCheck() {
	m.mu.RLock()
	metrics := m.metrics
	m.mu.RUnlock()

	// Check if metrics are stale
	if time.Since(metrics.LastUpdated) > 5*time.Minute {
		m.generateAlert("metrics_stale", "high", "Metrics are not being updated",
			"last_updated", metrics.LastUpdated, "5_minutes", nil)
	}

	// Check processing status
	if metrics.ProcessingStatus == "error" {
		m.generateAlert("processing_error", "critical", "Processing has encountered errors",
			"processing_status", metrics.ProcessingStatus, "error", nil)
	}
}

// checkThresholds checks if any metrics violate thresholds
func (m *ChargingFailureMonitor) checkThresholds() {
	metrics := m.metrics
	thresholds := m.thresholds

	// Check failure rate
	if metrics.FailureRate > thresholds.HighFailureRate {
		m.generateAlert("high_failure_rate", "high",
			fmt.Sprintf("Charging failure rate is %.2f%% (threshold: %.2f%%)",
				metrics.FailureRate, thresholds.HighFailureRate),
			"failure_rate", metrics.FailureRate, thresholds.HighFailureRate, nil)
	}

	// Check success rate
	if metrics.SuccessRate > 0 && metrics.SuccessRate < thresholds.LowSuccessRate {
		m.generateAlert("low_success_rate", "medium",
			fmt.Sprintf("Processing success rate is %.2f%% (threshold: %.2f%%)",
				metrics.SuccessRate, thresholds.LowSuccessRate),
			"success_rate", metrics.SuccessRate, thresholds.LowSuccessRate, nil)
	}

	// Check queue size
	if metrics.ProcessingQueue > thresholds.HighQueueSize {
		m.generateAlert("high_queue_size", "medium",
			fmt.Sprintf("Processing queue size is %d (threshold: %d)",
				metrics.ProcessingQueue, thresholds.HighQueueSize),
			"queue_size", metrics.ProcessingQueue, thresholds.HighQueueSize, nil)
	}

	// Check processing delay
	if metrics.AverageProcessingTime > thresholds.ProcessingDelay {
		m.generateAlert("processing_delay", "medium",
			fmt.Sprintf("Average processing time is %.2f minutes (threshold: %.2f)",
				metrics.AverageProcessingTime, thresholds.ProcessingDelay),
			"processing_time", metrics.AverageProcessingTime, thresholds.ProcessingDelay, nil)
	}
}

// generateAlert creates and queues a new alert
func (m *ChargingFailureMonitor) generateAlert(alertType, severity, message, metric string,
	value, threshold interface{}, metadata map[string]interface{}) {

	alert := &Alert{
		ID:           fmt.Sprintf("%s_%d", alertType, time.Now().Unix()),
		Type:         alertType,
		Severity:     severity,
		Message:      message,
		Metric:       metric,
		Value:        value,
		Threshold:    threshold,
		Timestamp:    time.Now(),
		Acknowledged: false,
		Metadata:     metadata,
	}

	// Add to alerts list
	m.mu.Lock()
	m.alerts = append(m.alerts, alert)
	m.mu.Unlock()

	// Send to alert channel
	select {
	case m.alertChan <- alert:
		m.logger.Info("Alert generated",
			zap.String("type", alertType),
			zap.String("severity", severity),
			zap.String("message", message))
	default:
		m.logger.Warn("Alert channel full, dropping alert", zap.String("type", alertType))
	}
}

// processAlert handles alert distribution and logging
func (m *ChargingFailureMonitor) processAlert(alert *Alert) {
	// Log the alert
	m.logger.Info("Processing alert",
		zap.String("id", alert.ID),
		zap.String("type", alert.Type),
		zap.String("severity", alert.Severity),
		zap.String("message", alert.Message),
		zap.Any("value", alert.Value),
		zap.Any("threshold", alert.Threshold))

	// Broadcast alert in real-time if real-time monitor is available
	if m.realTimeMonitor != nil && m.realTimeMonitor.IsRunning() {
		m.realTimeMonitor.BroadcastAlert(alert)
	}

	// TODO: Implement alert distribution (email, Slack, webhook, etc.)
	// For now, just log the alert
	switch alert.Severity {
	case "critical":
		m.logger.Error("CRITICAL ALERT", zap.String("message", alert.Message))
	case "high":
		m.logger.Warn("HIGH PRIORITY ALERT", zap.String("message", alert.Message))
	case "medium":
		m.logger.Warn("MEDIUM PRIORITY ALERT", zap.String("message", alert.Message))
	case "low":
		m.logger.Info("LOW PRIORITY ALERT", zap.String("message", alert.Message))
	}
}

// GetDashboardData returns formatted data for dashboard display
func (m *ChargingFailureMonitor) GetDashboardData() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := m.metrics
	alerts := m.alerts

	// Count unacknowledged alerts by severity
	alertCounts := make(map[string]int)
	for _, alert := range alerts {
		if !alert.Acknowledged {
			alertCounts[alert.Severity]++
		}
	}

	return map[string]interface{}{
		"metrics": map[string]interface{}{
			"total_subscriptions":     metrics.TotalSubscriptions,
			"charging_failures":       metrics.ChargingFailures,
			"failure_rate":            metrics.FailureRate,
			"never_charged":           metrics.NeverCharged,
			"stale_charges":           metrics.StaleCharges,
			"charging_recent":         metrics.ChargingRecent,
			"charging_delayed":        metrics.ChargingDelayed,
			"charging_stale":          metrics.ChargingStale,
			"processing_queue":        metrics.ProcessingQueue,
			"processed_today":         metrics.ProcessedToday,
			"success_rate":            metrics.SuccessRate,
			"last_updated":            metrics.LastUpdated,
			"processing_status":       metrics.ProcessingStatus,
			"average_processing_time": metrics.AverageProcessingTime,
			"metadata":                metrics.Metadata,
		},
		"alerts": map[string]interface{}{
			"total":          len(alerts),
			"unacknowledged": len(alerts) - countAcknowledged(alerts),
			"by_severity":    alertCounts,
			"recent":         getRecentAlerts(alerts, 10),
		},
		"status": map[string]interface{}{
			"monitor_running": m.isRunning,
			"last_check":      time.Now(),
			"uptime":          time.Since(metrics.LastUpdated).String(),
		},
	}
}

// Helper functions
func countAcknowledged(alerts []*Alert) int {
	count := 0
	for _, alert := range alerts {
		if alert.Acknowledged {
			count++
		}
	}
	return count
}

func getRecentAlerts(alerts []*Alert, limit int) []*Alert {
	if len(alerts) <= limit {
		return alerts
	}
	return alerts[len(alerts)-limit:]
}

// GetSystemHealth returns the system health status
func (m *ChargingFailureMonitor) GetSystemHealth() *SystemHealth {
	if m.healthMonitor != nil {
		return m.healthMonitor.GetHealth()
	}

	// Return basic health if no health monitor is configured
	return &SystemHealth{
		OverallStatus: HealthStatusUnknown,
		LastCheck:     time.Now(),
		Components:    make(map[string]*ComponentHealth),
		StartTime:     time.Now(),
	}
}

// SetHealthMonitor sets the system health monitor reference
func (m *ChargingFailureMonitor) SetHealthMonitor(hm *SystemHealthMonitor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthMonitor = hm
}

// IsRunning returns whether the monitor is running
func (m *ChargingFailureMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// GetMetricsSummary returns a summary of historical metrics
func (m *ChargingFailureMonitor) GetMetricsSummary(period string) (map[string]interface{}, error) {
	// For now, return a basic summary from current metrics
	// In a full implementation, this would query the historical data manager
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := map[string]interface{}{
		"period":                period,
		"start_time":            time.Now().Add(-24 * time.Hour),
		"end_time":              time.Now(),
		"data_points":           1,
		"avg_failure_rate":      m.metrics.FailureRate,
		"avg_success_rate":      m.metrics.SuccessRate,
		"max_subscriptions":     m.metrics.TotalSubscriptions,
		"min_subscriptions":     m.metrics.TotalSubscriptions,
		"avg_charging_failures": float64(m.metrics.ChargingFailures),
	}

	return summary, nil
}

// GetHistoricalMetrics retrieves historical metrics for a time range
func (m *ChargingFailureMonitor) GetHistoricalMetrics(startTime, endTime time.Time, limit int) ([]*HistoricalMetrics, error) {
	// For now, return current metrics as historical data
	// In a full implementation, this would query the historical data manager
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a single historical metrics entry from current data
	historicalMetrics := &HistoricalMetrics{
		ID:                    1,
		Timestamp:             m.metrics.LastUpdated,
		TotalSubscriptions:    m.metrics.TotalSubscriptions,
		ChargingFailures:      m.metrics.ChargingFailures,
		FailureRate:           m.metrics.FailureRate,
		SuccessRate:           m.metrics.SuccessRate,
		NeverCharged:          m.metrics.NeverCharged,
		ChargingRecent:        m.metrics.ChargingRecent,
		ChargingDelayed:       m.metrics.ChargingDelayed,
		ChargingStale:         m.metrics.ChargingStale,
		ProcessingStatus:      m.metrics.ProcessingStatus,
		ProcessedToday:        m.metrics.ProcessedToday,
		ProcessingQueue:       m.metrics.ProcessingQueue,
		AverageProcessingTime: m.metrics.AverageProcessingTime,
		Metadata:              m.metrics.Metadata,
	}

	return []*HistoricalMetrics{historicalMetrics}, nil
}

// AnalyzeTrends performs trend analysis on historical data
func (m *ChargingFailureMonitor) AnalyzeTrends(metric string, period string) (*TrendAnalysis, error) {
	// For now, return a basic trend analysis
	// In a full implementation, this would query the historical data manager
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a simple trend analysis
	trendAnalysis := &TrendAnalysis{
		Metric:        metric,
		Period:        period,
		StartTime:     time.Now().Add(-24 * time.Hour),
		EndTime:       time.Now(),
		CurrentValue:  0.0,
		PreviousValue: 0.0,
		Change:        0.0,
		ChangePercent: 0.0,
		Trend:         "stable",
		DataPoints:    []DataPoint{},
	}

	// Set values based on the requested metric
	switch metric {
	case "failure_rate":
		trendAnalysis.CurrentValue = m.metrics.FailureRate
		trendAnalysis.PreviousValue = m.metrics.FailureRate
	case "success_rate":
		trendAnalysis.CurrentValue = m.metrics.SuccessRate
		trendAnalysis.PreviousValue = m.metrics.SuccessRate
	case "total_subscriptions":
		trendAnalysis.CurrentValue = float64(m.metrics.TotalSubscriptions)
		trendAnalysis.PreviousValue = float64(m.metrics.TotalSubscriptions)
	case "charging_failures":
		trendAnalysis.CurrentValue = float64(m.metrics.ChargingFailures)
		trendAnalysis.PreviousValue = float64(m.metrics.ChargingFailures)
	}

	return trendAnalysis, nil
}
