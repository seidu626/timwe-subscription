# 🛡️ Enhanced Resilience & Recovery for NotificationMonitor

## 📋 Overview

The NotificationMonitor has been enhanced with comprehensive resilience and recovery mechanisms to handle network connectivity issues, database failures, and other operational challenges that can occur in production environments.

## 🚨 Problems Addressed

### **Network Connectivity Issues**
- **DNS Resolution Failures**: `lookup tigo.timwe.com: Temporary failure in name resolution`
- **Connection Timeouts**: `dial tcp 195.23.53.126:443: connect: no route to host`
- **Circuit Breaker Failures**: `Circuit breaker classified failure in SendMT`

### **Cascading Failures**
- Network issues → Opt-in failures → Opt-out processing incomplete
- Database connectivity problems → Processing halted
- Service unavailability → Worker becomes unresponsive

## 🏗️ Architecture Overview

### **Circuit Breaker Pattern**
The system implements a three-state circuit breaker:
1. **CLOSED**: Normal operation, requests pass through
2. **OPEN**: System is failing, requests are blocked
3. **HALF_OPEN**: Testing recovery, limited requests allowed

### **Health Check System**
- **Periodic health checks** for Redis and database connectivity
- **Automatic circuit breaker management** based on health status
- **Graceful degradation** when system health is compromised

### **Exponential Backoff with Jitter**
- **Intelligent retry logic** with exponential backoff
- **Jitter addition** to prevent thundering herd problems
- **Configurable retry limits** and backoff parameters

## ⚙️ Configuration

### **Resilience Configuration**
```yaml
resilience:
  max_retries: 3                    # Maximum retry attempts for failed operations
  initial_backoff: "1s"             # Initial backoff duration
  max_backoff: "30s"                # Maximum backoff duration
  backoff_multiplier: 2.0           # Multiplier for exponential backoff
  circuit_breaker_threshold: 5      # Number of failures before circuit breaker opens
  circuit_breaker_timeout: "60s"    # Time to wait before attempting recovery
  graceful_degradation: true        # Enable graceful degradation mode
  health_check_interval: "30s"      # Interval for health checks
```

### **Default Values**
```go
if cfg.MaxRetries <= 0 {
    cfg.MaxRetries = 3
}
if cfg.InitialBackoff <= 0 {
    cfg.InitialBackoff = 1 * time.Second
}
if cfg.MaxBackoff <= 0 {
    cfg.MaxBackoff = 30 * time.Second
}
if cfg.BackoffMultiplier <= 0 {
    cfg.BackoffMultiplier = 2.0
}
if cfg.CircuitBreakerThreshold <= 0 {
    cfg.CircuitBreakerThreshold = 5
}
if cfg.CircuitBreakerTimeout <= 0 {
    cfg.CircuitBreakerTimeout = 60 * time.Second
}
if cfg.HealthCheckInterval <= 0 {
    cfg.HealthCheckInterval = 30 * time.Second
}
```

## 🔧 Implementation Details

### **1. Circuit Breaker Management**

#### **State Transitions**
```go
// Circuit breaker opens when failure threshold is reached
if m.circuitBreakerFailures >= m.cfg.CircuitBreakerThreshold {
    m.circuitBreakerState = "OPEN"
    m.logger.Warn("circuit breaker opened due to high failure rate",
        zap.String("errorType", errorType),
        zap.Int("failures", m.circuitBreakerFailures),
        zap.Int("threshold", m.cfg.CircuitBreakerThreshold))
}
```

#### **Recovery Attempts**
```go
func (m *NotificationMonitor) attemptCircuitBreakerRecovery() {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.circuitBreakerState == "OPEN" {
        m.circuitBreakerState = "HALF_OPEN"
        m.logger.Info("circuit breaker moved to HALF_OPEN state for recovery attempt")
    }
}
```

### **2. Health Check System**

#### **Periodic Health Monitoring**
```go
func (m *NotificationMonitor) healthCheckLoop() {
    ticker := time.NewTicker(m.cfg.HealthCheckInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            m.performHealthCheck()
        case <-m.ctx.Done():
            return
        }
    }
}
```

#### **Comprehensive Health Checks**
```go
func (m *NotificationMonitor) performHealthCheck() {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Check Redis connectivity
    _, err := m.redis.Ping(m.ctx).Result()
    if err != nil {
        m.logger.Error("health check failed: Redis connectivity issue", zap.Error(err))
        m.recordError("health_check_redis")
    }

    // Check database connectivity
    if err := m.checkDatabaseHealth(); err != nil {
        m.logger.Error("health check failed: Database connectivity issue", zap.Error(err))
        m.recordError("health_check_database")
    }

    // Update health check timestamp
    m.healthLastCheck = time.Now()

    // Check if we should enter graceful degradation mode
    if m.consecutiveErrors >= m.cfg.CircuitBreakerThreshold {
        m.gracefulMode = true
        m.logger.Warn("entering graceful degradation mode due to high error rate",
            zap.Int("consecutiveErrors", m.consecutiveErrors),
            zap.Int("threshold", m.cfg.CircuitBreakerThreshold))
    }
}
```

### **3. Enhanced Opt-in Processing**

#### **Resilient Opt-in with Retry Logic**
```go
func (m *NotificationMonitor) attemptOptinWithResilience(msisdn, productIDStr, channel string) bool {
    // Check circuit breaker state before attempting
    if m.isCircuitBreakerOpen() {
        m.logger.Debug("circuit breaker is OPEN, skipping opt-in attempt",
            zap.String("msisdn", msisdn),
            zap.String("productId", productIDStr),
            zap.String("entryChannel", channel))
        return false
    }

    // Implement exponential backoff with retry logic
    backoff := m.cfg.InitialBackoff
    maxBackoff := m.cfg.MaxBackoff

    for attempt := 1; attempt <= m.cfg.MaxRetries; attempt++ {
        // Attempt opt-in with timeout and error handling
        err := m.attemptOptinWithTimeout(optin, 30*time.Second)
        if err == nil {
            // Success! Record success to potentially close circuit breaker
            m.recordSuccess()
            return true
        }

        // Analyze error type and handle accordingly
        errorType := m.classifyOptinError(err)
        m.recordError(fmt.Sprintf("optin_%s", errorType))

        // Check if this is a permanent failure that shouldn't be retried
        if m.isPermanentOptinFailure(err) {
            return false
        }

        // Implement exponential backoff with jitter
        if attempt < m.cfg.MaxRetries {
            jitter := time.Duration(rand.Intn(100)) * time.Millisecond
            sleepDuration := backoff + jitter
            
            if sleepDuration > maxBackoff {
                sleepDuration = maxBackoff
            }

            time.Sleep(sleepDuration)

            // Calculate next backoff
            backoff = time.Duration(float64(backoff) * m.cfg.BackoffMultiplier)
            if backoff > maxBackoff {
                backoff = maxBackoff
            }
        }
    }

    return false
}
```

#### **Error Classification and Handling**
```go
func (m *NotificationMonitor) classifyOptinError(err error) string {
    if err == nil {
        return "none"
    }

    errStr := err.Error()
    
    // Network-related errors
    if strings.Contains(errStr, "dial tcp") || 
       strings.Contains(errStr, "no route to host") ||
       strings.Contains(errStr, "connection refused") ||
       strings.Contains(errStr, "timeout") {
        return "network"
    }

    // DNS-related errors
    if strings.Contains(errStr, "lookup") || 
       strings.Contains(errStr, "name resolution") {
        return "dns"
    }

    // Circuit breaker errors
    if strings.Contains(errStr, "circuit breaker") || 
       strings.Contains(errStr, "Circuit breaker classified failure") {
        return "circuit_breaker"
    }

    // Business logic errors
    if strings.Contains(errStr, "MSISDN") || 
       strings.Contains(errStr, "product") ||
       strings.Contains(errStr, "validation") {
        return "business_logic"
    }

    return "unknown"
}
```

### **4. Graceful Degradation**

#### **Processing Mode Selection**
```go
func (m *NotificationMonitor) processCycleWithResilience() error {
    // Check if we're in graceful degradation mode
    if m.gracefulMode {
        m.logger.Info("processing cycle in graceful degradation mode",
            zap.Int("consecutiveErrors", m.getCircuitBreakerFailures()))
        
        // In graceful mode, only process critical operations
        if err := m.processUserOptoutWithResilience(); err != nil {
            return fmt.Errorf("graceful mode processing failed: %w", err)
        }
        
        // Skip other processing in graceful mode
        return nil
    }

    // Normal processing mode
    if err := m.processCycle(); err != nil {
        return err
    }

    // Record success to potentially close circuit breaker
    m.recordSuccess()
    return nil
}
```

## 📊 Monitoring and Metrics

### **Resilience Status**
```go
func (m *NotificationMonitor) GetResilienceStatus() map[string]interface{} {
    m.mu.RLock()
    defer m.mu.RUnlock()

    return map[string]interface{}{
        "circuit_breaker_state":    m.circuitBreakerState,
        "circuit_breaker_failures": m.circuitBreakerFailures,
        "last_failure_time":        m.lastFailureTime.Format(time.RFC3339),
        "consecutive_errors":       m.consecutiveErrors,
        "graceful_mode":            m.gracefulMode,
        "health_last_check":        m.healthLastCheck.Format(time.RFC3339),
        "is_healthy":               m.isHealthy(),
    }
}
```

### **Health Check Status**
```go
func (m *NotificationMonitor) isHealthy() bool {
    // System is healthy if:
    // 1. Circuit breaker is not OPEN
    // 2. Consecutive errors are below threshold
    // 3. Recent health checks passed
    return m.circuitBreakerState != "OPEN" &&
           m.consecutiveErrors < m.cfg.CircuitBreakerThreshold &&
           time.Since(m.healthLastCheck) < m.cfg.HealthCheckInterval*2
}
```

## 🔄 Recovery Flow

### **1. Normal Operation (CLOSED State)**
```
Request → Process → Success/Failure
                ↓
        Record Result → Update Circuit Breaker State
```

### **2. Circuit Breaker Opens (OPEN State)**
```
High Failure Rate → Circuit Breaker Opens → Block All Requests
                                              ↓
                                    Wait for Timeout
                                              ↓
                                    Move to HALF_OPEN
```

### **3. Recovery Attempt (HALF_OPEN State)**
```
Allow Limited Requests → Monitor Results
        ↓
    Success → Close Circuit Breaker
        ↓
    Failure → Return to OPEN State
```

### **4. Graceful Degradation**
```
High Error Rate → Enter Graceful Mode → Process Only Critical Operations
                                              ↓
                                    Monitor System Health
                                              ↓
                                    Return to Normal Mode
```

## 🎯 Benefits

### **Operational Excellence**
- **Automatic failure detection** and circuit breaker management
- **Proactive health monitoring** with periodic checks
- **Intelligent retry logic** with exponential backoff

### **Business Continuity**
- **Graceful degradation** during system stress
- **Automatic recovery** from temporary failures
- **Reduced manual intervention** requirements

### **System Reliability**
- **Prevention of cascading failures** through circuit breakers
- **Timeout protection** for long-running operations
- **Error classification** for targeted handling

### **Monitoring and Observability**
- **Comprehensive health status** reporting
- **Circuit breaker state tracking** for operational visibility
- **Detailed error logging** with context and classification

## 🚀 Usage Examples

### **Configuration Loading**
```go
config, err := LoadNotificationMonitorConfig("config/notification-monitor.yaml")
if err != nil {
    log.Fatalf("Failed to load configuration: %v", err)
}

monitor := NewNotificationMonitor(logger, repo, userSvc, redis, config)
```

### **Status Monitoring**
```go
// Get current resilience status
status := monitor.GetResilienceStatus()
fmt.Printf("Circuit Breaker State: %s\n", status["circuit_breaker_state"])
fmt.Printf("Is Healthy: %v\n", status["is_healthy"])

// Get configuration summary
config := monitor.GetConfigurationSummary()
fmt.Printf("Max Retries: %d\n", config["max_retries"])
fmt.Printf("Circuit Breaker Threshold: %d\n", config["circuit_breaker_threshold"])
```

### **Runtime Configuration Updates**
```go
// Update configuration at runtime
newConfig := NotificationMonitorConfig{
    MaxRetries: 5,
    CircuitBreakerThreshold: 10,
    // ... other settings
}

err := monitor.UpdateConfiguration(newConfig)
if err != nil {
    log.Printf("Failed to update configuration: %v", err)
}
```

## 🔍 Troubleshooting

### **Common Issues and Solutions**

#### **1. Circuit Breaker Stuck in OPEN State**
- **Check health check logs** for connectivity issues
- **Verify timeout configuration** is appropriate
- **Monitor consecutive error count** and threshold

#### **2. High Retry Counts**
- **Review backoff configuration** for appropriate delays
- **Check error classification** for permanent vs. temporary failures
- **Monitor network connectivity** to external services

#### **3. Graceful Degradation Mode**
- **Check system health status** and recent failures
- **Review circuit breaker threshold** configuration
- **Monitor recovery attempts** and success rates

### **Debug Logging**
Enable debug logging to see detailed resilience behavior:
```yaml
logging:
  level: "debug"
  include_caller: true
```

## 📈 Performance Considerations

### **Memory Usage**
- **Circuit breaker state**: Minimal memory overhead
- **Health check data**: Small, bounded storage
- **Error tracking**: Configurable limits prevent unbounded growth

### **CPU Overhead**
- **Health checks**: Minimal impact (configurable interval)
- **Error classification**: Fast string matching
- **Circuit breaker logic**: Negligible computational cost

### **Network Impact**
- **Health check queries**: Lightweight database/Redis calls
- **Retry logic**: Controlled by backoff and jitter
- **Graceful degradation**: Reduces external service calls

## 🔮 Future Enhancements

### **Planned Improvements**
1. **Adaptive thresholds** based on historical performance
2. **Machine learning** for error pattern recognition
3. **Distributed circuit breakers** across multiple instances
4. **Advanced health check** with dependency mapping
5. **Integration with external monitoring** systems

### **Extensibility Points**
- **Custom error classifiers** for domain-specific errors
- **Pluggable health check** providers
- **Configurable circuit breaker** strategies
- **Custom graceful degradation** policies

---

This enhanced resilience and recovery system ensures that the NotificationMonitor can operate reliably even in challenging network and infrastructure conditions, providing robust business continuity for subscription management operations. 