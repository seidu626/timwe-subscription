package monitoring

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HistoricalMetrics represents historical metrics data
type HistoricalMetrics struct {
	ID                    int64                  `json:"id" db:"id"`
	Timestamp             time.Time              `json:"timestamp" db:"timestamp"`
	TotalSubscriptions    int64                  `json:"total_subscriptions" db:"total_subscriptions"`
	ChargingFailures      int64                  `json:"charging_failures" db:"charging_failures"`
	FailureRate           float64                `json:"failure_rate" db:"failure_rate"`
	SuccessRate           float64                `json:"success_rate" db:"success_rate"`
	NeverCharged          int64                  `json:"never_charged" db:"never_charged"`
	ChargingRecent        int64                  `json:"charging_recent" db:"charging_recent"`
	ChargingDelayed       int64                  `json:"charging_delayed" db:"charging_delayed"`
	ChargingStale         int64                  `json:"charging_stale" db:"charging_stale"`
	ProcessingStatus      string                 `json:"processing_status" db:"processing_status"`
	ProcessedToday        int64                  `json:"processed_today" db:"processed_today"`
	ProcessingQueue       int64                  `json:"processing_queue" db:"processing_queue"`
	AverageProcessingTime float64                `json:"average_processing_time" db:"average_processing_time"`
	Metadata              map[string]interface{} `json:"metadata" db:"metadata"`
}

// TrendAnalysis represents trend analysis results
type TrendAnalysis struct {
	Metric        string      `json:"metric"`
	Period        string      `json:"period"`
	StartTime     time.Time   `json:"start_time"`
	EndTime       time.Time   `json:"end_time"`
	CurrentValue  float64     `json:"current_value"`
	PreviousValue float64     `json:"previous_value"`
	Change        float64     `json:"change"`
	ChangePercent float64     `json:"change_percent"`
	Trend         string      `json:"trend"` // "increasing", "decreasing", "stable"
	DataPoints    []DataPoint `json:"data_points"`
}

// DataPoint represents a single data point in a trend
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// HistoricalDataManager manages historical metrics storage and analysis
type HistoricalDataManager struct {
	db        *sql.DB
	logger    *zap.Logger
	config    *HistoricalDataConfig
	stopChan  chan struct{}
	isRunning bool
	mu        sync.RWMutex
}

// HistoricalDataConfig holds configuration for historical data management
type HistoricalDataConfig struct {
	RetentionPeriod   time.Duration `json:"retention_period" yaml:"retention_period"`
	StorageInterval   time.Duration `json:"storage_interval" yaml:"storage_interval"`
	MaxDataPoints     int           `json:"max_data_points" yaml:"max_data_points"`
	EnableCompression bool          `json:"enable_compression" yaml:"enable_compression"`
	DatabaseTable     string        `json:"database_table" yaml:"database_table"`
}

// NewHistoricalDataManager creates a new historical data manager
func NewHistoricalDataManager(db *sql.DB, config *HistoricalDataConfig, logger *zap.Logger) *HistoricalDataManager {
	if config == nil {
		config = &HistoricalDataConfig{
			RetentionPeriod:   30 * 24 * time.Hour, // 30 days
			StorageInterval:   1 * time.Hour,       // Every hour
			MaxDataPoints:     10000,
			EnableCompression: false,
			DatabaseTable:     "historical_metrics",
		}
	}

	return &HistoricalDataManager{
		db:       db,
		config:   config,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Start begins historical data management
func (hdm *HistoricalDataManager) Start(ctx context.Context) error {
	if hdm.isRunning {
		return nil
	}

	hdm.isRunning = true
	hdm.logger.Info("Starting historical data manager")

	// Initialize database table
	if err := hdm.initializeTable(); err != nil {
		return fmt.Errorf("failed to initialize table: %w", err)
	}

	// Start data collection goroutine
	go hdm.dataCollectionLoop(ctx)

	// Start cleanup goroutine
	go hdm.cleanupLoop(ctx)

	hdm.logger.Info("Historical data manager started successfully")
	return nil
}

// Stop stops historical data management
func (hdm *HistoricalDataManager) Stop() {
	if !hdm.isRunning {
		return
	}

	hdm.logger.Info("Stopping historical data manager")
	close(hdm.stopChan)
	hdm.isRunning = false
}

// initializeTable creates the historical metrics table if it doesn't exist
func (hdm *HistoricalDataManager) initializeTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			timestamp TIMESTAMP NOT NULL,
			total_subscriptions BIGINT NOT NULL,
			charging_failures BIGINT NOT NULL,
			failure_rate DOUBLE PRECISION NOT NULL,
			success_rate DOUBLE PRECISION NOT NULL,
			never_charged BIGINT NOT NULL,
			charging_recent BIGINT NOT NULL,
			charging_delayed BIGINT NOT NULL,
			charging_stale BIGINT NOT NULL,
			processing_status VARCHAR(50) NOT NULL,
			processed_today BIGINT NOT NULL,
			processing_queue BIGINT NOT NULL,
			average_processing_time DOUBLE PRECISION NOT NULL,
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, hdm.config.DatabaseTable)

	_, err := hdm.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create index on timestamp for efficient queries
	indexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON %s (timestamp)
	`, hdm.config.DatabaseTable, hdm.config.DatabaseTable)

	_, err = hdm.db.Exec(indexQuery)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	hdm.logger.Info("Historical metrics table initialized",
		zap.String("table", hdm.config.DatabaseTable))
	return nil
}

// StoreMetrics stores current metrics as historical data
func (hdm *HistoricalDataManager) StoreMetrics(metrics *ChargingFailureMetrics) error {
	if !hdm.isRunning {
		return fmt.Errorf("historical data manager is not running")
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(metrics.Metadata)
	if err != nil {
		hdm.logger.Warn("Failed to marshal metadata", zap.Error(err))
		metadataJSON = []byte("{}")
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (
			timestamp, total_subscriptions, charging_failures, failure_rate, success_rate,
			never_charged, charging_recent, charging_delayed, charging_stale,
			processing_status, processed_today, processing_queue, average_processing_time, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, hdm.config.DatabaseTable)

	_, err = hdm.db.Exec(query,
		time.Now(),
		metrics.TotalSubscriptions,
		metrics.ChargingFailures,
		metrics.FailureRate,
		metrics.SuccessRate,
		metrics.NeverCharged,
		metrics.ChargingRecent,
		metrics.ChargingDelayed,
		metrics.ChargingStale,
		metrics.ProcessingStatus,
		metrics.ProcessedToday,
		metrics.ProcessingQueue,
		metrics.AverageProcessingTime,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to store historical metrics: %w", err)
	}

	hdm.logger.Debug("Stored historical metrics",
		zap.Time("timestamp", time.Now()),
		zap.Int64("total_subscriptions", metrics.TotalSubscriptions))
	return nil
}

// GetHistoricalMetrics retrieves historical metrics for a time range
func (hdm *HistoricalDataManager) GetHistoricalMetrics(startTime, endTime time.Time, limit int) ([]*HistoricalMetrics, error) {
	if limit <= 0 {
		limit = hdm.config.MaxDataPoints
	}

	query := fmt.Sprintf(`
		SELECT id, timestamp, total_subscriptions, charging_failures, failure_rate, success_rate,
		       never_charged, charging_recent, charging_delayed, charging_stale,
		       processing_status, processed_today, processing_queue, average_processing_time, metadata
		FROM %s
		WHERE timestamp BETWEEN $1 AND $2
		ORDER BY timestamp DESC
		LIMIT $3
	`, hdm.config.DatabaseTable)

	rows, err := hdm.db.Query(query, startTime, endTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query historical metrics: %w", err)
	}
	defer rows.Close()

	var metrics []*HistoricalMetrics
	for rows.Next() {
		var hm HistoricalMetrics
		var metadataJSON []byte

		err := rows.Scan(
			&hm.ID, &hm.Timestamp, &hm.TotalSubscriptions, &hm.ChargingFailures,
			&hm.FailureRate, &hm.SuccessRate, &hm.NeverCharged, &hm.ChargingRecent,
			&hm.ChargingDelayed, &hm.ChargingStale, &hm.ProcessingStatus,
			&hm.ProcessedToday, &hm.ProcessingQueue, &hm.AverageProcessingTime,
			&metadataJSON,
		)
		if err != nil {
			hdm.logger.Error("Failed to scan historical metrics row", zap.Error(err))
			continue
		}

		// Parse metadata JSON
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &hm.Metadata); err != nil {
				hdm.logger.Warn("Failed to unmarshal metadata", zap.Error(err))
				hm.Metadata = make(map[string]interface{})
			}
		}

		metrics = append(metrics, &hm)
	}

	return metrics, nil
}

// AnalyzeTrends performs trend analysis on historical data
func (hdm *HistoricalDataManager) AnalyzeTrends(metric string, period string) (*TrendAnalysis, error) {
	endTime := time.Now()
	var startTime time.Time

	switch period {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-24 * time.Hour) // Default to 24h
	}

	// Get historical data
	metrics, err := hdm.GetHistoricalMetrics(startTime, endTime, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical metrics: %w", err)
	}

	if len(metrics) < 2 {
		return nil, fmt.Errorf("insufficient data for trend analysis")
	}

	// Sort by timestamp (oldest first)
	// Note: In a real implementation, you'd want to sort properly
	// For now, we'll use the first and last metrics

	current := metrics[0]
	previous := metrics[len(metrics)-1]

	// Calculate trend
	var currentValue, previousValue float64
	switch metric {
	case "failure_rate":
		currentValue = current.FailureRate
		previousValue = previous.FailureRate
	case "success_rate":
		currentValue = current.SuccessRate
		previousValue = previous.SuccessRate
	case "total_subscriptions":
		currentValue = float64(current.TotalSubscriptions)
		previousValue = float64(previous.TotalSubscriptions)
	case "charging_failures":
		currentValue = float64(current.ChargingFailures)
		previousValue = float64(previous.ChargingFailures)
	default:
		return nil, fmt.Errorf("unsupported metric: %s", metric)
	}

	change := currentValue - previousValue
	var changePercent float64
	if previousValue != 0 {
		changePercent = (change / previousValue) * 100
	}

	// Determine trend direction
	trend := "stable"
	if change > 0.01 { // Small threshold to avoid noise
		trend = "increasing"
	} else if change < -0.01 {
		trend = "decreasing"
	}

	// Create data points for visualization
	var dataPoints []DataPoint
	for _, m := range metrics {
		var value float64
		switch metric {
		case "failure_rate":
			value = m.FailureRate
		case "success_rate":
			value = m.SuccessRate
		case "total_subscriptions":
			value = float64(m.TotalSubscriptions)
		case "charging_failures":
			value = float64(m.ChargingFailures)
		}

		dataPoints = append(dataPoints, DataPoint{
			Timestamp: m.Timestamp,
			Value:     value,
		})
	}

	return &TrendAnalysis{
		Metric:        metric,
		Period:        period,
		StartTime:     startTime,
		EndTime:       endTime,
		CurrentValue:  currentValue,
		PreviousValue: previousValue,
		Change:        change,
		ChangePercent: changePercent,
		Trend:         trend,
		DataPoints:    dataPoints,
	}, nil
}

// dataCollectionLoop periodically stores current metrics
func (hdm *HistoricalDataManager) dataCollectionLoop(ctx context.Context) {
	ticker := time.NewTicker(hdm.config.StorageInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hdm.stopChan:
			return
		case <-ticker.C:
			// This would typically be called from the main monitoring loop
			// For now, we'll just log that we're ready to collect data
			hdm.logger.Debug("Historical data collection cycle ready")
		}
	}
}

// cleanupLoop periodically removes old data based on retention policy
func (hdm *HistoricalDataManager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // Run cleanup daily
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hdm.stopChan:
			return
		case <-ticker.C:
			hdm.cleanupOldData()
		}
	}
}

// cleanupOldData removes data older than the retention period
func (hdm *HistoricalDataManager) cleanupOldData() {
	cutoffTime := time.Now().Add(-hdm.config.RetentionPeriod)

	query := fmt.Sprintf(`
		DELETE FROM %s WHERE timestamp < $1
	`, hdm.config.DatabaseTable)

	result, err := hdm.db.Exec(query, cutoffTime)
	if err != nil {
		hdm.logger.Error("Failed to cleanup old data", zap.Error(err))
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		hdm.logger.Info("Cleaned up old historical data",
			zap.Int64("rows_removed", rowsAffected),
			zap.Time("cutoff_time", cutoffTime))
	}
}

// GetMetricsSummary returns a summary of historical metrics
func (hdm *HistoricalDataManager) GetMetricsSummary(period string) (map[string]interface{}, error) {
	endTime := time.Now()
	var startTime time.Time

	switch period {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-24 * time.Hour)
	}

	query := fmt.Sprintf(`
		SELECT 
			AVG(failure_rate) as avg_failure_rate,
			AVG(success_rate) as avg_success_rate,
			MAX(total_subscriptions) as max_subscriptions,
			MIN(total_subscriptions) as min_subscriptions,
			AVG(charging_failures) as avg_charging_failures,
			COUNT(*) as data_points
		FROM %s
		WHERE timestamp BETWEEN $1 AND $2
	`, hdm.config.DatabaseTable)

	var avgFailureRate, avgSuccessRate, maxSubscriptions, minSubscriptions, avgChargingFailures sql.NullFloat64
	var dataPoints int

	err := hdm.db.QueryRow(query, startTime, endTime).Scan(
		&avgFailureRate, &avgSuccessRate, &maxSubscriptions, &minSubscriptions,
		&avgChargingFailures, &dataPoints,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics summary: %w", err)
	}

	summary := map[string]interface{}{
		"period":                period,
		"start_time":            startTime,
		"end_time":              endTime,
		"data_points":           dataPoints,
		"avg_failure_rate":      avgFailureRate.Float64,
		"avg_success_rate":      avgSuccessRate.Float64,
		"max_subscriptions":     maxSubscriptions.Float64,
		"min_subscriptions":     minSubscriptions.Float64,
		"avg_charging_failures": avgChargingFailures.Float64,
	}

	return summary, nil
}

// IsRunning returns whether the historical data manager is running
func (hdm *HistoricalDataManager) IsRunning() bool {
	return hdm.isRunning
}
