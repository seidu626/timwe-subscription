# High Volume Optimization Guide

## Overview

This document describes the optimizations implemented to handle 10,000+ requests efficiently in the subscription-external service. The system now includes dynamic scaling, improved resource management, and enhanced monitoring for high-volume batch processing.

## Key Optimizations

### 1. Dynamic Worker Scaling

The system now automatically scales the number of workers based on the request volume:

```go
func calculateOptimalWorkers(requestCount int) int {
    switch {
    case requestCount <= 100:
        return 5      // Small batches
    case requestCount <= 1000:
        return 20     // Medium batches
    case requestCount <= 5000:
        return 50     // Large batches
    case requestCount <= 10000:
        return 100    // Very large batches
    default:
        return 200    // Massive batches (10k+)
    }
}
```

### 2. Optimized HTTP Client Configuration

Enhanced HTTP client settings for high-volume processing:

```go
client := &fasthttp.Client{
    MaxConnsPerHost:     maxConnections,    // Minimum 1000 for high volume
    MaxIdleConnDuration: 30 * time.Second,  // Connection pooling
    ReadTimeout:         cfg.Application.TIMWE.Timeout,
    WriteTimeout:        cfg.Application.TIMWE.Timeout,
    MaxResponseBodySize: 10 * 1024 * 1024,  // 10MB max response size
    DisablePathNormalizing: true,           // Performance optimization
    NoDefaultUserAgentHeader: true,         // Reduce header overhead
}
```

### 3. Enhanced Circuit Breaker

Optimized circuit breaker for high-volume scenarios:

```go
cbSettings := gobreaker.Settings{
    Name:        "TIMWE API Circuit Breaker",
    MaxRequests: 100,              // Increased from 10
    Timeout:     15 * time.Second, // Faster recovery
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        // More lenient for high volume
        return counts.Requests >= 50 && 
               float64(counts.ConsecutiveFailures)/float64(counts.Requests) >= 0.8
    },
}
```

### 4. Adaptive Batch Processing

Dynamic batch size calculation based on request volume:

```go
func calculateOptimalBatchSize(requestCount int) int {
    maxBufferSize := 10000
    
    switch {
    case requestCount <= 100:
        return requestCount    // Small batches - buffer all
    case requestCount <= 1000:
        return 500            // Medium batches
    case requestCount <= 5000:
        return 1000           // Large batches
    case requestCount <= 10000:
        return 2000           // Very large batches
    default:
        return maxBufferSize  // Cap at reasonable size
    }
}
```

### 5. Progress Tracking

Real-time progress monitoring for large batches:

- **Progress Reporting**: Every 10 seconds for batches > 1000 requests
- **Percentage Tracking**: Real-time completion percentage
- **Performance Metrics**: Processing rate and throughput

### 6. Optimized Logging

Batch error logging to reduce overhead:

- **Error Batching**: Groups errors into batches of 10
- **Reduced Log Volume**: Only logs sample errors instead of every failure
- **Performance Impact**: Minimal logging overhead during high-volume processing

## Performance Characteristics

### Expected Throughput

| Request Count | Workers | Expected Time | Throughput |
|---------------|---------|---------------|------------|
| 100           | 5       | ~10 seconds   | 10 req/s   |
| 1,000         | 20      | ~30 seconds   | 33 req/s   |
| 5,000         | 50      | ~2 minutes    | 42 req/s   |
| 10,000        | 100     | ~4 minutes    | 42 req/s   |
| 50,000        | 200     | ~20 minutes   | 42 req/s   |

### Resource Requirements

#### Memory Usage
- **Base Memory**: ~50MB
- **Per 1,000 Requests**: ~10MB additional
- **10,000 Requests**: ~150MB total

#### CPU Usage
- **Concurrent Workers**: Scales with request volume
- **HTTP Connections**: Up to 1,000 concurrent connections
- **Database Connections**: Pooled and optimized

#### Network Usage
- **Connection Pooling**: Reuses connections for efficiency
- **Request Throttling**: 50ms between requests (reduced from 100ms)
- **Timeout Optimization**: Faster recovery from failures

## Configuration Recommendations

### For 10,000+ Requests

```yaml
application:
  timwe:
    maxConnections: 2000      # Increased for high volume
    timeout: 30s              # Reasonable timeout
    # Other settings...
```

### System Requirements

#### Minimum Requirements
- **CPU**: 4 cores
- **Memory**: 2GB RAM
- **Network**: 100 Mbps

#### Recommended for 10k+ Requests
- **CPU**: 8+ cores
- **Memory**: 4GB+ RAM
- **Network**: 1 Gbps
- **Storage**: SSD for database

## Monitoring and Observability

### Key Metrics to Monitor

1. **Processing Rate**: Requests per second
2. **Success Rate**: Percentage of successful subscriptions
3. **Error Rate**: Types and frequency of errors
4. **Circuit Breaker Status**: Open/closed state
5. **Memory Usage**: Heap and system memory
6. **CPU Usage**: Per-core utilization
7. **Network I/O**: Bytes sent/received

### Log Analysis

```bash
# Monitor processing progress
grep "Batch processing progress" logs/app.log

# Track error rates
grep "Batch of subscription failures" logs/app.log

# Monitor circuit breaker
grep "Circuit breaker" logs/app.log
```

## Best Practices

### 1. Batch Size Recommendations
- **Small Batches (≤100)**: Use single requests
- **Medium Batches (100-1,000)**: Use batch endpoint
- **Large Batches (1,000-10,000)**: Monitor progress
- **Massive Batches (10,000+)**: Consider splitting into smaller batches

### 2. Error Handling
- **Retry Logic**: Implement exponential backoff
- **Circuit Breaker**: Monitor and adjust thresholds
- **Error Classification**: Distinguish between transient and permanent errors

### 3. Resource Management
- **Connection Pooling**: Reuse HTTP connections
- **Memory Management**: Monitor heap usage
- **CPU Optimization**: Scale workers appropriately

### 4. Monitoring
- **Real-time Metrics**: Use progress tracking
- **Alerting**: Set up alerts for high error rates
- **Logging**: Use structured logging for analysis

## Troubleshooting

### Common Issues

1. **Memory Exhaustion**
   - Reduce batch size
   - Increase system memory
   - Monitor heap usage

2. **Circuit Breaker Tripping**
   - Check TIMWE API status
   - Adjust circuit breaker thresholds
   - Monitor error patterns

3. **Slow Processing**
   - Increase worker count
   - Check network connectivity
   - Monitor TIMWE API response times

4. **High Error Rates**
   - Validate MSISDN format
   - Check product configuration
   - Monitor TIMWE API errors

## Future Enhancements

### Planned Improvements

1. **Async Processing**: Implement async/await pattern
2. **Database Optimization**: Connection pooling and query optimization
3. **Caching**: Redis caching for frequently accessed data
4. **Load Balancing**: Distribute load across multiple instances
5. **Auto-scaling**: Dynamic scaling based on load
6. **Metrics Dashboard**: Real-time monitoring dashboard

### Performance Targets

- **10,000 Requests**: < 5 minutes processing time
- **50,000 Requests**: < 20 minutes processing time
- **100,000 Requests**: < 45 minutes processing time
- **Error Rate**: < 5% for valid requests
- **Memory Usage**: < 2GB for 50k requests 