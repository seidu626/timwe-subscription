package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EnhancedMonitor provides comprehensive monitoring with automated recovery
type EnhancedMonitor struct {
	logger                 *zap.Logger
	msisdnValidationStats  *MSISDNValidationStats
	networkHealthStats     *NetworkHealthStats
	chargingFailureMonitor *ChargingFailureMonitor
	automatedRecovery      *AutomatedRecovery
	alertManager           *AlertManager
	mu                     sync.RWMutex
	isRunning              bool
	stopChan               chan struct{}
}

// MSISDNValidationStats tracks MSISDN validation metrics
type MSISDNValidationStats struct {
	TotalValidations     int64            `json:"total_validations"`
	ValidMSISDNs         int64            `json:"valid_msisdns"`
	InvalidMSISDNs       int64            `json:"invalid_msisdns"`
	ValidationErrors     int64            `json:"validation_errors"`
	CacheHits            int64            `json:"cache_hits"`
	CacheMisses          int64            `json:"cache_misses"`
	PreventedAPICalls    int64            `json:"prevented_api_calls"`
	ValidationLatency    time.Duration    `json:"validation_latency"`
	InvalidReasons       map[string]int64 `json:"invalid_reasons"`
	OperatorDistribution map[string]int64 `json:"operator_distribution"`
	LastUpdated          time.Time        `json:"last_updated"`
	mu                   sync.RWMutex
}

// NetworkHealthStats tracks network connectivity and performance
type NetworkHealthStats struct {
	TotalRequests       int64                      `json:"total_requests"`
	SuccessfulRequests  int64                      `json:"successful_requests"`
	FailedRequests      int64                      `json:"failed_requests"`
	TimeoutErrors       int64                      `json:"timeout_errors"`
	ConnectionErrors    int64                      `json:"connection_errors"`
	CircuitBreakerTrips int64                      `json:"circuit_breaker_trips"`
	AverageLatency      time.Duration              `json:"average_latency"`
	P95Latency          time.Duration              `json:"p95_latency"`
	P99Latency          time.Duration              `json:"p99_latency"`
	LastHealthCheck     time.Time                  `json:"last_health_check"`
	HealthCheckStatus   string                     `json:"health_check_status"`
	EndpointHealth      map[string]*EndpointHealth `json:"endpoint_health"`
	mu                  sync.RWMutex
}

// EndpointHealth tracks health of individual endpoints
type EndpointHealth struct {
	URL                 string        `json:"url"`
	Status              string        `json:"status"` // healthy, unhealthy, degraded
	LastCheck           time.Time     `json:"last_check"`
	ResponseTime        time.Duration `json:"response_time"`
	SuccessRate         float64       `json:"success_rate"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
}

// AutomatedRecovery handles automated recovery actions
type AutomatedRecovery struct {
	logger              *zap.Logger
	recoveryActions     map[string]RecoveryAction
	recoveryHistory     []RecoveryEvent
	isEnabled           bool
	maxRecoveryAttempts int
	recoveryInterval    time.Duration
	mu                  sync.RWMutex
}

// RecoveryAction defines an automated recovery action
type RecoveryAction struct {
	Name        string
	Description string
	Action      func(context.Context) error
	Cooldown    time.Duration
	LastRun     time.Time
}

// RecoveryEvent records a recovery action execution
type RecoveryEvent struct {
	Timestamp time.Time     `json:"timestamp"`
	Action    string        `json:"action"`
	Trigger   string        `json:"trigger"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// AlertManager handles enhanced alerting with different channels
type AlertManager struct {
	logger           *zap.Logger
	alertChannels    map[string]AlertChannel
	alertRules       []EnhancedAlertRule
	suppressedAlerts map[string]time.Time
	mu               sync.RWMutex
}

// AlertChannel defines an alert delivery channel
type AlertChannel interface {
	SendAlert(alert *Alert) error
	GetName() string
	IsEnabled() bool
}

// EnhancedAlertRule defines conditions for triggering alerts in enhanced monitoring
type EnhancedAlertRule struct {
	Name          string
	Condition     func(metrics interface{}) bool
	Severity      string
	Message       string
	Channels      []string
	Cooldown      time.Duration
	LastTriggered time.Time
}

// NewEnhancedMonitor creates a new enhanced monitoring system
func NewEnhancedMonitor(logger *zap.Logger, chargingMonitor *ChargingFailureMonitor) *EnhancedMonitor {
	return &EnhancedMonitor{
		logger: logger,
		msisdnValidationStats: &MSISDNValidationStats{
			InvalidReasons:       make(map[string]int64),
			OperatorDistribution: make(map[string]int64),
		},
		networkHealthStats: &NetworkHealthStats{
			EndpointHealth: make(map[string]*EndpointHealth),
		},
		chargingFailureMonitor: chargingMonitor,
		automatedRecovery: &AutomatedRecovery{
			logger:              logger,
			recoveryActions:     make(map[string]RecoveryAction),
			recoveryHistory:     make([]RecoveryEvent, 0),
			isEnabled:           true,
			maxRecoveryAttempts: 3,
			recoveryInterval:    5 * time.Minute,
		},
		alertManager: &AlertManager{
			logger:           logger,
			alertChannels:    make(map[string]AlertChannel),
			alertRules:       make([]EnhancedAlertRule, 0),
			suppressedAlerts: make(map[string]time.Time),
		},
		stopChan: make(chan struct{}),
	}
}

// Start starts the enhanced monitoring system
func (em *EnhancedMonitor) Start(ctx context.Context) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.isRunning {
		return fmt.Errorf("enhanced monitor is already running")
	}

	em.logger.Info("Starting enhanced monitoring system")
	em.isRunning = true

	// Initialize default alert rules
	em.initializeDefaultAlertRules()

	// Initialize automated recovery actions
	em.initializeRecoveryActions()

	// Start monitoring goroutines
	go em.monitoringLoop(ctx)
	go em.healthCheckLoop(ctx)
	go em.recoveryLoop(ctx)

	return nil
}

// Stop stops the enhanced monitoring system
func (em *EnhancedMonitor) Stop() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if !em.isRunning {
		return fmt.Errorf("enhanced monitor is not running")
	}

	em.logger.Info("Stopping enhanced monitoring system")
	close(em.stopChan)
	em.isRunning = false

	return nil
}

// RecordMSISDNValidation records MSISDN validation metrics
func (em *EnhancedMonitor) RecordMSISDNValidation(isValid bool, reason string, operator string, latency time.Duration, cacheHit bool, preventedAPICall bool) {
	em.msisdnValidationStats.mu.Lock()
	defer em.msisdnValidationStats.mu.Unlock()

	em.msisdnValidationStats.TotalValidations++
	em.msisdnValidationStats.ValidationLatency = latency
	em.msisdnValidationStats.LastUpdated = time.Now()

	if cacheHit {
		em.msisdnValidationStats.CacheHits++
	} else {
		em.msisdnValidationStats.CacheMisses++
	}

	if preventedAPICall {
		em.msisdnValidationStats.PreventedAPICalls++
	}

	if isValid {
		em.msisdnValidationStats.ValidMSISDNs++
		if operator != "" {
			em.msisdnValidationStats.OperatorDistribution[operator]++
		}
	} else {
		em.msisdnValidationStats.InvalidMSISDNs++
		if reason != "" {
			em.msisdnValidationStats.InvalidReasons[reason]++
		}
	}
}

// RecordNetworkRequest records network request metrics
func (em *EnhancedMonitor) RecordNetworkRequest(success bool, latency time.Duration, errorType string, endpoint string) {
	em.networkHealthStats.mu.Lock()
	defer em.networkHealthStats.mu.Unlock()

	em.networkHealthStats.TotalRequests++

	if success {
		em.networkHealthStats.SuccessfulRequests++
	} else {
		em.networkHealthStats.FailedRequests++

		switch errorType {
		case "timeout":
			em.networkHealthStats.TimeoutErrors++
		case "connection":
			em.networkHealthStats.ConnectionErrors++
		case "circuit_breaker":
			em.networkHealthStats.CircuitBreakerTrips++
		}
	}

	// Update average latency (simple moving average)
	if em.networkHealthStats.TotalRequests == 1 {
		em.networkHealthStats.AverageLatency = latency
	} else {
		em.networkHealthStats.AverageLatency = (em.networkHealthStats.AverageLatency + latency) / 2
	}

	// Update endpoint health
	if endpoint != "" {
		if health, exists := em.networkHealthStats.EndpointHealth[endpoint]; exists {
			health.LastCheck = time.Now()
			health.ResponseTime = latency

			if success {
				health.ConsecutiveFailures = 0
				health.Status = "healthy"
			} else {
				health.ConsecutiveFailures++
				if health.ConsecutiveFailures >= 3 {
					health.Status = "unhealthy"
				} else {
					health.Status = "degraded"
				}
			}
		} else {
			status := "healthy"
			consecutiveFailures := 0
			if !success {
				status = "degraded"
				consecutiveFailures = 1
			}

			em.networkHealthStats.EndpointHealth[endpoint] = &EndpointHealth{
				URL:                 endpoint,
				Status:              status,
				LastCheck:           time.Now(),
				ResponseTime:        latency,
				ConsecutiveFailures: consecutiveFailures,
			}
		}
	}
}

// GetMSISDNValidationStats returns MSISDN validation statistics
func (em *EnhancedMonitor) GetMSISDNValidationStats() *MSISDNValidationStats {
	em.msisdnValidationStats.mu.RLock()
	defer em.msisdnValidationStats.mu.RUnlock()

	// Return a copy to avoid race conditions
	stats := &MSISDNValidationStats{
		TotalValidations:     em.msisdnValidationStats.TotalValidations,
		ValidMSISDNs:         em.msisdnValidationStats.ValidMSISDNs,
		InvalidMSISDNs:       em.msisdnValidationStats.InvalidMSISDNs,
		ValidationErrors:     em.msisdnValidationStats.ValidationErrors,
		CacheHits:            em.msisdnValidationStats.CacheHits,
		CacheMisses:          em.msisdnValidationStats.CacheMisses,
		PreventedAPICalls:    em.msisdnValidationStats.PreventedAPICalls,
		ValidationLatency:    em.msisdnValidationStats.ValidationLatency,
		LastUpdated:          em.msisdnValidationStats.LastUpdated,
		InvalidReasons:       make(map[string]int64),
		OperatorDistribution: make(map[string]int64),
	}

	// Copy maps
	for k, v := range em.msisdnValidationStats.InvalidReasons {
		stats.InvalidReasons[k] = v
	}
	for k, v := range em.msisdnValidationStats.OperatorDistribution {
		stats.OperatorDistribution[k] = v
	}

	return stats
}

// GetNetworkHealthStats returns network health statistics
func (em *EnhancedMonitor) GetNetworkHealthStats() *NetworkHealthStats {
	em.networkHealthStats.mu.RLock()
	defer em.networkHealthStats.mu.RUnlock()

	// Return a copy to avoid race conditions
	stats := &NetworkHealthStats{
		TotalRequests:       em.networkHealthStats.TotalRequests,
		SuccessfulRequests:  em.networkHealthStats.SuccessfulRequests,
		FailedRequests:      em.networkHealthStats.FailedRequests,
		TimeoutErrors:       em.networkHealthStats.TimeoutErrors,
		ConnectionErrors:    em.networkHealthStats.ConnectionErrors,
		CircuitBreakerTrips: em.networkHealthStats.CircuitBreakerTrips,
		AverageLatency:      em.networkHealthStats.AverageLatency,
		P95Latency:          em.networkHealthStats.P95Latency,
		P99Latency:          em.networkHealthStats.P99Latency,
		LastHealthCheck:     em.networkHealthStats.LastHealthCheck,
		HealthCheckStatus:   em.networkHealthStats.HealthCheckStatus,
		EndpointHealth:      make(map[string]*EndpointHealth),
	}

	// Copy endpoint health map
	for k, v := range em.networkHealthStats.EndpointHealth {
		stats.EndpointHealth[k] = &EndpointHealth{
			URL:                 v.URL,
			Status:              v.Status,
			LastCheck:           v.LastCheck,
			ResponseTime:        v.ResponseTime,
			SuccessRate:         v.SuccessRate,
			ConsecutiveFailures: v.ConsecutiveFailures,
		}
	}

	return stats
}

// monitoringLoop runs the main monitoring logic
func (em *EnhancedMonitor) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-em.stopChan:
			return
		case <-ticker.C:
			em.checkAlertRules()
		}
	}
}

// healthCheckLoop performs periodic health checks
func (em *EnhancedMonitor) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second) // Health check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-em.stopChan:
			return
		case <-ticker.C:
			em.performHealthChecks(ctx)
		}
	}
}

// recoveryLoop handles automated recovery actions
func (em *EnhancedMonitor) recoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(em.automatedRecovery.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-em.stopChan:
			return
		case <-ticker.C:
			em.executeRecoveryActions(ctx)
		}
	}
}

// initializeDefaultAlertRules sets up default alert rules
func (em *EnhancedMonitor) initializeDefaultAlertRules() {
	em.alertManager.alertRules = []EnhancedAlertRule{
		{
			Name:     "high_invalid_msisdn_rate",
			Severity: "high",
			Message:  "High INVALID_MSISDN rate detected",
			Channels: []string{"log", "webhook"},
			Cooldown: 5 * time.Minute,
			Condition: func(metrics interface{}) bool {
				if stats, ok := metrics.(*MSISDNValidationStats); ok {
					if stats.TotalValidations > 100 {
						invalidRate := float64(stats.InvalidMSISDNs) / float64(stats.TotalValidations)
						return invalidRate > 0.3 // Alert if more than 30% invalid
					}
				}
				return false
			},
		},
		{
			Name:     "network_connectivity_issues",
			Severity: "critical",
			Message:  "Network connectivity issues detected",
			Channels: []string{"log", "webhook"},
			Cooldown: 2 * time.Minute,
			Condition: func(metrics interface{}) bool {
				if stats, ok := metrics.(*NetworkHealthStats); ok {
					if stats.TotalRequests > 50 {
						errorRate := float64(stats.FailedRequests) / float64(stats.TotalRequests)
						return errorRate > 0.2 // Alert if more than 20% errors
					}
				}
				return false
			},
		},
		{
			Name:     "prevented_api_calls_high",
			Severity: "medium",
			Message:  "High number of prevented API calls due to validation",
			Channels: []string{"log"},
			Cooldown: 10 * time.Minute,
			Condition: func(metrics interface{}) bool {
				if stats, ok := metrics.(*MSISDNValidationStats); ok {
					return stats.PreventedAPICalls > 1000 // Alert if more than 1000 prevented calls
				}
				return false
			},
		},
	}
}

// initializeRecoveryActions sets up automated recovery actions
func (em *EnhancedMonitor) initializeRecoveryActions() {
	em.automatedRecovery.recoveryActions = map[string]RecoveryAction{
		"clear_msisdn_cache": {
			Name:        "Clear MSISDN Cache",
			Description: "Clears MSISDN validation cache to refresh validation results",
			Cooldown:    10 * time.Minute,
			Action: func(ctx context.Context) error {
				em.logger.Info("Executing automated recovery: clearing MSISDN cache")
				// Implementation would clear the cache
				return nil
			},
		},
		"reset_circuit_breaker": {
			Name:        "Reset Circuit Breaker",
			Description: "Resets circuit breaker to allow new requests",
			Cooldown:    5 * time.Minute,
			Action: func(ctx context.Context) error {
				em.logger.Info("Executing automated recovery: resetting circuit breaker")
				// Implementation would reset circuit breaker
				return nil
			},
		},
	}
}

// checkAlertRules evaluates alert rules and triggers alerts
func (em *EnhancedMonitor) checkAlertRules() {
	msisdnStats := em.GetMSISDNValidationStats()
	networkStats := em.GetNetworkHealthStats()

	for _, rule := range em.alertManager.alertRules {
		// Check cooldown
		if time.Since(rule.LastTriggered) < rule.Cooldown {
			continue
		}

		// Check MSISDN validation conditions
		if rule.Condition(msisdnStats) {
			em.triggerAlert(&rule, msisdnStats)
			continue
		}

		// Check network health conditions
		if rule.Condition(networkStats) {
			em.triggerAlert(&rule, networkStats)
			continue
		}
	}
}

// triggerAlert triggers an alert
func (em *EnhancedMonitor) triggerAlert(rule *EnhancedAlertRule, metrics interface{}) {
	alert := &Alert{
		ID:        fmt.Sprintf("%s_%d", rule.Name, time.Now().Unix()),
		Type:      rule.Name,
		Severity:  rule.Severity,
		Message:   rule.Message,
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{"metrics": metrics},
	}

	em.logger.Warn("Alert triggered",
		zap.String("rule", rule.Name),
		zap.String("severity", rule.Severity),
		zap.String("message", rule.Message))

	// Update last triggered time
	rule.LastTriggered = time.Now()

	// Send alert through configured channels
	for _, channelName := range rule.Channels {
		if channel, exists := em.alertManager.alertChannels[channelName]; exists && channel.IsEnabled() {
			if err := channel.SendAlert(alert); err != nil {
				em.logger.Error("Failed to send alert",
					zap.String("channel", channelName),
					zap.Error(err))
			}
		}
	}
}

// performHealthChecks performs health checks on external services
func (em *EnhancedMonitor) performHealthChecks(ctx context.Context) {
	em.networkHealthStats.mu.Lock()
	em.networkHealthStats.LastHealthCheck = time.Now()
	em.networkHealthStats.HealthCheckStatus = "checking"
	em.networkHealthStats.mu.Unlock()

	// Perform health checks on known endpoints
	// This would be implemented based on actual endpoints
	em.logger.Debug("Performing health checks")

	em.networkHealthStats.mu.Lock()
	em.networkHealthStats.HealthCheckStatus = "completed"
	em.networkHealthStats.mu.Unlock()
}

// executeRecoveryActions executes automated recovery actions when needed
func (em *EnhancedMonitor) executeRecoveryActions(ctx context.Context) {
	if !em.automatedRecovery.isEnabled {
		return
	}

	// Check if recovery is needed based on metrics
	msisdnStats := em.GetMSISDNValidationStats()
	networkStats := em.GetNetworkHealthStats()

	// Example recovery triggers
	if msisdnStats.TotalValidations > 1000 {
		invalidRate := float64(msisdnStats.InvalidMSISDNs) / float64(msisdnStats.TotalValidations)
		if invalidRate > 0.5 { // If more than 50% invalid, clear cache
			em.executeRecoveryAction(ctx, "clear_msisdn_cache", "high_invalid_rate")
		}
	}

	if networkStats.TotalRequests > 100 {
		errorRate := float64(networkStats.FailedRequests) / float64(networkStats.TotalRequests)
		if errorRate > 0.3 { // If more than 30% errors, reset circuit breaker
			em.executeRecoveryAction(ctx, "reset_circuit_breaker", "high_error_rate")
		}
	}
}

// executeRecoveryAction executes a specific recovery action
func (em *EnhancedMonitor) executeRecoveryAction(ctx context.Context, actionName, trigger string) {
	em.automatedRecovery.mu.Lock()
	defer em.automatedRecovery.mu.Unlock()

	action, exists := em.automatedRecovery.recoveryActions[actionName]
	if !exists {
		return
	}

	// Check cooldown
	if time.Since(action.LastRun) < action.Cooldown {
		return
	}

	startTime := time.Now()
	err := action.Action(ctx)
	duration := time.Since(startTime)

	// Record recovery event
	event := RecoveryEvent{
		Timestamp: startTime,
		Action:    actionName,
		Trigger:   trigger,
		Success:   err == nil,
		Duration:  duration,
	}

	if err != nil {
		event.Error = err.Error()
		em.logger.Error("Recovery action failed",
			zap.String("action", actionName),
			zap.String("trigger", trigger),
			zap.Error(err))
	} else {
		em.logger.Info("Recovery action executed successfully",
			zap.String("action", actionName),
			zap.String("trigger", trigger),
			zap.Duration("duration", duration))
	}

	em.automatedRecovery.recoveryHistory = append(em.automatedRecovery.recoveryHistory, event)
	action.LastRun = time.Now()
	em.automatedRecovery.recoveryActions[actionName] = action
}

// GetRecoveryHistory returns the history of recovery actions
func (em *EnhancedMonitor) GetRecoveryHistory() []RecoveryEvent {
	em.automatedRecovery.mu.RLock()
	defer em.automatedRecovery.mu.RUnlock()

	// Return a copy
	history := make([]RecoveryEvent, len(em.automatedRecovery.recoveryHistory))
	copy(history, em.automatedRecovery.recoveryHistory)
	return history
}
