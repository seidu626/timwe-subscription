# Already Active Subscription Handling Implementation

## Overview
This document describes the implementation of enhanced handling for `SubscriptionResultOptinAlreadyActive` responses in the subscription-external service. When an optin request returns this response, the system now:

1. Checks if the MSISDN already exists in the subscriptions database
2. Inserts the subscription record if it doesn't exist
3. Checks for renewal notifications in the current month
4. Sends a renewal request if no renewal notification was sent this month

## Changes Made

### 1. Repository Layer Enhancements
**File: `internal/repository/postgres.go`**

#### New Methods Added:
- **`CheckSubscriptionExists(msisdn string, productId int) (bool, error)`**
  - Checks if an active subscription exists for the given MSISDN and product
  - Returns true if subscription exists, false otherwise
  - Includes proper error handling for database operations

- **`CheckRenewalNotificationExists(msisdn string, productId int) (bool, error)`**
  - Checks if a renewal notification was sent to the MSISDN in the current month
  - Uses the first day of the current month as the start date
  - Filters by notification type 'RENEWAL'
  - Returns true if renewal notification exists, false otherwise

### 2. Service Layer Enhancements
**File: `internal/service/subscription.go`**

#### New Methods Added:
- **`HandleAlreadyActiveSubscription(msisdn string, product *domain.Product, entryChannel string) error`**
  - Main handler for already active subscription scenarios
  - Orchestrates the entire workflow for handling existing subscriptions
  - Includes comprehensive logging for debugging and monitoring

- **`SendRenewalRequest(msisdn string, product *domain.Product, entryChannel string) error`**
  - Sends renewal MT requests to the TIMWE API
  - Uses 'RENEW' keyword for renewal operations
  - Creates and saves renewal notification records
  - Includes proper error handling and logging

#### Modified Methods:
- **`ProcessOptin(req *domain.OptinRequest) error`**
  - Enhanced to call `HandleAlreadyActiveSubscription` when `SubscriptionResultOptinAlreadyActive` is detected
  - Maintains existing staff type checking functionality
  - Preserves all existing error handling and logging

### 3. Interface Refactoring
**File: `internal/repository/subscription.interface.go` (New)**

#### New Interface:
- **`SubscriptionRepositoryInterface`**
  - Defines the contract for subscription repository operations
  - Enables proper mocking in tests
  - Includes all existing and new repository methods

#### Updated Service:
- **`SubscriptionService`** now uses `SubscriptionRepositoryInterface` instead of concrete type
- **`NewSubscriptionService`** constructor updated to accept interface
- Enables better testability and dependency injection

### 4. Testing Enhancements
**File: `internal/service/subscription_test.go`**

#### New Test Methods:
- **`TestHandleAlreadyActiveSubscription`**
  - Tests the core logic for handling already active subscriptions
  - Includes test cases for:
    - Subscription exists, renewal exists (should skip)
    - Subscription check error handling
    - Renewal check error handling
  - Uses mock implementations to avoid external dependencies

#### Mock Implementations:
- **`MockSubscriptionRepository`**
  - Implements `SubscriptionRepositoryInterface`
  - Configurable behavior for testing different scenarios
  - Includes all required methods with mock responses

## Implementation Details

### Database Queries

#### Subscription Existence Check:
```sql
SELECT COUNT(*) 
FROM subscriptions 
WHERE user_identifier = $1 AND product_id = $2 AND status = 'active'
```

#### Renewal Notification Check:
```sql
SELECT COUNT(*) 
FROM notifications 
WHERE msisdn = $1 
AND product_id = $2 
AND type = 'RENEWAL' 
AND created_at >= $3
```

### Renewal Request Format
When sending renewal requests, the system uses:
- **SubKeyword**: "RENEW" (instead of "SUB" for new subscriptions)
- **Context**: "Renewal"
- **MessageType**: "Renewal"
- **Type**: "RENEWAL"
- **Tags**: ["renewal", "subscription"]

### Error Handling Strategy
1. **Database Errors**: Properly logged and returned with context
2. **API Errors**: Handled through existing circuit breaker and retry mechanisms
3. **Validation Errors**: Comprehensive input validation with clear error messages
4. **Graceful Degradation**: System continues processing other products if one fails

### Logging Strategy
- **Info Level**: Normal workflow steps, successful operations
- **Warn Level**: Non-critical issues, expected failures
- **Error Level**: Critical failures, database errors, API errors
- **Structured Logging**: All logs include relevant context (MSISDN, productId, etc.)

## Workflow Summary

### When `SubscriptionResultOptinAlreadyActive` is Received:

1. **Log the Event**: Record that an already active subscription was detected
2. **Check Database**: Query subscriptions table for existing record
3. **Insert if Missing**: Create subscription record if not found in database
4. **Check Renewal History**: Query notifications table for current month renewals
5. **Send Renewal if Needed**: Send renewal request if no renewal this month
6. **Log Results**: Record all actions taken for audit trail

### Benefits:
- **Data Consistency**: Ensures subscription records exist in local database
- **Renewal Management**: Prevents duplicate renewal requests in same month
- **Audit Trail**: Comprehensive logging for compliance and debugging
- **Error Resilience**: Graceful handling of database and API failures
- **Testability**: Interface-based design enables comprehensive testing

## Testing Results
- ✅ All existing tests pass
- ✅ New functionality tests pass
- ✅ Build successful with no compilation errors
- ✅ Interface refactoring maintains backward compatibility
- ✅ Mock implementations work correctly

## Configuration Requirements
No additional configuration is required. The implementation uses existing:
- Database connections
- TIMWE API configuration
- Logging configuration
- Circuit breaker settings

## Future Enhancements
Potential improvements for future iterations:
1. **Renewal Frequency Configuration**: Make renewal frequency configurable
2. **Notification Templates**: Support for different renewal message templates
3. **Analytics**: Track renewal success rates and patterns
4. **Retry Logic**: Enhanced retry mechanisms for failed renewal requests
5. **Metrics**: Prometheus metrics for monitoring renewal operations 