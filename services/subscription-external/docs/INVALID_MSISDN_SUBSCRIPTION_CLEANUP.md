# INVALID_MSISDN Subscription Cleanup Implementation

## Overview

This document describes the implementation of automatic subscription cleanup when an `INVALID_MSISDN` error is encountered during MT (Mobile Terminated) operations.

## Problem Statement

When the TIMWE API returns an `INVALID_MSISDN` error, it indicates that the MSISDN is no longer valid or accessible. Previously, the system would only log this error to the `invalid_msisdn_logs` table but would not clean up any existing subscriptions for that MSISDN, potentially leaving orphaned subscription records.

## Solution

The system now automatically:
1. Detects `INVALID_MSISDN` responses from the TIMWE API
2. Logs the error to the `invalid_msisdn_logs` table (existing functionality)
3. **NEW**: Checks if any active subscriptions exist for the invalid MSISDN and product
4. **ENHANCED**: **Completely deletes** any found subscriptions (instead of just deactivating them)

## Implementation Details

### 1. Repository Interface Update

Added new methods to `SubscriptionRepositoryInterface`:

```go
// FindAndRemoveSubscription finds and removes/deactivates a subscription for the given MSISDN and product
FindAndRemoveSubscription(msisdn string, productId int) error

// DeleteSubscriptionRecord completely removes a subscription record for the given MSISDN and product
DeleteSubscriptionRecord(msisdn string, productId int) error
```

### 2. Repository Implementation

Implemented in `postgres.go`:

```go
// FindAndRemoveSubscription - Deactivates subscriptions (sets status to 'inactive' and end_date)
func (r *SubscriptionRepository) FindAndRemoveSubscription(msisdn string, productId int) error {
    // Check if subscription exists
    // If found, update status to 'inactive' and set end_date
    // Log the operation for audit purposes
}

// DeleteSubscriptionRecord - Completely removes subscription records
func (r *SubscriptionRepository) DeleteSubscriptionRecord(msisdn string, productId int) error {
    // Check if subscription exists
    // If found, completely delete the record from the database
    // Log the operation for audit purposes
}
```

### 3. Service Layer Integration

Updated `detectAndLogInvalidMSISDN` method in `subscription.go`:

```go
// If INVALID_MSISDN is detected, log it and clean up subscriptions
if isInvalidMSISDN {
    // ... existing logging logic ...
    
    // ENHANCED: Completely remove any existing subscriptions for this invalid MSISDN and product
    if err := s.repo.DeleteSubscriptionRecord(mtReq.UserIdentifier, mtReq.ProductID); err != nil {
        s.logger.Error("Failed to delete subscription record for invalid MSISDN", ...)
    } else {
        s.logger.Info("Successfully deleted subscription record for invalid MSISDN", ...)
    }
}
```

## Database Changes

The implementation provides two approaches:

- **`FindAndRemoveSubscription`**: Updates `subscriptions` table to set `status = 'inactive'` and `end_date = NOW()`
- **`DeleteSubscriptionRecord`**: **Completely removes** records from the `subscriptions` table
- **`invalid_msisdn_logs`** table: Continues to log all INVALID_MSISDN occurrences

## Current Implementation

**The system now uses `DeleteSubscriptionRecord` by default**, which means:
- Invalid MSISDN subscriptions are **completely removed** from the database
- No orphaned or inactive subscription records remain
- Cleaner database state with only valid, active subscriptions

## Error Handling

- **Non-blocking**: Subscription cleanup failures don't prevent the main INVALID_MSISDN logging
- **Comprehensive logging**: All operations are logged with appropriate log levels
- **Graceful degradation**: If cleanup fails, the error is logged but doesn't break the main flow

## Logging

The system now provides detailed logging for subscription deletion operations:

```
INFO: Successfully deleted subscription record for invalid MSISDN {"msisdn": "233261344927", "productId": 8509}
ERROR: Failed to delete subscription record for invalid MSISDN {"msisdn": "233261344927", "productId": 8509, "error": "database error"}
```

## Testing

- Added `TestDeleteSubscriptionRecordForInvalidMSISDN` test case
- Updated mock repository to satisfy interface requirements
- All existing tests continue to pass
- New deletion functionality is thoroughly tested

## Benefits

1. **Data Consistency**: Prevents orphaned subscription records for invalid MSISDNs
2. **Complete Cleanup**: **Completely removes** invalid subscriptions instead of just marking them inactive
3. **Automatic Cleanup**: No manual intervention required
4. **Audit Trail**: Complete logging of all cleanup operations
5. **Non-disruptive**: Fails gracefully without affecting main functionality
6. **Performance**: Efficient database operations with proper indexing
7. **Flexibility**: Two cleanup approaches available (deactivate vs. delete)

## Usage

The functionality is automatically triggered whenever an `INVALID_MSISDN` response is detected in:

- MT responses from TIMWE API
- Subscription renewal requests
- Opt-in operations
- Any other MT-based operations

## Monitoring

Monitor the following log patterns to track subscription cleanup:

- `"INVALID_MSISDN detected, logging for reference and cleaning up subscriptions"`
- `"Successfully deleted subscription record for invalid MSISDN"`
- `"Failed to delete subscription record for invalid MSISDN"`

## Future Enhancements

Potential improvements for future versions:

1. **Configurable Cleanup Strategy**: Allow choosing between deactivation and deletion via configuration
2. **Batch Processing**: Handle multiple INVALID_MSISDN responses in a single cleanup operation
3. **Metrics**: Add Prometheus metrics for tracking cleanup operations
4. **Retry Logic**: Implement retry mechanism for failed cleanup operations
5. **Notification**: Send alerts for cleanup failures above a certain threshold
6. **Soft Delete**: Option to move deleted records to an archive table instead of permanent deletion 