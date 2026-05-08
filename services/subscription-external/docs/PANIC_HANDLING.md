# Panic and Fatal Error Handling System

This document describes the comprehensive panic and fatal error handling system implemented in the TIMWE Subscription External Service.

## Overview

The panic handling system provides:
- **Automatic panic recovery** with comprehensive logging
- **Fatal error handling** with graceful shutdown
- **Worker goroutine protection** with metrics tracking
- **HTTP handler panic recovery** with proper error responses
- **Configurable behavior** for different environments
- **System health monitoring** during panic recovery

## Architecture

### Core Components

1. **PanicHandler** - Main panic handling utility
2. **PanicRecoveryMiddleware** - HTTP handler wrapper
3. **WorkerWrapper** - Worker function wrapper with metrics
4. **Configuration System** - Environment-based configuration

### Global Instance

The system provides a global panic handler instance that can be accessed throughout the application:

```go
// Initialize the global panic handler
utils.InitPanicHandler(logger)

// Access the global instance
panicHandler := utils.GetGlobalPanicHandler()

// Use global convenience functions
utils.RecoverPanic()
utils.SafeGo(func() { /* ... */ })
utils.HandleFatalError(err, context)
```

## Configuration

### Configuration File

The system loads configuration from `config/panic_handler.yaml`:

```yaml
# Production environment
production:
  enable_recovery: true
  log_stack_traces: true
  log_goroutine_info: true
  max_stack_depth: 64
  recovery_timeout: 30s
  exit_on_fatal: true
  exit_code: 1

# Development environment
development:
  enable_recovery: true
  log_stack_traces: true
  log_goroutine_info: true
  max_stack_depth: 64
  recovery_timeout: 30s
  exit_on_fatal: false  # Don't exit in development
  exit_code: 1
```

### Environment Variables

Configuration can be overridden with environment variables:

```bash
export PANIC_ENABLE_RECOVERY=true
export PANIC_LOG_STACK_TRACES=true
export PANIC_EXIT_ON_FATAL=true
export PANIC_RECOVERY_TIMEOUT=30s
export PANIC_EXIT_CODE=1
```

## Usage Examples

### 1. Basic Panic Recovery

```go
import "github.com/seidu626/subscription-manager/subscription-external/internal/utils"

func someFunction() {
    defer utils.RecoverPanic()
    
    // Your code here
    // If a panic occurs, it will be caught and logged
}
```

### 2. Context-Aware Panic Recovery

```go
func someFunctionWithContext(ctx context.Context) {
    defer utils.RecoverPanicWithContext(ctx)
    
    // Your code here
    // Panic will be logged with context information
}
```

### 3. Safe Goroutine Execution

```go
// Run function in goroutine with panic recovery
utils.SafeGo(func() {
    // Your code here
    // Panics are automatically recovered
})

// With context
utils.SafeGoWithContext(ctx, func(ctx context.Context) {
    // Your code here
})
```

### 4. Worker Function Wrapping

```go
import "github.com/seidu626/subscription-manager/subscription-external/internal/utils"

// Create a worker wrapper
workerWrapper := utils.NewWorkerWrapper(panicHandler, logger, "subscription-processor")

// Wrap your worker function
wrappedWorker := workerWrapper.WrapWorker(func() error {
    // Your worker logic here
    return nil
})

// Execute with panic recovery and metrics
err := wrappedWorker()

// Or run in goroutine
workerWrapper.SafeGo(func() error {
    // Your worker logic here
    return nil
})
```

### 5. HTTP Handler Panic Recovery

```go
import "github.com/seidu626/subscription-manager/subscription-external/internal/middleware"

// Create panic recovery middleware
panicMiddleware := middleware.NewPanicRecoveryMiddleware(logger, panicHandler)

// Wrap your FastHTTP handler
wrappedHandler := panicMiddleware.WrapFastHTTP(func(ctx *fasthttp.RequestCtx) {
    // Your handler logic here
    // Panics are automatically recovered and logged
})

// With metrics
wrappedHandlerWithMetrics := panicMiddleware.WrapFastHTTPWithMetrics(
    func(ctx *fasthttp.RequestCtx) {
        // Your handler logic here
    },
    "subscription-handler",
)
```

### 6. Fatal Error Handling

```go
// Handle fatal errors with context
utils.HandleFatalError(err, map[string]interface{}{
    "component": "database",
    "operation": "connection",
    "timestamp": time.Now(),
})

// Log errors with context
utils.LogError(err, map[string]interface{}{
    "component": "service",
    "operation": "subscription-processing",
})
```

## Panic Recovery Process

When a panic occurs, the system:

1. **Catches the panic** using `defer` and `recover()`
2. **Logs comprehensive information**:
   - Panic value and type
   - Stack trace
   - Caller information
   - Goroutine count
   - System memory stats
   - Context information (if available)

3. **Executes recovery logic**:
   - Forces garbage collection
   - Logs memory stats after recovery
   - Performs cleanup operations

4. **Handles termination**:
   - Exits process if configured (`exit_on_fatal: true`)
   - Continues execution if configured (`exit_on_fatal: false`)

## Worker Metrics

The WorkerWrapper provides comprehensive metrics:

```go
metrics := workerWrapper.GetMetrics()

// Success rate
successRate := workerWrapper.GetSuccessRate()

// Panic rate
panicRate := workerWrapper.GetPanicRate()

// Average execution time
avgTime := workerWrapper.GetAverageExecutionTime()

// Log health status
workerWrapper.LogHealthStatus()
```

## Best Practices

### 1. Always Use Panic Recovery

```go
// ❌ Don't do this
go func() {
    riskyFunction()
}()

// ✅ Do this instead
utils.SafeGo(func() error {
    return riskyFunction()
})
```

### 2. Provide Context for Errors

```go
// ❌ Don't do this
utils.HandleFatalError(err, nil)

// ✅ Do this instead
utils.HandleFatalError(err, map[string]interface{}{
    "component": "subscription-service",
    "operation": "process-optin",
    "msisdn": msisdn,
    "product_id": productID,
})
```

### 3. Use Worker Wrappers for Long-Running Functions

```go
// ❌ Don't do this
go func() {
    for {
        processSubscription()
        time.Sleep(time.Second)
    }
}()

// ✅ Do this instead
workerWrapper := utils.NewWorkerWrapper(panicHandler, logger, "subscription-processor")
workerWrapper.SafeGoWithContext(ctx, func(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := processSubscription(); err != nil {
                return err
            }
            time.Sleep(time.Second)
        }
    }
})
```

### 4. Configure Appropriately for Environment

```yaml
# Development - allow debugging
development:
  exit_on_fatal: false
  log_stack_traces: true

# Production - ensure stability
production:
  exit_on_fatal: true
  log_stack_traces: true
  recovery_timeout: 30s
```

## Monitoring and Alerting

### Health Checks

The system provides health status logging:

```go
// Log worker health status
workerWrapper.LogHealthStatus()

// Check for high panic rates
if workerWrapper.GetPanicRate() > 10.0 {
    // Alert operations team
    logger.Warn("High panic rate detected", 
        zap.Float64("panic_rate", workerWrapper.GetPanicRate()))
}
```

### Metrics Integration

Worker metrics can be integrated with monitoring systems:

```go
// Export metrics for Prometheus
panicCounter.WithLabelValues(workerName).Add(float64(metrics.PanicCount))
successCounter.WithLabelValues(workerName).Add(float64(metrics.SuccessfulExecutions))
executionTimeHistogram.WithLabelValues(workerName).Observe(avgTime.Seconds())
```

## Troubleshooting

### Common Issues

1. **Panic Handler Not Initialized**
   - Ensure `utils.InitPanicHandler(logger)` is called early in main()
   - Check configuration file exists and is valid

2. **High Panic Rates**
   - Review worker logic for potential race conditions
   - Check system resources (memory, CPU)
   - Verify database connections and timeouts

3. **Recovery Timeouts**
   - Increase `recovery_timeout` in configuration
   - Review recovery logic for blocking operations

### Debug Mode

Enable debug logging for troubleshooting:

```go
logger.SetLevel(zap.DebugLevel)

// Or via environment
export LOG_LEVEL=debug
```

## Integration with Existing Code

### Existing Handlers

Wrap existing HTTP handlers:

```go
// Before
router.HandleFunc("/subscriptions", subscriptionHandler)

// After
panicMiddleware := middleware.NewPanicRecoveryMiddleware(logger, panicHandler)
wrappedHandler := panicMiddleware.Wrap(subscriptionHandler)
router.HandleFunc("/subscriptions", wrappedHandler)
```

### Existing Workers

Wrap existing worker functions:

```go
// Before
go func() {
    for {
        processWork()
        time.Sleep(time.Second)
    }
}()

// After
workerWrapper := utils.NewWorkerWrapper(panicHandler, logger, "work-processor")
workerWrapper.SafeGo(func() error {
    for {
        if err := processWork(); err != nil {
            return err
        }
        time.Sleep(time.Second)
    }
})
```

## Performance Considerations

- **Minimal overhead** - Panic recovery only activates when needed
- **Efficient logging** - Stack traces are only captured when panics occur
- **Configurable depth** - Stack trace depth can be limited via configuration
- **Memory management** - Automatic garbage collection during recovery

## Security Considerations

- **Stack trace logging** - Ensure sensitive information is not logged
- **Error message sanitization** - Avoid logging user data in error messages
- **Exit code handling** - Use appropriate exit codes for different failure scenarios

## Future Enhancements

- **Remote panic reporting** - Send panic information to external monitoring systems
- **Automatic recovery strategies** - Implement intelligent recovery based on panic type
- **Performance profiling** - Add execution time profiling during panic recovery
- **Circuit breaker patterns** - Implement circuit breakers for frequently panicking components 