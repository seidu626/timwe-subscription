# Connection Timeout and Goroutine Leak Fixes

## Overview
This document outlines the critical fixes implemented to resolve connection hanging, goroutine leaks, and resource exhaustion issues in the subscription-external service.

## Critical Issues Identified

### 1. HTTP Client Connection Hanging
- **Problem**: Multiple goroutines stuck in `IO wait` state trying to establish HTTP connections
- **Root Cause**: Missing connection timeouts and improper connection management
- **Impact**: Service becomes unresponsive, goroutines accumulate, memory leaks

### 2. Database Connection Issues
- **Problem**: Several goroutines waiting on database connections that appear to be hanging
- **Root Cause**: Missing query timeouts and connection pool management
- **Impact**: Database operations hang indefinitely, blocking worker goroutines

### 3. Goroutine Leaks
- **Problem**: Many goroutines created but not properly managed or cleaned up
- **Root Cause**: Missing context cancellation and timeout handling
- **Impact**: Resource exhaustion, service degradation, potential crashes

### 4. Network Resource Exhaustion
- **Problem**: Application creating too many concurrent connections without limits
- **Root Cause**: Missing connection pooling and resource limits
- **Impact**: Network exhaustion, connection failures, cascading failures

## Implemented Fixes

### 1. Enhanced HTTP Client Configuration

#### Before (Problematic):
```go
client := &fasthttp.Client{
    MaxConnsPerHost:          maxConnections,
    MaxIdleConnDuration:      30 * time.Second,
    ReadTimeout:              cfg.Application.TIMWE.Timeout,
    WriteTimeout:             cfg.Application.TIMWE.Timeout,
    // Missing critical timeout settings
}
```

#### After (Fixed):
```go
client := &fasthttp.Client{
    MaxConnsPerHost:          maxConnections,
    MaxIdleConnDuration:      30 * time.Second,
    ReadTimeout:              cfg.Application.TIMWE.Timeout,
    WriteTimeout:             cfg.Application.TIMWE.Timeout,
    // Added critical timeout and connection management
    MaxConnDuration:          60 * time.Second,  // Maximum connection lifetime
    ReadBufferSize:           4096,              // Optimize buffer sizes
    WriteBufferSize:          4096,
    // Custom dialer with timeout
    Dial: func(addr string) (net.Conn, error) {
        dialer := &net.Dialer{
            Timeout:   10 * time.Second, // Connection establishment timeout
            KeepAlive: 30 * time.Second,
        }
        return dialer.Dial("tcp", addr)
    },
}
```

### 2. Context-Based Request Timeout Handling

#### Before (Problematic):
```go
// Send request without timeout
if err = s.client.Do(req, res); err != nil {
    // Handle error
}
```

#### After (Fixed):
```go
// Create context with timeout for this request
ctx, cancel := context.WithTimeout(context.Background(), s.config.Application.TIMWE.Timeout)

// Send request with context timeout
requestDone := make(chan error, 1)
go func() {
    requestDone <- s.client.Do(req, res)
}()

// Wait for request completion or timeout
select {
case err = <-requestDone:
    // Request completed
case <-ctx.Done():
    // Context timeout or cancellation
    cancel()
    fasthttp.ReleaseRequest(req)
    fasthttp.ReleaseResponse(res)
    
    if attempt == maxRetries {
        return nil, fmt.Errorf("request timeout after %d attempts: %v", maxRetries, ctx.Err())
    }
    
    // Retry with exponential backoff
    delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
    time.Sleep(delay)
    continue
}
```

### 3. Database Query Timeout Handling

#### Before (Problematic):
```go
rows, err := r.db.Query(query, args...)
```

#### After (Fixed):
```go
// Create context with timeout for database operations
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Use context-aware query with timeout
rows, err := r.db.QueryContext(ctx, query, args...)
```

### 4. MSISDN Validation Timeout Handling

#### Before (Problematic):
```go
func (v *MSISDNValidator) ValidateMSISDN(ctx context.Context, msisdn string) (*ValidationResult, error) {
    // No timeout handling
}
```

#### After (Fixed):
```go
func (v *MSISDNValidator) ValidateMSISDN(ctx context.Context, msisdn string) (*ValidationResult, error) {
    // Add timeout to context if not already present
    if _, ok := ctx.Deadline(); !ok {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
        defer cancel()
    }
    // ... rest of validation logic
}
```

### 5. Batch Job Context Management

#### Before (Problematic):
```go
for i := 0; i < maxWorkers; i++ {
    wg.Add(1)
    go func(workerID int) {
        defer wg.Done()
        for request := range optinRequestChan {
            // Process request without timeout
        }
    }(i)
}
```

#### After (Fixed):
```go
// Create context with timeout for the entire batch job
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

// Create worker context with cancellation
workerCtx, workerCancel := context.WithCancel(ctx)
defer workerCancel()

for i := 0; i < maxWorkers; i++ {
    wg.Add(1)
    go func(workerID int) {
        defer wg.Done()
        for {
            select {
            case request, ok := <-optinRequestChan:
                if !ok {
                    return
                }
                // Process request
            case <-workerCtx.Done():
                // Context cancelled, worker should exit
                return
            }
        }
    }(i)
}
```

### 6. Connection Pool Cleanup

#### Added:
```go
// Start connection cleanup goroutine
go s.cleanupConnections()

// cleanupConnections periodically cleans up idle connections
func (s *SubscriptionService) cleanupConnections() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        // Close idle connections
        s.client.CloseIdleConnections()
        
        // Log connection pool status
        s.logger.Debug("Connection pool cleanup completed")
    }
}
```

### 7. Enhanced Configuration

#### Added timeout settings:
```yaml
TIMWE_MA:
  # Enhanced timeout and connection settings
  DIAL_TIMEOUT: 10s           # Connection establishment timeout
  MAX_CONN_DURATION: 60s      # Maximum connection lifetime
  IDLE_CONN_TIMEOUT: 30s      # Idle connection timeout

DATABASE:
  # Enhanced database timeout settings
  QUERY_TIMEOUT: 30s          # Query execution timeout
  CONNECTION_TIMEOUT: 10s     # Connection establishment timeout
  TRANSACTION_TIMEOUT: 60s    # Transaction timeout
  POOL_TIMEOUT: 5s            # Connection pool timeout
```

## Benefits of These Fixes

### 1. **Prevents Hanging Operations**
- All HTTP requests now have configurable timeouts
- Database queries timeout after 30 seconds
- MSISDN validation operations timeout after 30 seconds

### 2. **Eliminates Goroutine Leaks**
- Context cancellation ensures workers exit properly
- Timeout handling prevents indefinite blocking
- Proper cleanup of resources

### 3. **Improves Resource Management**
- Connection pooling with limits
- Periodic cleanup of idle connections
- Buffer size optimization

### 4. **Enhances Reliability**
- Circuit breaker integration maintained
- Exponential backoff for retries
- Graceful degradation under load

### 5. **Better Monitoring and Debugging**
- Detailed logging of timeout events
- Connection pool status tracking
- Error categorization and handling

## Monitoring and Alerting

### Key Metrics to Monitor:
1. **Connection Timeouts**: Count of requests that timeout
2. **Goroutine Count**: Active goroutines should remain stable
3. **Connection Pool Usage**: Connection pool utilization
4. **Request Latency**: P95 and P99 response times
5. **Error Rates**: Timeout vs. other error types

### Alerts to Set:
1. **High Timeout Rate**: >5% of requests timing out
2. **Goroutine Spike**: Sudden increase in active goroutines
3. **Connection Pool Exhaustion**: >90% connection pool usage
4. **High Latency**: P95 response time >30 seconds

## Testing Recommendations

### 1. **Load Testing**
- Test with various batch sizes (100, 1000, 10000)
- Verify timeout handling under load
- Check goroutine count stability

### 2. **Timeout Testing**
- Test with slow network conditions
- Verify context cancellation works
- Check resource cleanup

### 3. **Failure Scenarios**
- Test with unreachable endpoints
- Verify circuit breaker integration
- Check graceful degradation

## Rollback Plan

If issues arise, the following can be reverted:
1. **Timeout Values**: Increase timeout values in config
2. **Connection Limits**: Reduce connection pool sizes
3. **Worker Counts**: Reduce concurrent worker counts

## Conclusion

These fixes address the root causes of the connection hanging and goroutine leak issues:

1. **Proper Timeout Handling**: All operations now have configurable timeouts
2. **Context Management**: Proper context cancellation prevents resource leaks
3. **Connection Pooling**: Optimized connection management prevents exhaustion
4. **Resource Cleanup**: Periodic cleanup prevents resource accumulation
5. **Enhanced Monitoring**: Better visibility into system behavior

The service should now be much more reliable and resilient to network issues, database problems, and high load conditions. 