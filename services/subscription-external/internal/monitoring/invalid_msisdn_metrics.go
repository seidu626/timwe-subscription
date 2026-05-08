package monitoring

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// InvalidMSISDNMetrics tracks metrics for INVALID_MSISDN handling operations
type InvalidMSISDNMetrics struct {
	mu sync.RWMutex

	// Counters
	TotalInvalidMSISDNsDetected int64
	TotalLogsCreated            int64
	TotalSubscriptionsCleaned   int64
	TotalCleanupFailures        int64
	TotalRetryAttempts          int64

	// Timing metrics
	TotalCleanupTime   time.Duration
	AverageCleanupTime time.Duration
	MinCleanupTime     time.Duration
	MaxCleanupTime     time.Duration

	// Batch processing metrics
	TotalBatchesProcessed int64
	TotalBatchItems       int64
	AverageBatchSize      float64

	// Error tracking
	LastError          string
	LastErrorTimestamp time.Time
	ErrorCountByType   map[string]int64

	// Performance metrics
	CleanupOperationsPerSecond float64
	LastCleanupTimestamp       time.Time

	logger *zap.Logger
}

// NewInvalidMSISDNMetrics creates a new metrics collector
func NewInvalidMSISDNMetrics(logger *zap.Logger) *InvalidMSISDNMetrics {
	return &InvalidMSISDNMetrics{
		ErrorCountByType: make(map[string]int64),
		logger:           logger,
		MinCleanupTime:   time.Hour, // Initialize with a large value
	}
}

// RecordInvalidMSISDNDetected records when an INVALID_MSISDN is detected
func (m *InvalidMSISDNMetrics) RecordInvalidMSISDNDetected() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalInvalidMSISDNsDetected++
	m.LastCleanupTimestamp = time.Now()
}

// RecordLogCreated records when an invalid MSISDN log is created
func (m *InvalidMSISDNMetrics) RecordLogCreated() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalLogsCreated++
}

// RecordSubscriptionCleaned records when a subscription is successfully cleaned up
func (m *InvalidMSISDNMetrics) RecordSubscriptionCleaned(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalSubscriptionsCleaned++
	m.TotalCleanupTime += duration

	// Update timing statistics
	if duration < m.MinCleanupTime {
		m.MinCleanupTime = duration
	}
	if duration > m.MaxCleanupTime {
		m.MaxCleanupTime = duration
	}

	// Calculate average
	if m.TotalSubscriptionsCleaned > 0 {
		m.AverageCleanupTime = m.TotalCleanupTime / time.Duration(m.TotalSubscriptionsCleaned)
	}

	m.LastCleanupTimestamp = time.Now()
}

// RecordCleanupFailure records when a cleanup operation fails
func (m *InvalidMSISDNMetrics) RecordCleanupFailure(errorType string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalCleanupFailures++
	m.LastError = err.Error()
	m.LastErrorTimestamp = time.Now()

	// Track error counts by type
	m.ErrorCountByType[errorType]++

	m.logger.Error("INVALID_MSISDN cleanup failure recorded",
		zap.String("errorType", errorType),
		zap.Error(err),
		zap.Int64("totalFailures", m.TotalCleanupFailures))
}

// RecordRetryAttempt records when a retry attempt is made
func (m *InvalidMSISDNMetrics) RecordRetryAttempt() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRetryAttempts++
}

// RecordBatchProcessed records when a batch is processed
func (m *InvalidMSISDNMetrics) RecordBatchProcessed(batchSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalBatchesProcessed++
	m.TotalBatchItems += int64(batchSize)

	// Calculate average batch size
	if m.TotalBatchesProcessed > 0 {
		m.AverageBatchSize = float64(m.TotalBatchItems) / float64(m.TotalBatchesProcessed)
	}
}

// GetMetricsSnapshot returns a snapshot of current metrics
func (m *InvalidMSISDNMetrics) GetMetricsSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate operations per second if we have data
	var opsPerSecond float64
	if m.LastCleanupTimestamp.After(time.Time{}) {
		timeSinceLast := time.Since(m.LastCleanupTimestamp)
		if timeSinceLast > 0 {
			opsPerSecond = float64(m.TotalSubscriptionsCleaned) / timeSinceLast.Seconds()
		}
	}

	return map[string]interface{}{
		"total_invalid_msisdns_detected": m.TotalInvalidMSISDNsDetected,
		"total_logs_created":             m.TotalLogsCreated,
		"total_subscriptions_cleaned":    m.TotalSubscriptionsCleaned,
		"total_cleanup_failures":         m.TotalCleanupFailures,
		"total_retry_attempts":           m.TotalRetryAttempts,
		"total_cleanup_time_ms":          m.TotalCleanupTime.Milliseconds(),
		"average_cleanup_time_ms":        m.AverageCleanupTime.Milliseconds(),
		"min_cleanup_time_ms":            m.MinCleanupTime.Milliseconds(),
		"max_cleanup_time_ms":            m.MaxCleanupTime.Milliseconds(),
		"total_batches_processed":        m.TotalBatchesProcessed,
		"total_batch_items":              m.TotalBatchItems,
		"average_batch_size":             m.AverageBatchSize,
		"cleanup_operations_per_second":  opsPerSecond,
		"last_cleanup_timestamp":         m.LastCleanupTimestamp,
		"last_error":                     m.LastError,
		"last_error_timestamp":           m.LastErrorTimestamp,
		"error_count_by_type":            m.ErrorCountByType,
	}
}

// Reset resets all metrics to zero
func (m *InvalidMSISDNMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalInvalidMSISDNsDetected = 0
	m.TotalLogsCreated = 0
	m.TotalSubscriptionsCleaned = 0
	m.TotalCleanupFailures = 0
	m.TotalRetryAttempts = 0
	m.TotalCleanupTime = 0
	m.AverageCleanupTime = 0
	m.MinCleanupTime = time.Hour
	m.MaxCleanupTime = 0
	m.TotalBatchesProcessed = 0
	m.TotalBatchItems = 0
	m.AverageBatchSize = 0
	m.CleanupOperationsPerSecond = 0
	m.LastCleanupTimestamp = time.Time{}
	m.LastError = ""
	m.LastErrorTimestamp = time.Time{}
	m.ErrorCountByType = make(map[string]int64)

	m.logger.Info("INVALID_MSISDN metrics reset")
}

// LogSummary logs a summary of current metrics
func (m *InvalidMSISDNMetrics) LogSummary() {
	metrics := m.GetMetricsSnapshot()

	m.logger.Info("INVALID_MSISDN Metrics Summary",
		zap.Int64("totalDetected", metrics["total_invalid_msisdns_detected"].(int64)),
		zap.Int64("totalCleaned", metrics["total_subscriptions_cleaned"].(int64)),
		zap.Int64("totalFailures", metrics["total_cleanup_failures"].(int64)),
		zap.Int64("totalRetries", metrics["total_retry_attempts"].(int64)),
		zap.Int64("totalBatches", metrics["total_batches_processed"].(int64)),
		zap.Float64("avgCleanupTimeMs", metrics["average_cleanup_time_ms"].(float64)),
		zap.Float64("opsPerSecond", metrics["cleanup_operations_per_second"].(float64)))
}

// GetSuccessRate returns the success rate of cleanup operations
func (m *InvalidMSISDNMetrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.TotalSubscriptionsCleaned + m.TotalCleanupFailures
	if total == 0 {
		return 100.0
	}

	successRate := (float64(m.TotalSubscriptionsCleaned) / float64(total)) * 100.0
	return successRate
}

// GetAverageCleanupTime returns the average cleanup time
func (m *InvalidMSISDNMetrics) GetAverageCleanupTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.AverageCleanupTime
}

// GetTotalOperations returns the total number of operations
func (m *InvalidMSISDNMetrics) GetTotalOperations() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.TotalSubscriptionsCleaned + m.TotalCleanupFailures
}
