# Goroutine Leak Fixes and Prevention Guide

## Issues Identified

The stack trace revealed several critical issues causing goroutine leaks and resource exhaustion:

1. **Missing Graceful Shutdown**: The main service didn't implement proper graceful shutdown
2. **Goroutine Leaks**: HTTP requests were creating goroutines that never got cleaned up
3. **Database Connection Issues**: PostgreSQL connections were hanging due to missing timeouts
4. **Resource Cleanup**: FastHTTP client connections and database rows weren't being properly closed
5. **Context Management**: Missing context cancellation and timeouts in critical operations

## Fixes Implemented

### 1. Graceful Shutdown Implementation

**File**: `services/subscription-external/cmd/main.go`

- Added signal handling for SIGINT and SIGTERM
- Implemented proper server shutdown with context timeout
- Added cleanup of all monitors, workers, and connections
- Ensured database and Redis connections are properly closed

```go
// Set up signal handling for graceful shutdown
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

go func() {
    if err := server.ListenAndServe(fmt.Sprintf(":%d", cfg.Application.Port)); err != nil {
        log.Printf("Server error: %v", err)
    }
}()

<-quit
log.Println("Shutting down server...")

// Create shutdown context with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := server.ShutdownWithContext(shutdownCtx); err != nil {
    log.Printf("Failed to shutdown server: %v", err)
}
```

### 2. HTTP Request Goroutine Leak Prevention

**File**: `services/subscription-external/internal/service/subscription.go`

- Fixed `sendMTWithRetry` function to properly handle context cancellation
- Added proper resource cleanup with defer statements
- Prevented goroutine leaks in HTTP request handling
- Added context timeout handling to prevent hanging requests

```go
// Ensure cleanup happens regardless of how we exit
defer func() {
    fasthttp.ReleaseRequest(req)
    fasthttp.ReleaseResponse(res)
}()

// Send request with context timeout
requestDone := make(chan error, 1)
go func() {
    select {
    case requestDone <- s.client.Do(req, res):
    case <-ctx.Done():
        // Context was cancelled, don't block
        select {
        case requestDone <- ctx.Err():
        default:
        }
    }
}()
```

### 3. Database Connection Timeout Configuration

**File**: `common/postgres/database.go`

- Added connection timeouts to prevent hanging connections
- Set statement and transaction timeouts
- Configured connection pool limits
- Added health check configuration
- **NEW**: Unified configuration system that integrates with main config file
- **NEW**: Removed all hardcoded values in favor of configurable settings
- **NEW**: Support for multiple configuration sources (main config, env vars, defaults)

```go
// Set connection timeouts to prevent hanging connections
conf.ConnConfig.Config.ConnectTimeout = config.ConnectTimeout
conf.ConnConfig.Config.RuntimeParams["statement_timeout"] = config.StatementTimeout.String()
conf.ConnConfig.Config.RuntimeParams["idle_in_transaction_session_timeout"] = config.IdleInTransactionSessionTimeout.String()

// Set connection pool limits to prevent resource exhaustion
conf.MaxConns = config.MaxConns
conf.MinConns = config.MinConns
conf.MaxConnLifetime = config.MaxConnLifetime
conf.MaxConnIdleTime = config.MaxConnIdleTime

// Health check configuration
conf.HealthCheckPeriod = config.HealthCheckPeriod
```

**Configuration Sources (in order of priority):**
1. **Direct configuration struct** - Programmatic configuration
2. **Main application config** - Centralized configuration file
3. **Environment variables** - Runtime configuration
4. **Default values** - Fallback settings

**Usage Examples:**
```go
// Option 1: Use main config (recommended for production)
mainConfig := config.InitConfig(logger, "config", []string{"config.yaml"})
pool, err := postgres.NewPGXPoolFromMainConfig(ctx, mainConfig, logger, logLevel)

// Option 2: Use defaults (recommended for development)
pool, err := postgres.NewPGXPoolWithDefaults(ctx, "", logger, logLevel)

// Option 3: Use environment variables
pool, err := postgres.NewPGXPoolFromEnv(ctx, "", logger, logLevel)

// Option 4: Use custom configuration
config := &postgres.DatabaseConfig{ /* your settings */ }
pool, err := postgres.NewPGXPoolFromConfig(ctx, config, logger, logLevel)
```

### 4. Service Cleanup and Resource Management

**File**: `services/subscription-external/internal/service/subscription.go`

- Added proper service shutdown method
- Implemented connection cleanup ticker
- Added idle connection cleanup
- Proper resource lifecycle management

```go
// Stop gracefully shuts down the subscription service
func (s *SubscriptionService) Stop() {
    s.logger.Info("Stopping subscription service...")
    
    // Close all idle connections
    s.client.CloseIdleConnections()
    
    // Stop the cleanup goroutine
    if s.cleanupTicker != nil {
        s.cleanupTicker.Stop()
    }
    
    s.logger.Info("Subscription service stopped")
}
```

## Configuration Recommendations

### 1. Environment Variables

Add these environment variables to your deployment configuration:

```bash
# Database timeouts
PGCONNECT_TIMEOUT=10
PGSTATEMENT_TIMEOUT=30
PGIDLE_IN_TRANSACTION_SESSION_TIMEOUT=30

# Connection pool limits
PGMAX_CONNS=50
PGMIN_CONNS=5
PGMAX_CONN_LIFETIME=3600
PGMAX_CONN_IDLE_TIME=1800
```

### 2. Application Configuration

Update your `config.yaml` to include proper timeouts:

```yaml
APPLICATION:
  HTTP:
    READ_TIMEOUT: 60s
    WRITE_TIMEOUT: 60s
    IDLE_TIMEOUT: 120s
    CONCURRENCY: 1000  # Limit concurrent requests
  
  TIMWE_MA:
    TIMEOUT: 30s
    DIAL_TIMEOUT: 10s
    MAX_CONN_DURATION: 60s
    IDLE_CONN_TIMEOUT: 30s
```

## Monitoring and Prevention

### 1. Goroutine Monitoring

Add goroutine count monitoring to your health checks:

```go
func (s *SubscriptionService) GetHealthStatus() map[string]interface{} {
    return map[string]interface{}{
        "goroutines": runtime.NumGoroutine(),
        "memory": map[string]interface{}{
            "alloc": runtime.ReadMemStats().Alloc,
            "total": runtime.ReadMemStats().TotalAlloc,
        },
        "connections": s.client.ConnNum(),
    }
}
```

### 2. Connection Pool Monitoring

Monitor database connection pool health:

```go
func (db *Database) GetPoolStats() map[string]interface{} {
    stats := db.pool.Stat()
    return map[string]interface{}{
        "total_connections": stats.TotalConns(),
        "idle_connections":  stats.IdleConns(),
        "in_use":           stats.AcquiredConns(),
        "wait_count":       stats.WaitCount(),
    }
}
```

### 3. Regular Health Checks

Implement regular health checks to detect issues early:

```go
func (s *SubscriptionService) healthCheck() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        goroutines := runtime.NumGoroutine()
        if goroutines > 1000 {
            s.logger.Warn("High goroutine count detected", 
                zap.Int("goroutines", goroutines))
        }
        
        // Check connection pool health
        if s.client.ConnNum() > 100 {
            s.logger.Warn("High connection count detected",
                zap.Int("connections", s.client.ConnNum()))
        }
    }
}
```

## Best Practices

### 1. Always Use Context

- Use `context.WithTimeout` for all external calls
- Pass context through the call chain
- Cancel contexts when operations complete

### 2. Resource Cleanup

- Use `defer` statements for cleanup
- Implement proper `Close()` methods
- Clean up resources in shutdown handlers

### 3. Connection Management

- Set appropriate timeouts for all connections
- Use connection pooling with limits
- Monitor connection pool health

### 4. Error Handling

- Handle errors gracefully
- Implement retry logic with backoff
- Log errors with sufficient context

### 5. Testing

- Test graceful shutdown scenarios
- Monitor goroutine counts in tests
- Test connection timeout scenarios

## Deployment Checklist

- [ ] Graceful shutdown implemented
- [ ] Connection timeouts configured
- [ ] Resource cleanup handlers added
- [ ] Health monitoring implemented
- [ ] Configuration validated
- [ ] Load testing completed
- [ ] Monitoring alerts configured

## Conclusion

These fixes address the root causes of goroutine leaks and resource exhaustion. The key principles are:

1. **Always use context with timeouts**
2. **Implement proper cleanup and shutdown**
3. **Monitor resource usage**
4. **Set appropriate limits and timeouts**
5. **Handle errors gracefully**

Regular monitoring and proactive resource management will prevent these issues from recurring. 