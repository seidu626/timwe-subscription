// Adaptive rate limiter for resubscription processing
// File: internal/service/rate_limiter.go

package service

import (
    "sync"
    "time"
    
    "go.uber.org/zap"
)

// AdaptiveRateLimiter manages request rate based on error rates
type AdaptiveRateLimiter struct {
    baseRate        int
    currentRate     int
    minRate         int
    maxRate         int
    errorThreshold  float64
    adjustInterval  time.Duration
    
    // Metrics
    totalRequests   int64
    failedRequests  int64
    window          time.Duration
    windowStart     time.Time
    
    // Control
    ticker          *time.Ticker
    mu              sync.RWMutex
    logger          *zap.Logger
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(baseRate int, errorThreshold float64, logger *zap.Logger) *AdaptiveRateLimiter {
    if baseRate <= 0 {
        baseRate = 100
    }
    if errorThreshold <= 0 {
        errorThreshold = 0.05 // 5% default
    }
    
    rl := &AdaptiveRateLimiter{
        baseRate:       baseRate,
        currentRate:    baseRate,
        minRate:        1,
        maxRate:        baseRate * 2,
        errorThreshold: errorThreshold,
        adjustInterval: 30 * time.Second,
        window:         1 * time.Minute,
        windowStart:    time.Now(),
        logger:         logger,
    }
    
    rl.ticker = time.NewTicker(time.Second / time.Duration(rl.currentRate))
    go rl.monitor()
    
    return rl
}

// Wait blocks until the next request can be sent
func (rl *AdaptiveRateLimiter) Wait() {
    <-rl.ticker.C
}

// RecordRequest records a request result for rate adjustment
func (rl *AdaptiveRateLimiter) RecordRequest(success bool) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    rl.totalRequests++
    if !success {
        rl.failedRequests++
    }
}

// monitor periodically adjusts the rate based on error rates
func (rl *AdaptiveRateLimiter) monitor() {
    adjustTicker := time.NewTicker(rl.adjustInterval)
    defer adjustTicker.Stop()
    
    for range adjustTicker.C {
        rl.adjustRate()
    }
}

// adjustRate adjusts the rate based on current error rate
func (rl *AdaptiveRateLimiter) adjustRate() {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    // Reset window if needed
    if time.Since(rl.windowStart) > rl.window {
        rl.totalRequests = 0
        rl.failedRequests = 0
        rl.windowStart = time.Now()
        return
    }
    
    if rl.totalRequests == 0 {
        return
    }
    
    errorRate := float64(rl.failedRequests) / float64(rl.totalRequests)
    oldRate := rl.currentRate
    
    if errorRate > rl.errorThreshold {
        // High error rate - reduce speed
        rl.currentRate = int(float64(rl.currentRate) * 0.8)
        if rl.currentRate < rl.minRate {
            rl.currentRate = rl.minRate
        }
    } else if errorRate < rl.errorThreshold/2 {
        // Low error rate - increase speed
        rl.currentRate = int(float64(rl.currentRate) * 1.1)
        if rl.currentRate > rl.maxRate {
            rl.currentRate = rl.maxRate
        }
    }
    
    // Apply new rate if changed
    if oldRate != rl.currentRate {
        rl.ticker.Reset(time.Second / time.Duration(rl.currentRate))
        rl.logger.Info("Rate limit adjusted",
            zap.Int("oldRate", oldRate),
            zap.Int("newRate", rl.currentRate),
            zap.Float64("errorRate", errorRate),
            zap.Int64("totalRequests", rl.totalRequests),
            zap.Int64("failedRequests", rl.failedRequests),
        )
    }
}

// GetCurrentRate returns the current rate limit
func (rl *AdaptiveRateLimiter) GetCurrentRate() int {
    rl.mu.RLock()
    defer rl.mu.RUnlock()
    return rl.currentRate
}

// GetStats returns current statistics
func (rl *AdaptiveRateLimiter) GetStats() map[string]interface{} {
    rl.mu.RLock()
    defer rl.mu.RUnlock()
    
    errorRate := float64(0)
    if rl.totalRequests > 0 {
        errorRate = float64(rl.failedRequests) / float64(rl.totalRequests)
    }
    
    return map[string]interface{}{
        "current_rate":     rl.currentRate,
        "base_rate":        rl.baseRate,
        "error_threshold":  rl.errorThreshold,
        "total_requests":   rl.totalRequests,
        "failed_requests":  rl.failedRequests,
        "error_rate":       errorRate,
        "window_start":     rl.windowStart,
    }
}

// Stop stops the rate limiter
func (rl *AdaptiveRateLimiter) Stop() {
    rl.ticker.Stop()
}

// BulkheadLimiter provides concurrency control
type BulkheadLimiter struct {
    semaphore chan struct{}
    maxConcurrent int
    currentActive int64
    mu sync.RWMutex
}

// NewBulkheadLimiter creates a new bulkhead limiter
func NewBulkheadLimiter(maxConcurrent int) *BulkheadLimiter {
    if maxConcurrent <= 0 {
        maxConcurrent = 100
    }
    
    return &BulkheadLimiter{
        semaphore:     make(chan struct{}, maxConcurrent),
        maxConcurrent: maxConcurrent,
    }
}

// Acquire acquires a permit
func (bl *BulkheadLimiter) Acquire() {
    bl.semaphore <- struct{}{}
    bl.mu.Lock()
    bl.currentActive++
    bl.mu.Unlock()
}

// Release releases a permit
func (bl *BulkheadLimiter) Release() {
    <-bl.semaphore
    bl.mu.Lock()
    bl.currentActive--
    bl.mu.Unlock()
}

// TryAcquire attempts to acquire a permit without blocking
func (bl *BulkheadLimiter) TryAcquire() bool {
    select {
    case bl.semaphore <- struct{}{}:
        bl.mu.Lock()
        bl.currentActive++
        bl.mu.Unlock()
        return true
    default:
        return false
    }
}

// GetActiveCount returns the number of active operations
func (bl *BulkheadLimiter) GetActiveCount() int64 {
    bl.mu.RLock()
    defer bl.mu.RUnlock()
    return bl.currentActive
}
