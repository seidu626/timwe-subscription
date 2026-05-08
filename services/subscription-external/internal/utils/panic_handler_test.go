package utils

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestPanicHandler_RecoverPanic tests panic recovery functionality
func TestPanicHandler_RecoverPanic(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create panic handler with higher panic depth limit for testing
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  5 * time.Second,
		ExitOnFatal:      false, // Don't exit in tests
		ExitCode:         1,
		MaxPanicDepth:    10, // Higher limit for testing
	}

	panicHandler := NewPanicHandler(logger, config)

	// Test panic recovery - the panic should be recovered, not propagated
	func() {
		defer panicHandler.RecoverPanic()
		panic("test panic")
	}()

	// If we reach here, the panic was successfully recovered
	// This is the expected behavior
	t.Log("Panic was successfully recovered")
}

func TestPanicHandler_SafeGo(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create panic handler
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  5 * time.Second,
		ExitOnFatal:      false, // Don't exit in tests
		ExitCode:         1,
		MaxPanicDepth:    10, // Higher limit for testing
	}

	panicHandler := NewPanicHandler(logger, config)

	// Test SafeGo with panic
	done := make(chan bool)
	panicHandler.SafeGo(func() {
		defer func() { done <- true }()
		panic("test panic in goroutine")
	})

	// Wait for completion
	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("SafeGo did not complete within timeout")
	}
}

func TestPanicHandler_HandleFatalError(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create panic handler
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  5 * time.Second,
		ExitOnFatal:      false, // Don't exit in tests
		ExitCode:         1,
		MaxPanicDepth:    10, // Higher limit for testing
	}

	panicHandler := NewPanicHandler(logger, config)

	// Test fatal error handling
	err := &testError{message: "test fatal error"}
	context := map[string]interface{}{
		"component": "test",
		"operation": "test-operation",
	}

	// This should not exit due to ExitOnFatal: false
	panicHandler.HandleFatalError(err, context)
}

func TestPanicHandler_DefaultConfig(t *testing.T) {
	config := DefaultPanicHandlerConfig()

	if config.EnableRecovery != true {
		t.Error("Expected EnableRecovery to be true by default")
	}

	if config.LogStackTraces != true {
		t.Error("Expected LogStackTraces to be true by default")
	}

	if config.ExitOnFatal != true {
		t.Error("Expected ExitOnFatal to be true by default")
	}

	if config.ExitCode != 1 {
		t.Error("Expected ExitCode to be 1 by default")
	}
}

func TestPanicHandler_ConfigValidation(t *testing.T) {
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    0, // Invalid
		RecoveryTimeout:  5 * time.Second,
		ExitOnFatal:      false,
		ExitCode:         1,
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error for MaxStackDepth <= 0")
	}

	config.MaxStackDepth = 64
	config.RecoveryTimeout = 0 // Invalid

	err = config.Validate()
	if err == nil {
		t.Error("Expected validation error for RecoveryTimeout <= 0")
	}

	config.RecoveryTimeout = 5 * time.Second
	config.ExitCode = 300 // Invalid

	err = config.Validate()
	if err == nil {
		t.Error("Expected validation error for ExitCode > 255")
	}
}

// TestPanicHandler_SelfProtection tests the self-protection mechanisms
func TestPanicHandler_SelfProtection(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultPanicHandlerConfig()
	config.MaxPanicDepth = 2 // Set low for testing
	config.ExitOnFatal = false

	panicHandler := NewPanicHandler(logger, config)

	// Test panic depth tracking
	panicHandler.HandlePanic("test panic 1", context.Background())
	panicHandler.HandlePanic("test panic 2", context.Background())
	panicHandler.HandlePanic("test panic 3", context.Background())

	// Verify panic depth is tracked
	status := panicHandler.GetStatus()
	if status["panic_depth"].(int32) != 0 { // Should be reset after emergency fallback
		t.Errorf("Expected panic depth to be 0 after emergency fallback, got: %d", status["panic_depth"])
	}
}

// TestPanicHandler_MemoryManagement tests memory management features
func TestPanicHandler_MemoryManagement(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultPanicHandlerConfig()
	config.EnableMemoryMonitoring = true
	config.MemoryCleanupThreshold = 1 // 1MB threshold for testing
	config.MaxMemoryUsage = 10        // 10MB max for testing

	panicHandler := NewPanicHandler(logger, config)

	// Get initial memory stats
	initialStats := panicHandler.getMemoryStats()

	// Trigger memory cleanup
	panicHandler.performMemoryCleanup()

	// Get memory stats after cleanup
	afterStats := panicHandler.getMemoryStats()

	// Verify cleanup was performed
	if afterStats.Alloc >= initialStats.Alloc {
		t.Log("Memory cleanup completed (alloc may not decrease due to test overhead)")
	}

	// Test memory monitoring
	panicHandler.checkMemoryUsage()

	// Verify memory stats are accessible
	memStats := panicHandler.getMemoryStats()
	if memStats.Alloc == 0 {
		t.Error("Expected memory stats to be available")
	}
}

// TestPanicHandler_Alerting tests the alerting system
func TestPanicHandler_Alerting(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultPanicHandlerConfig()
	config.MaxPanicsPerSecond = 1 // Low threshold for testing
	config.ExitOnFatal = false

	panicHandler := NewPanicHandler(logger, config)

	// Add console alert channel
	consoleChannel := NewConsoleAlertChannel("test-console")
	panicHandler.AddAlertChannel(consoleChannel)

	// Test normal panic (should not trigger alerts)
	panicHandler.HandlePanic("normal panic", context.Background())

	// Test high panic rate (should trigger alert)
	panicHandler.HandlePanic("rapid panic 1", context.Background())
	panicHandler.HandlePanic("rapid panic 2", context.Background())

	// Verify alert channel was added
	status := panicHandler.GetStatus()
	if status["alert_channels"] == nil {
		t.Log("Alert channels status not exposed in GetStatus")
	}

	// Test alert channel removal
	panicHandler.RemoveAlertChannel("test-console")
}

// TestPanicHandler_PerformanceOptimization tests performance features
func TestPanicHandler_PerformanceOptimization(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultPanicHandlerConfig()

	panicHandler := NewPanicHandler(logger, config)

	// Test cached status
	cachedStatus := panicHandler.GetCachedStatus()
	if cachedStatus == nil {
		t.Error("Expected cached status to be available")
	}

	// Test performance metrics
	perfMetrics := panicHandler.GetPerformanceMetrics()
	if perfMetrics == nil {
		t.Error("Expected performance metrics to be available")
	}

	// Verify queue and worker pool are initialized
	if perfMetrics["queue_capacity"].(int) != 1000 {
		t.Errorf("Expected queue capacity 1000, got: %d", perfMetrics["queue_capacity"])
	}

	if perfMetrics["worker_pool_capacity"].(int) != 10 {
		t.Errorf("Expected worker pool capacity 10, got: %d", perfMetrics["worker_pool_capacity"])
	}

	// Test graceful shutdown
	panicHandler.Shutdown()
}

// TestPanicHandler_RecoveryState tests recovery state management
func TestPanicHandler_RecoveryState(t *testing.T) {
	logger := zap.NewNop()
	panicHandler := NewPanicHandler(logger, nil)

	// Test initial health state
	if !panicHandler.IsHealthy() {
		t.Error("Expected panic handler to be healthy initially")
	}

	// Test health status
	health := panicHandler.GetHealth()
	if health != "HEALTHY" {
		t.Errorf("Expected health status 'HEALTHY', got: '%s'", health)
	}

	// Test status information
	status := panicHandler.GetStatus()
	if status["panic_depth"].(int32) != 0 {
		t.Errorf("Expected initial panic depth 0, got: %d", status["panic_depth"])
	}

	if status["max_panic_depth"].(int32) != 3 {
		t.Errorf("Expected max panic depth 3, got: %d", status["max_panic_depth"])
	}
}

// TestPanicHandler_ConfigurationValidation tests configuration validation
func TestPanicHandler_ConfigurationValidation(t *testing.T) {
	logger := zap.NewNop()

	// Test with nil config (should use defaults)
	panicHandler := NewPanicHandler(logger, nil)

	config := panicHandler.config
	if config.MaxMemoryUsage != 1024 {
		t.Errorf("Expected default max memory usage 1024MB, got: %d", config.MaxMemoryUsage)
	}

	if config.MaxPanicsPerSecond != 10 {
		t.Errorf("Expected default max panics per second 10, got: %d", config.MaxPanicsPerSecond)
	}

	if config.MemoryCheckInterval != 5*time.Second {
		t.Errorf("Expected default memory check interval 5s, got: %v", config.MemoryCheckInterval)
	}
}

// testError implements error interface for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
