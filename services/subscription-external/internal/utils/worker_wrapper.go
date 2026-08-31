package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WorkerWrapper provides panic recovery and monitoring for worker functions
type WorkerWrapper struct {
	panicHandler *PanicHandler
	logger       *zap.Logger
	workerName   string
	metrics      *WorkerMetrics

	// mu guards all access to metrics so concurrent workers (and readers)
	// never race on the shared counters (SafeGo runs writers concurrently).
	mu sync.Mutex
}

// WorkerMetrics tracks worker performance and health
type WorkerMetrics struct {
	TotalExecutions      int64
	SuccessfulExecutions int64
	FailedExecutions     int64
	PanicCount           int64
	TotalExecutionTime   time.Duration
	LastExecutionTime    time.Time
	LastError            error
	LastPanic            interface{}
}

// NewWorkerWrapper creates a new worker wrapper
func NewWorkerWrapper(panicHandler *PanicHandler, logger *zap.Logger, workerName string) *WorkerWrapper {
	return &WorkerWrapper{
		panicHandler: panicHandler,
		logger:       logger,
		workerName:   workerName,
		metrics:      &WorkerMetrics{},
	}
}

// WrapWorker wraps a worker function with panic recovery and metrics
func (ww *WorkerWrapper) WrapWorker(worker func() error) func() error {
	return func() error {
		start := time.Now()
		ww.mu.Lock()
		ww.metrics.TotalExecutions++
		ww.mu.Unlock()

		defer func() {
			if r := recover(); r != nil {
				ww.mu.Lock()
				ww.metrics.PanicCount++
				ww.metrics.FailedExecutions++
				ww.metrics.LastPanic = r
				ww.metrics.LastExecutionTime = time.Now()
				total, panics := ww.metrics.TotalExecutions, ww.metrics.PanicCount
				ww.mu.Unlock()

				ww.logger.Error("WORKER PANIC",
					zap.String("worker_name", ww.workerName),
					zap.Any("panic_value", r),
					zap.String("panic_type", fmt.Sprintf("%T", r)),
					zap.Duration("execution_time", time.Since(start)),
					zap.Int64("total_executions", total),
					zap.Int64("panic_count", panics),
				)

				// Use panic handler if available
				if ww.panicHandler != nil {
					ww.panicHandler.HandlePanic(r, nil)
				}
			}
		}()

		// Execute the worker function
		err := worker()
		executionTime := time.Since(start)

		// Update metrics
		ww.mu.Lock()
		ww.metrics.TotalExecutionTime += executionTime
		ww.metrics.LastExecutionTime = time.Now()

		if err != nil {
			ww.metrics.FailedExecutions++
			ww.metrics.LastError = err
			total, failed := ww.metrics.TotalExecutions, ww.metrics.FailedExecutions
			ww.mu.Unlock()

			ww.logger.Error("WORKER FAILED",
				zap.String("worker_name", ww.workerName),
				zap.Error(err),
				zap.Duration("execution_time", executionTime),
				zap.Int64("total_executions", total),
				zap.Int64("failed_executions", failed),
			)
		} else {
			ww.metrics.SuccessfulExecutions++
			total, succeeded := ww.metrics.TotalExecutions, ww.metrics.SuccessfulExecutions
			ww.mu.Unlock()

			ww.logger.Debug("WORKER SUCCESS",
				zap.String("worker_name", ww.workerName),
				zap.Duration("execution_time", executionTime),
				zap.Int64("total_executions", total),
				zap.Int64("successful_executions", succeeded),
			)
		}

		return err
	}
}

// WrapWorkerWithContext wraps a worker function with context and panic recovery
func (ww *WorkerWrapper) WrapWorkerWithContext(worker func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		start := time.Now()
		ww.mu.Lock()
		ww.metrics.TotalExecutions++
		ww.mu.Unlock()

		defer func() {
			if r := recover(); r != nil {
				ww.mu.Lock()
				ww.metrics.PanicCount++
				ww.metrics.FailedExecutions++
				ww.metrics.LastPanic = r
				ww.metrics.LastExecutionTime = time.Now()
				total, panics := ww.metrics.TotalExecutions, ww.metrics.PanicCount
				ww.mu.Unlock()

				ww.logger.Error("WORKER PANIC WITH CONTEXT",
					zap.String("worker_name", ww.workerName),
					zap.Any("panic_value", r),
					zap.String("panic_type", fmt.Sprintf("%T", r)),
					zap.Duration("execution_time", time.Since(start)),
					zap.Int64("total_executions", total),
					zap.Int64("panic_count", panics),
				)

				// Use panic handler if available
				if ww.panicHandler != nil {
					ww.panicHandler.HandlePanic(r, ctx)
				}
			}
		}()

		// Execute the worker function
		err := worker(ctx)
		executionTime := time.Since(start)

		// Update metrics
		ww.mu.Lock()
		ww.metrics.TotalExecutionTime += executionTime
		ww.metrics.LastExecutionTime = time.Now()

		if err != nil {
			ww.metrics.FailedExecutions++
			ww.metrics.LastError = err
			total, failed := ww.metrics.TotalExecutions, ww.metrics.FailedExecutions
			ww.mu.Unlock()

			ww.logger.Error("WORKER FAILED WITH CONTEXT",
				zap.String("worker_name", ww.workerName),
				zap.Error(err),
				zap.Duration("execution_time", executionTime),
				zap.Int64("total_executions", total),
				zap.Int64("failed_executions", failed),
			)
		} else {
			ww.metrics.SuccessfulExecutions++
			total, succeeded := ww.metrics.TotalExecutions, ww.metrics.SuccessfulExecutions
			ww.mu.Unlock()

			ww.logger.Debug("WORKER SUCCESS WITH CONTEXT",
				zap.String("worker_name", ww.workerName),
				zap.Duration("execution_time", executionTime),
				zap.Int64("total_executions", total),
				zap.Int64("successful_executions", succeeded),
			)
		}

		return err
	}
}

// SafeGo runs a worker function in a goroutine with panic recovery
func (ww *WorkerWrapper) SafeGo(worker func() error) {
	go func() {
		wrappedWorker := ww.WrapWorker(worker)
		if err := wrappedWorker(); err != nil {
			ww.logger.Error("Worker goroutine failed",
				zap.String("worker_name", ww.workerName),
				zap.Error(err),
			)
		}
	}()
}

// SafeGoWithContext runs a worker function in a goroutine with context and panic recovery
func (ww *WorkerWrapper) SafeGoWithContext(ctx context.Context, worker func(context.Context) error) {
	go func() {
		wrappedWorker := ww.WrapWorkerWithContext(worker)
		if err := wrappedWorker(ctx); err != nil {
			ww.logger.Error("Worker goroutine failed with context",
				zap.String("worker_name", ww.workerName),
				zap.Error(err),
			)
		}
	}()
}

// GetMetrics returns a copy of the worker metrics
func (ww *WorkerWrapper) GetMetrics() WorkerMetrics {
	ww.mu.Lock()
	defer ww.mu.Unlock()
	return *ww.metrics
}

// GetSuccessRate returns the success rate as a percentage
func (ww *WorkerWrapper) GetSuccessRate() float64 {
	ww.mu.Lock()
	defer ww.mu.Unlock()
	if ww.metrics.TotalExecutions == 0 {
		return 0.0
	}
	return float64(ww.metrics.SuccessfulExecutions) / float64(ww.metrics.TotalExecutions) * 100.0
}

// GetPanicRate returns the panic rate as a percentage
func (ww *WorkerWrapper) GetPanicRate() float64 {
	ww.mu.Lock()
	defer ww.mu.Unlock()
	if ww.metrics.TotalExecutions == 0 {
		return 0.0
	}
	return float64(ww.metrics.PanicCount) / float64(ww.metrics.TotalExecutions) * 100.0
}

// GetAverageExecutionTime returns the average execution time
func (ww *WorkerWrapper) GetAverageExecutionTime() time.Duration {
	ww.mu.Lock()
	defer ww.mu.Unlock()
	if ww.metrics.TotalExecutions == 0 {
		return 0
	}
	return ww.metrics.TotalExecutionTime / time.Duration(ww.metrics.TotalExecutions)
}

// ResetMetrics resets all metrics to zero
func (ww *WorkerWrapper) ResetMetrics() {
	ww.mu.Lock()
	ww.metrics = &WorkerMetrics{}
	ww.mu.Unlock()
	ww.logger.Info("Worker metrics reset",
		zap.String("worker_name", ww.workerName),
	)
}

// GetWorkerName returns the worker name
func (ww *WorkerWrapper) GetWorkerName() string {
	return ww.workerName
}

// GetPanicHandler returns the underlying panic handler
func (ww *WorkerWrapper) GetPanicHandler() *PanicHandler {
	return ww.panicHandler
}

// LogHealthStatus logs the current health status of the worker
func (ww *WorkerWrapper) LogHealthStatus() {
	ww.mu.Lock()
	total := ww.metrics.TotalExecutions
	succeeded := ww.metrics.SuccessfulExecutions
	failed := ww.metrics.FailedExecutions
	panics := ww.metrics.PanicCount
	totalTime := ww.metrics.TotalExecutionTime
	lastTime := ww.metrics.LastExecutionTime
	ww.mu.Unlock()

	successRate := 0.0
	panicRate := 0.0
	avgExecutionTime := time.Duration(0)
	if total > 0 {
		successRate = float64(succeeded) / float64(total) * 100.0
		panicRate = float64(panics) / float64(total) * 100.0
		avgExecutionTime = totalTime / time.Duration(total)
	}

	ww.logger.Info("WORKER HEALTH STATUS",
		zap.String("worker_name", ww.workerName),
		zap.Int64("total_executions", total),
		zap.Int64("successful_executions", succeeded),
		zap.Int64("failed_executions", failed),
		zap.Int64("panic_count", panics),
		zap.Float64("success_rate_percent", successRate),
		zap.Float64("panic_rate_percent", panicRate),
		zap.Duration("average_execution_time", avgExecutionTime),
		zap.Duration("total_execution_time", totalTime),
		zap.Time("last_execution_time", lastTime),
	)

	// Log warning if panic rate is high
	if panicRate > 10.0 {
		ww.logger.Warn("Worker has high panic rate",
			zap.String("worker_name", ww.workerName),
			zap.Float64("panic_rate_percent", panicRate),
			zap.Int64("panic_count", panics),
		)
	}

	// Log warning if success rate is low
	if successRate < 80.0 {
		ww.logger.Warn("Worker has low success rate",
			zap.String("worker_name", ww.workerName),
			zap.Float64("success_rate_percent", successRate),
			zap.Int64("failed_executions", failed),
		)
	}
}
