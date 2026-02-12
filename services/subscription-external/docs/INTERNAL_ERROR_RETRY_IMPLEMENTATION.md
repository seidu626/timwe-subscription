# INTERNAL_ERROR Retry Implementation

## Overview
This document describes the implementation of retry logic for INTERNAL_ERROR responses in the subscription-external service. The system now automatically retries requests when the TIMWE API returns an INTERNAL_ERROR response code.

## Problem Statement

### INTERNAL_ERROR Handling
The TIMWE API can return `INTERNAL_ERROR` response codes, which typically indicate temporary server-side issues that may resolve with retry attempts. Previously, these errors were treated as permanent failures and immediately returned to the client.

### Requirements
- Retry requests up to 3 times when receiving INTERNAL_ERROR response codes
- Use exponential backoff between retry attempts
- Maintain proper logging and error context
- Preserve existing circuit breaker functionality

## Implementation Details

### Retry Strategy

#### Maximum Retries
- **Count**: 3 attempts maximum
- **Base Delay**: 200ms
- **Backoff Strategy**: Exponential (200ms, 400ms, 800ms)

#### Retry Conditions
The following conditions trigger retry attempts:
1. **Network Errors**: Connection failures, timeouts
2. **HTTP Errors**: Non-200 status codes
3. **Parsing Errors**: JSON unmarshaling failures
4. **INTERNAL_ERROR Response**: TIMWE API returns `code: "INTERNAL_ERROR"`

#### Non-Retryable Conditions
The following conditions do not trigger retries:
1. **Business Logic Errors**: Invalid MSISDN, configuration errors
2. **Validation Errors**: Response validation failures
3. **Success Responses**: Any successful response

### Code Structure

#### Refactored Methods
1. **`SendMT`**: Main subscription request method
2. **`SendStatusCheck`**: Status check request method

#### New Helper Methods
1. **`sendMTWithRetry`**: Handles MT request retry logic
2. **`sendStatusCheckWithRetry`**: Handles status check retry logic

### Retry Flow

```
Request → Send → Parse Response → Check Code
                                    ↓
                              INTERNAL_ERROR?
                                    ↓
                              Yes → Retry (max 3x)
                                    ↓
                              No → Return Response
```

### Exponential Backoff Calculation
```go
delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
```

**Attempt Delays:**
- Attempt 1: 200ms
- Attempt 2: 400ms  
- Attempt 3: 800ms

## Logging Enhancements

### Retry Attempt Logging
- **Warning Level**: Retry attempts with attempt number
- **Info Level**: Retry delays and next attempt details
- **Error Level**: Final failure after all retries

### Enhanced Context
- **Attempt Number**: Track which retry attempt
- **MSISDN**: User identifier for correlation
- **Request ID**: TIMWE request identifier
- **Delay Duration**: Backoff delay information

### Example Log Messages
```
WARN  MT request failed with internal error, retrying attempt=1 msisdn=233244123456 requestId=1530783:1754395350765
INFO  Retrying MT request after INTERNAL_ERROR attempt=2 delay=400ms msisdn=233244123456
ERROR MT request failed with internal error after all retries msisdn=233244123456 requestId=1530783:1754395350765
```

## Resource Management

### Memory Management
- **Request/Response Objects**: Properly released after each attempt
- **Connection Pooling**: Maintained through fasthttp client
- **Circuit Breaker**: Preserved existing functionality

### Performance Considerations
- **Connection Reuse**: HTTP connections reused across retries
- **Request Body**: Marshaled once, reused across attempts
- **Auth Key**: Generated once, reused across attempts

## Error Handling

### Retryable vs Non-Retryable Errors

#### Retryable Errors
- Network connection failures
- HTTP 5xx status codes
- JSON parsing errors
- INTERNAL_ERROR response codes

#### Non-Retryable Errors
- HTTP 4xx status codes (client errors)
- Business logic validation errors
- Authentication failures
- Configuration errors

### Error Propagation
- **Circuit Breaker**: Maintains existing circuit breaker integration
- **Error Context**: Preserves original error context
- **Request ID**: Includes TIMWE request ID in error messages

## Testing Considerations

### Test Scenarios
1. **Single INTERNAL_ERROR**: Should retry and succeed
2. **Multiple INTERNAL_ERROR**: Should retry up to 3 times then fail
3. **Mixed Responses**: INTERNAL_ERROR followed by success
4. **Network Errors**: Combined with INTERNAL_ERROR handling
5. **Circuit Breaker**: Integration with existing circuit breaker

### Mock Responses
```json
{
  "code": "INTERNAL_ERROR",
  "message": "Internal server error",
  "requestId": "1530783:1754395350765",
  "inError": true
}
```

## Configuration

### Retry Parameters
- **Max Retries**: 3 (hardcoded)
- **Base Delay**: 200ms (hardcoded)
- **Backoff Multiplier**: 2x (hardcoded)

### Future Enhancements
- **Configurable Retries**: Make retry count configurable
- **Configurable Delays**: Make base delay configurable
- **Jitter**: Add random jitter to prevent thundering herd
- **Retry Headers**: Add retry attempt headers to requests

## Monitoring and Observability

### Metrics to Track
- **Retry Count**: Number of retries per request
- **Retry Success Rate**: Percentage of retries that succeed
- **Average Retry Delay**: Mean delay between retries
- **INTERNAL_ERROR Rate**: Frequency of INTERNAL_ERROR responses

### Alerting
- **High Retry Rate**: Alert when retry rate exceeds threshold
- **Retry Failure Rate**: Alert when retry success rate drops
- **Circuit Breaker Trips**: Existing circuit breaker alerts

## Rollback Plan

### Emergency Rollback
If issues arise with the retry logic:
1. **Feature Flag**: Add retry enable/disable flag
2. **Gradual Rollout**: Deploy to subset of traffic first
3. **Monitoring**: Watch error rates and performance metrics
4. **Quick Revert**: Revert to previous implementation if needed

### Rollback Steps
1. Deploy previous version without retry logic
2. Monitor error rates and performance
3. Investigate root cause of issues
4. Fix and redeploy with retry logic

## Conclusion

The INTERNAL_ERROR retry implementation provides:
- **Improved Reliability**: Automatic recovery from temporary server issues
- **Better User Experience**: Reduced failure rates for transient errors
- **Enhanced Observability**: Detailed logging and monitoring capabilities
- **Maintained Performance**: Efficient resource usage and minimal overhead

This implementation follows Go best practices and maintains compatibility with existing circuit breaker patterns while adding robust retry capabilities for INTERNAL_ERROR responses. 