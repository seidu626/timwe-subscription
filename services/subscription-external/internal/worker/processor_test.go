package worker

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

// MockResubscriptionTracker implements ResubscriptionTracker for testing
type MockResubscriptionTracker struct {
	stats *CheckpointData
}

func NewMockResubscriptionTracker() *MockResubscriptionTracker {
	return &MockResubscriptionTracker{
		stats: &CheckpointData{
			BatchID:        "test_batch",
			Status:         "pending",
			TotalCount:     100,
			ProcessedCount: 0,
			SuccessCount:   0,
			FailureCount:   0,
		},
	}
}

func (m *MockResubscriptionTracker) InitializeBatch(totalCount int) error {
	m.stats.TotalCount = totalCount
	m.stats.Status = "in_progress"
	return nil
}

func (m *MockResubscriptionTracker) CheckIfProcessed(msisdn string, productID int) (bool, error) {
	return false, nil
}

func (m *MockResubscriptionTracker) RecordAttempt(msisdn string, productID int, subscriptionID int) error {
	return nil
}

func (m *MockResubscriptionTracker) UpdateResult(msisdn string, productID int, success bool, errorMessage string) error {
	if success {
		m.stats.SuccessCount++
	} else {
		m.stats.FailureCount++
	}
	m.stats.ProcessedCount++
	return nil
}

func (m *MockResubscriptionTracker) SaveCheckpoint(subscriptionID int, msisdn string) error {
	return nil
}

func (m *MockResubscriptionTracker) LoadCheckpoint() (*CheckpointData, error) {
	return m.stats, nil
}

func (m *MockResubscriptionTracker) MarkCompleted() error {
	m.stats.Status = "completed"
	return nil
}

func (m *MockResubscriptionTracker) GetStats() *CheckpointData {
	return m.stats
}

func (m *MockResubscriptionTracker) LogProgress() {
	// Mock logging
}

func (m *MockResubscriptionTracker) IncrementProcessed() {
	m.stats.ProcessedCount++
}

// TestProcessorCreation tests basic processor creation
func TestProcessorCreation(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := &ProcessingConfig{
		BatchSize:            100,
		MaxConcurrency:       5,
		RetryAttempts:        3,
		CheckpointInterval:   50,
		BatchID:              "test_batch",
		UpdateChargingHealth: true,
	}

	processor := NewResubscriptionProcessor(
		nil, // No repository
		nil, // No service
		nil, // No monitor
		logger,
		config,
	)

	if processor == nil {
		t.Fatal("Processor should not be nil")
	}

	if processor.GetStatus() != "stopped" {
		t.Errorf("Expected status 'stopped', got '%s'", processor.GetStatus())
	}

	// Test configuration
	retrievedConfig := processor.GetConfig()
	if retrievedConfig.BatchSize != 100 {
		t.Errorf("Expected batch size 100, got %d", retrievedConfig.BatchSize)
	}
}

// TestProcessorWithMockTracker tests processor with mock tracker
func TestProcessorWithMockTracker(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := &ProcessingConfig{
		BatchSize:            10,
		MaxConcurrency:       2,
		RetryAttempts:        1,
		CheckpointInterval:   5,
		BatchID:              "test_batch",
		UpdateChargingHealth: true,
	}

	processor := NewResubscriptionProcessor(
		nil, // No repository
		nil, // No service
		nil, // No monitor
		logger,
		config,
	)

	// Set mock tracker
	mockTracker := NewMockResubscriptionTracker()
	processor.SetTracker(mockTracker)

	// Test tracker integration
	if processor.tracker == nil {
		t.Fatal("Tracker should be set")
	}

	// Test configuration update
	newConfig := &ProcessingConfig{
		BatchSize:            20,
		MaxConcurrency:       3,
		RetryAttempts:        2,
		CheckpointInterval:   10,
		BatchID:              "updated_batch",
		UpdateChargingHealth: true,
	}

	if err := processor.UpdateConfig(newConfig); err != nil {
		t.Errorf("Failed to update config: %v", err)
	}

	updatedConfig := processor.GetConfig()
	if updatedConfig.BatchSize != 20 {
		t.Errorf("Expected updated batch size 20, got %d", updatedConfig.BatchSize)
	}
}

// TestProcessorStatistics tests statistics management
func TestProcessorStatistics(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	processor := NewResubscriptionProcessor(
		nil, nil, nil, logger, nil,
	)

	// Test initial stats
	stats := processor.GetStats()
	if stats.TotalProcessed != 0 {
		t.Errorf("Expected initial total processed 0, got %d", stats.TotalProcessed)
	}

	// Test reset stats
	processor.ResetStats()
	resetStats := processor.GetStats()
	if resetStats.TotalProcessed != 0 {
		t.Errorf("Expected reset total processed 0, got %d", resetStats.TotalProcessed)
	}

	// Test clear results
	processor.ClearResults()
	results := processor.GetResults("", 10)
	if len(results) != 0 {
		t.Errorf("Expected cleared results length 0, got %d", len(results))
	}
}

// TestProcessorExport tests result export functionality
func TestProcessorExport(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	processor := NewResubscriptionProcessor(
		nil, nil, nil, logger, nil,
	)

	// Test export with no results
	jsonData, err := processor.ExportResults("json", "")
	if err != nil {
		t.Errorf("Failed to export empty results to JSON: %v", err)
	}

	if string(jsonData) != "[]" {
		t.Errorf("Expected empty JSON array '[]', got '%s'", string(jsonData))
	}

	// Test unsupported format
	_, err = processor.ExportResults("xml", "")
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

// TestProcessorPauseResume tests pause and resume functionality
func TestProcessorPauseResume(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	processor := NewResubscriptionProcessor(
		nil, nil, nil, logger, nil,
	)

	// Test pause
	processor.Pause()
	if processor.GetStatus() != "stopped" {
		t.Errorf("Expected status 'stopped' after pause, got '%s'", processor.GetStatus())
	}

	// Test resume
	ctx := context.Background()
	if err := processor.Resume(ctx); err != nil {
		t.Errorf("Failed to resume: %v", err)
	}

	if processor.GetStatus() != "running" {
		t.Errorf("Expected status 'running' after resume, got '%s'", processor.GetStatus())
	}
}

// TestProcessorGracefulShutdown tests graceful shutdown
func TestProcessorGracefulShutdown(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	processor := NewResubscriptionProcessor(
		nil, nil, nil, logger, nil,
	)

	ctx := context.Background()

	// Test graceful shutdown
	if err := processor.GracefulShutdown(ctx, 1*time.Second); err != nil {
		t.Errorf("Graceful shutdown failed: %v", err)
	}

	if processor.GetStatus() != "stopped" {
		t.Errorf("Expected status 'stopped' after shutdown, got '%s'", processor.GetStatus())
	}
}

// TestProcessorTimeRangeFilter tests time range filtering
func TestProcessorTimeRangeFilter(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	processor := NewResubscriptionProcessor(
		nil, nil, nil, logger, nil,
	)

	// Test time range filtering with no results
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	results := processor.GetResultsByTimeRange(start, end, "")

	if len(results) != 0 {
		t.Errorf("Expected 0 results for time range, got %d", len(results))
	}
}

// BenchmarkProcessorCreation benchmarks processor creation
func BenchmarkProcessorCreation(b *testing.B) {
	logger, _ := zap.NewDevelopment()

	for i := 0; i < b.N; i++ {
		config := &ProcessingConfig{
			BatchSize:            100,
			MaxConcurrency:       5,
			RetryAttempts:        3,
			CheckpointInterval:   50,
			BatchID:              "benchmark_batch",
			UpdateChargingHealth: true,
		}

		_ = NewResubscriptionProcessor(
			nil, nil, nil, logger, config,
		)
	}
}

// BenchmarkConfigurationUpdate benchmarks configuration updates
func BenchmarkConfigurationUpdate(b *testing.B) {
	logger, _ := zap.NewDevelopment()

	processor := NewResubscriptionProcessor(
		nil, nil, nil, logger, nil,
	)

	newConfig := &ProcessingConfig{
		BatchSize:            200,
		MaxConcurrency:       10,
		RetryAttempts:        5,
		CheckpointInterval:   100,
		BatchID:              "benchmark_updated",
		UpdateChargingHealth: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.UpdateConfig(newConfig)
	}
}
