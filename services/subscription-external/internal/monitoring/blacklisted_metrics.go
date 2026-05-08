package monitoring

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// BlacklistedMetrics tracks metrics for BLACKLISTED user handling operations
type BlacklistedMetrics struct {
	mu                            sync.RWMutex
	TotalBlacklistedUsersDetected int64
	TotalUserbaseInsertions       int64
	TotalSubscriptionsCleaned     int64
	TotalOperationFailures        int64
	TotalRetryAttempts            int64
	TotalOperationTime            time.Duration
	AverageOperationTime          time.Duration
	MinOperationTime              time.Duration
	MaxOperationTime              time.Duration
	TotalBatchesProcessed         int64
	TotalBatchItems               int64
	AverageBatchSize              float64
	LastError                     string
	LastErrorTimestamp            time.Time
	ErrorCountByType              map[string]int64
	OperationsPerSecond           float64
	LastOperationTimestamp        time.Time
	UserbaseInsertionTime         time.Duration
	SubscriptionCleanupTime       time.Duration
	TotalAuditLogsCreated         int64
	logger                        *zap.Logger
}

// NewBlacklistedMetrics creates a new BlacklistedMetrics instance
func NewBlacklistedMetrics(logger *zap.Logger) *BlacklistedMetrics {
	return &BlacklistedMetrics{
		ErrorCountByType: make(map[string]int64),
		logger:           logger,
	}
}

// RecordBlacklistedUserDetected records when a BLACKLISTED user is detected
func (m *BlacklistedMetrics) RecordBlacklistedUserDetected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalBlacklistedUsersDetected++
}

// RecordUserbaseInsertion records when a user is successfully inserted into userbase
func (m *BlacklistedMetrics) RecordUserbaseInsertion(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalUserbaseInsertions++
	m.recordTiming(duration)
}

// RecordSubscriptionCleaned records when subscriptions are cleaned up for a blacklisted user
func (m *BlacklistedMetrics) RecordSubscriptionCleaned(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalSubscriptionsCleaned++
	m.recordTiming(duration)
}

// RecordOperationFailure records when an operation fails
func (m *BlacklistedMetrics) RecordOperationFailure(errorType string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalOperationFailures++
	m.LastError = errorType
	m.LastErrorTimestamp = time.Now()

	if m.ErrorCountByType == nil {
		m.ErrorCountByType = make(map[string]int64)
	}
	m.ErrorCountByType[errorType]++
}

// RecordRetryAttempt records when a retry attempt is made
func (m *BlacklistedMetrics) RecordRetryAttempt() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalRetryAttempts++
}

// RecordBatchProcessed records when a batch is processed
func (m *BlacklistedMetrics) RecordBatchProcessed(batchSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalBatchesProcessed++
	m.TotalBatchItems += int64(batchSize)
	m.AverageBatchSize = float64(m.TotalBatchItems) / float64(m.TotalBatchesProcessed)
}

// RecordAuditLogCreated records when an audit log is created
func (m *BlacklistedMetrics) RecordAuditLogCreated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalAuditLogsCreated++
}

// recordTiming records timing information for operations
func (m *BlacklistedMetrics) recordTiming(duration time.Duration) {
	m.TotalOperationTime += duration
	m.AverageOperationTime = m.TotalOperationTime / time.Duration(m.getTotalSuccessfulOperations())

	if m.MinOperationTime == 0 || duration < m.MinOperationTime {
		m.MinOperationTime = duration
	}
	if duration > m.MaxOperationTime {
		m.MaxOperationTime = duration
	}

	m.LastOperationTimestamp = time.Now()
}

// getTotalSuccessfulOperations returns the total number of successful operations
func (m *BlacklistedMetrics) getTotalSuccessfulOperations() int64 {
	return m.TotalUserbaseInsertions + m.TotalSubscriptionsCleaned
}

// GetMetricsSnapshot returns a snapshot of all metrics
func (m *BlacklistedMetrics) GetMetricsSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalOps := m.getTotalSuccessfulOperations()
	var opsPerSecond float64
	if m.TotalOperationTime > 0 {
		opsPerSecond = float64(totalOps) / m.TotalOperationTime.Seconds()
	}

	return map[string]interface{}{
		"total_blacklisted_users_detected": m.TotalBlacklistedUsersDetected,
		"total_userbase_insertions":        m.TotalUserbaseInsertions,
		"total_subscriptions_cleaned":      m.TotalSubscriptionsCleaned,
		"total_operation_failures":         m.TotalOperationFailures,
		"total_retry_attempts":             m.TotalRetryAttempts,
		"total_operation_time":             m.TotalOperationTime.String(),
		"average_operation_time":           m.AverageOperationTime.String(),
		"min_operation_time":               m.MinOperationTime.String(),
		"max_operation_time":               m.MaxOperationTime.String(),
		"total_batches_processed":          m.TotalBatchesProcessed,
		"total_batch_items":                m.TotalBatchItems,
		"average_batch_size":               m.AverageBatchSize,
		"last_error":                       m.LastError,
		"last_error_timestamp":             m.LastErrorTimestamp,
		"error_count_by_type":              m.ErrorCountByType,
		"operations_per_second":            opsPerSecond,
		"last_operation_timestamp":         m.LastOperationTimestamp,
		"total_audit_logs_created":         m.TotalAuditLogsCreated,
	}
}

// Reset resets all metrics to zero
func (m *BlacklistedMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalBlacklistedUsersDetected = 0
	m.TotalUserbaseInsertions = 0
	m.TotalSubscriptionsCleaned = 0
	m.TotalOperationFailures = 0
	m.TotalRetryAttempts = 0
	m.TotalOperationTime = 0
	m.AverageOperationTime = 0
	m.MinOperationTime = 0
	m.MaxOperationTime = 0
	m.TotalBatchesProcessed = 0
	m.TotalBatchItems = 0
	m.AverageBatchSize = 0
	m.LastError = ""
	m.LastErrorTimestamp = time.Time{}
	m.ErrorCountByType = make(map[string]int64)
	m.OperationsPerSecond = 0
	m.LastOperationTimestamp = time.Time{}
	m.TotalAuditLogsCreated = 0
}

// LogSummary logs a summary of current metrics
func (m *BlacklistedMetrics) LogSummary() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("Blacklisted Metrics Summary",
		zap.Int64("total_detected", m.TotalBlacklistedUsersDetected),
		zap.Int64("total_insertions", m.TotalUserbaseInsertions),
		zap.Int64("total_cleanups", m.TotalSubscriptionsCleaned),
		zap.Int64("total_failures", m.TotalOperationFailures),
		zap.Int64("total_retries", m.TotalRetryAttempts),
		zap.Duration("avg_operation_time", m.AverageOperationTime),
		zap.Int64("total_batches", m.TotalBatchesProcessed),
		zap.Float64("avg_batch_size", m.AverageBatchSize),
		zap.Int64("total_audit_logs", m.TotalAuditLogsCreated))
}

// GetSuccessRate returns the success rate as a percentage
func (m *BlacklistedMetrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalOps := m.getTotalSuccessfulOperations()
	totalAttempts := totalOps + m.TotalOperationFailures

	if totalAttempts == 0 {
		return 100.0
	}

	return (float64(totalOps) / float64(totalAttempts)) * 100.0
}

// GetAverageOperationTime returns the average operation time
func (m *BlacklistedMetrics) GetAverageOperationTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.AverageOperationTime
}

// GetTotalOperations returns the total number of operations
func (m *BlacklistedMetrics) GetTotalOperations() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getTotalSuccessfulOperations()
}
