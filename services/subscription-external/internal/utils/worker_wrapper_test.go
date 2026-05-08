package utils

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWorkerWrapper_WrapWorker(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create panic handler
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  5 * time.Second,
		MaxPanicDepth:    3,
		ExitOnFatal:      false, // Don't exit in tests
		ExitCode:         1,
	}

	panicHandler := NewPanicHandler(logger, config)

	// Create worker wrapper
	workerWrapper := NewWorkerWrapper(panicHandler, logger, "test-worker")

	// Test successful worker execution
	successWorker := func() error {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	}

	wrappedWorker := workerWrapper.WrapWorker(successWorker)
	err := wrappedWorker()

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	metrics := workerWrapper.GetMetrics()
	if metrics.TotalExecutions != 1 {
		t.Errorf("Expected 1 execution, got: %d", metrics.TotalExecutions)
	}

	if metrics.SuccessfulExecutions != 1 {
		t.Errorf("Expected 1 successful execution, got: %d", metrics.SuccessfulExecutions)
	}

	// Test worker with error
	errorWorker := func() error {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return errors.New("test error")
	}

	wrappedErrorWorker := workerWrapper.WrapWorker(errorWorker)
	err = wrappedErrorWorker()

	if err == nil {
		t.Error("Expected error, got nil")
	}

	metrics = workerWrapper.GetMetrics()
	if metrics.TotalExecutions != 2 {
		t.Errorf("Expected 2 executions, got: %d", metrics.TotalExecutions)
	}

	if metrics.FailedExecutions != 1 {
		t.Errorf("Expected 1 failed execution, got: %d", metrics.FailedExecutions)
	}
}

func TestWorkerWrapper_WrapWorkerWithPanic(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create panic handler
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  5 * time.Second,
		MaxPanicDepth:    3,
		ExitOnFatal:      false, // Don't exit in tests
		ExitCode:         1,
	}

	panicHandler := NewPanicHandler(logger, config)

	// Create worker wrapper
	workerWrapper := NewWorkerWrapper(panicHandler, logger, "panic-test-worker")

	// Test worker that panics
	panicWorker := func() error {
		time.Sleep(10 * time.Millisecond) // Simulate work
		panic("test panic in worker")
	}

	wrappedWorker := workerWrapper.WrapWorker(panicWorker)

	// The panic should be recovered and logged
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panic should have been recovered by worker wrapper: %v", r)
			}
		}()

		err := wrappedWorker()
		if err != nil {
			t.Logf("Worker returned error as expected: %v", err)
		}
	}()

	metrics := workerWrapper.GetMetrics()
	if metrics.TotalExecutions != 1 {
		t.Errorf("Expected 1 execution, got: %d", metrics.TotalExecutions)
	}

	if metrics.PanicCount != 1 {
		t.Errorf("Expected 1 panic, got: %d", metrics.PanicCount)
	}

	if metrics.FailedExecutions != 1 {
		t.Errorf("Expected 1 failed execution, got: %d", metrics.FailedExecutions)
	}
}

func TestWorkerWrapper_SafeGo(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create panic handler
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  5 * time.Second,
		MaxPanicDepth:    3,
		ExitOnFatal:      false, // Don't exit in tests
		ExitCode:         1,
	}

	panicHandler := NewPanicHandler(logger, config)

	// Create worker wrapper
	workerWrapper := NewWorkerWrapper(panicHandler, logger, "safe-go-test-worker")

	// Test SafeGo with successful worker
	done := make(chan bool)
	workerWrapper.SafeGo(func() error {
		defer func() { done <- true }()
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	})

	// Wait for completion
	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("SafeGo did not complete within timeout")
	}

	// Test SafeGo with panicking worker
	panicDone := make(chan bool)
	workerWrapper.SafeGo(func() error {
		defer func() { panicDone <- true }()
		time.Sleep(10 * time.Millisecond) // Simulate work
		panic("test panic in SafeGo")
	})

	// Wait for completion
	select {
	case <-panicDone:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("SafeGo with panic did not complete within timeout")
	}

	metrics := workerWrapper.GetMetrics()
	if metrics.TotalExecutions != 2 {
		t.Errorf("Expected 2 executions, got: %d", metrics.TotalExecutions)
	}

	if metrics.PanicCount != 1 {
		t.Errorf("Expected 1 panic, got: %d", metrics.PanicCount)
	}
}

func TestWorkerWrapper_Metrics(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create panic handler
	config := &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  5 * time.Second,
		MaxPanicDepth:    3,
		ExitOnFatal:      false, // Don't exit in tests
		ExitCode:         1,
	}

	panicHandler := NewPanicHandler(logger, config)

	// Create worker wrapper
	workerWrapper := NewWorkerWrapper(panicHandler, logger, "metrics-test-worker")

	// Execute some workers to generate metrics
	successWorker := func() error {
		time.Sleep(20 * time.Millisecond) // Simulate work
		return nil
	}

	errorWorker := func() error {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return errors.New("test error")
	}

	panicWorker := func() error {
		time.Sleep(15 * time.Millisecond) // Simulate work
		panic("test panic")
	}

	// Execute workers
	wrappedSuccessWorker := workerWrapper.WrapWorker(successWorker)
	wrappedErrorWorker := workerWrapper.WrapWorker(errorWorker)
	wrappedPanicWorker := workerWrapper.WrapWorker(panicWorker)

	// Run success worker
	_ = wrappedSuccessWorker()

	// Run error worker
	_ = wrappedErrorWorker()

	// Run panic worker (should be recovered)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Panic recovered as expected: %v", r)
			}
		}()
		_ = wrappedPanicWorker()
	}()

	// Check metrics
	metrics := workerWrapper.GetMetrics()
	if metrics.TotalExecutions != 3 {
		t.Errorf("Expected 3 executions, got: %d", metrics.TotalExecutions)
	}

	if metrics.SuccessfulExecutions != 1 {
		t.Errorf("Expected 1 successful execution, got: %d", metrics.SuccessfulExecutions)
	}

	if metrics.FailedExecutions != 2 {
		t.Errorf("Expected 2 failed executions, got: %d", metrics.FailedExecutions)
	}

	if metrics.PanicCount != 1 {
		t.Errorf("Expected 1 panic, got: %d", metrics.PanicCount)
	}

	// Check calculated metrics
	successRate := workerWrapper.GetSuccessRate()
	expectedSuccessRate := 1.0 / 3.0 * 100.0 // Convert to percentage
	if abs(successRate-expectedSuccessRate) > 0.001 {
		t.Errorf("Expected success rate ~%.3f%%, got: %.3f%%", expectedSuccessRate, successRate)
	}

	panicRate := workerWrapper.GetPanicRate()
	expectedPanicRate := 1.0 / 3.0 * 100.0 // Convert to percentage
	if abs(panicRate-expectedPanicRate) > 0.001 {
		t.Errorf("Expected panic rate ~%.3f%%, got: %.3f%%", expectedPanicRate, panicRate)
	}

	avgTime := workerWrapper.GetAverageExecutionTime()
	if avgTime <= 0 {
		t.Errorf("Expected positive average execution time, got: %v", avgTime)
	}

	// Test health status logging
	workerWrapper.LogHealthStatus()
}

func TestWorkerWrapper_ResetMetrics(t *testing.T) {
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
	}

	panicHandler := NewPanicHandler(logger, config)

	// Create worker wrapper
	workerWrapper := NewWorkerWrapper(panicHandler, logger, "reset-metrics-test-worker")

	// Execute a worker to generate metrics
	worker := func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	wrappedWorker := workerWrapper.WrapWorker(worker)
	_ = wrappedWorker()

	// Verify metrics were generated
	metrics := workerWrapper.GetMetrics()
	if metrics.TotalExecutions != 1 {
		t.Errorf("Expected 1 execution before reset, got: %d", metrics.TotalExecutions)
	}

	// Reset metrics
	workerWrapper.ResetMetrics()

	// Verify metrics were reset
	metrics = workerWrapper.GetMetrics()
	if metrics.TotalExecutions != 0 {
		t.Errorf("Expected 0 executions after reset, got: %d", metrics.TotalExecutions)
	}

	if metrics.SuccessfulExecutions != 0 {
		t.Errorf("Expected 0 successful executions after reset, got: %d", metrics.SuccessfulExecutions)
	}

	if metrics.FailedExecutions != 0 {
		t.Errorf("Expected 0 failed executions after reset, got: %d", metrics.FailedExecutions)
	}

	if metrics.PanicCount != 0 {
		t.Errorf("Expected 0 panics after reset, got: %d", metrics.PanicCount)
	}
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
