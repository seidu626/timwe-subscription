# OPTIN_ACTIVE_WAIT_CHARGING Status Handling Implementation

## Overview
This document describes the implementation of enhanced handling for subscription responses in the subscription-external service. The system now implements a strict success/error classification:

**Success Codes (Only 2):**
- `OPTIN_ALREADY_ACTIVE`: User already has active subscription
- `OPTIN_ACTIVE_WAIT_CHARGING`: Subscription active, waiting for charging

**Error Codes (All Others):**
- `OPTIN_CONFIG_NOT_FOUND`: Configuration error
- `INVALID_MSISDN`: Invalid mobile number
- `INVALID_ENTRY_FLOW_CHANNEL`: Invalid entry flow channel
- Any other subscription result not in the success list

## Problem Statement

### Success vs Error Classification
The TIMWE API can return various subscription results, but only two should be treated as successful outcomes:
- `OPTIN_ALREADY_ACTIVE`: Indicates user already has an active subscription
- `OPTIN_ACTIVE_WAIT_CHARGING`: Indicates subscription is active and waiting for charging confirmation

All other subscription results should be treated as errors, including:
- `OPTIN_CONFIG_NOT_FOUND`: Missing or invalid product configuration
- `INVALID_MSISDN`: Invalid mobile number format or validation failure
- `INVALID_ENTRY_FLOW_CHANNEL`: Invalid entry channel configuration
- Any other unexpected subscription results

### Main Response Error Indicators
The TIMWE API can also indicate errors at the main response level:
- `"inError": true` - Indicates the main response is an error
- `"code": "INVALID_MSISDN"` - Error code in the main response
- `"message": "INVALID_MSISDN"` - Error message in the main response

**Example of Main Response Error:**
```json
{
  "responseData": {
    "externalTxId": "73cb3af2-e97b-4704-a260-95e12ef8689f",
    "subscriptionError": "null",
    "subscriptionResult": "null"
  },
  "message": "INVALID_MSISDN",
  "inError": true,
  "requestId": "76985:1754566952002",
  "code": "INVALID_MSISDN"
}
```

In this case, even though `subscriptionResult` and `subscriptionError` are `"null"`, the main response indicates an error through `inError: true` and `code: "INVALID_MSISDN"`.

## Changes Made

### 1. Enhanced Constants and Error Handling

#### New Constants Added:
```go
// Subscription result codes from TIMWE API
const (
    SubscriptionResultOptinAlreadyActive = "OPTIN_ALREADY_ACTIVE"           // SUCCESS
    SubscriptionResultOptinActiveWaitCharging = "OPTIN_ACTIVE_WAIT_CHARGING" // SUCCESS
    SubscriptionResultOptinConfigNotFound = "OPTIN_CONFIG_NOT_FOUND"        // ERROR
    SubscriptionResultInvalidMsisdn = "INVALID_MSISDN"                      // ERROR
    SubscriptionResultInvalidEntryFlowChannel = "INVALID_ENTRY_FLOW_CHANNEL" // ERROR
    SubscriptionResultNull               = "null"
)

// Subscription error messages
const (
    SubscriptionErrorAlreadyActive = "Already Active"
    SubscriptionErrorActiveWaitCharging = "Active and Wait Charging"
    SubscriptionErrorOptinConfigNotFound = "Optin configuration not found!"
    SubscriptionErrorInvalidMsisdn = "Invalid MSISDN"
    SubscriptionErrorInvalidEntryFlowChannel = "Invalid Entry Flow Channel"
)
```

**Note:** Only `OPTIN_ALREADY_ACTIVE` and `OPTIN_ACTIVE_WAIT_CHARGING` are treated as success codes. All other subscription results are treated as errors.

### 2. Enhanced Response Validation

Updated `validateMTResponse()` method to implement comprehensive error checking:

1. **Main Response Error Check**: First checks `response.InError` and `response.Code` fields
2. **Subscription Result Check**: Then validates `subscriptionResult` field with strict success/error classification
3. **Subscription Error Check**: Validates `subscriptionError` field for additional error information
4. **Transaction ID Validation**: Ensures successful responses have valid transaction IDs

**New Validation Order:**
```go
// 1. Check main response error indicators
if response.InError {
    return MTResponseError with response.Code and response.Message
}

// 2. Check response code
if response.Code != "SUCCESS" {
    return MTResponseError with response.Code and response.Message
}

// 3. Check subscription result (only if main response is successful)
if subscriptionResult != "OPTIN_ALREADY_ACTIVE" && subscriptionResult != "OPTIN_ACTIVE_WAIT_CHARGING" {
    return MTResponseError with subscription result details
}
```

### 3. Circuit Breaker Improvements for Batch Operations

**Problem:** The original circuit breaker was too aggressive for batch operations:
- Tripped after only 3 consecutive failures
- Caused all subsequent valid requests to fail when one invalid MSISDN was encountered
- 60-second timeout was too long for batch recovery

**Solution:** Enhanced circuit breaker configuration:
```go
cbSettings := gobreaker.Settings{
    Name:        "TIMWE API Circuit Breaker",
    MaxRequests: 10,               // Increased from 5 to 10
    Timeout:     30 * time.Second, // Reduced from 60 to 30 seconds
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        // Only trip if we have at least 5 total requests and 70% failure rate
        return counts.Requests >= 5 && 
               float64(counts.ConsecutiveFailures)/float64(counts.Requests) >= 0.7
    },
}
```

**Benefits:**
- **More Resilient**: Requires 70% failure rate instead of just 3 consecutive failures
- **Batch-Friendly**: Allows more requests before tripping
- **Faster Recovery**: 30-second timeout for quicker recovery
- **Better Error Handling**: Distinguishes between systematic failures and individual invalid MSISDNs

#### Updated `isSubscriptionAlreadyActive` Method:
- Now includes `OPTIN_ACTIVE_WAIT_CHARGING` in the check
- Excludes `OPTIN_CONFIG_NOT_FOUND` (error condition)
- Returns true for both already active and waiting for charging subscriptions

#### New Helper Method:
```go
// Helper method to check if subscription is waiting for charging
func (s *SubscriptionService) isSubscriptionWaitingForCharging(response *domain.MTResponse) bool
```

### 3. Enhanced ProcessOptin Method
**File: `internal/service/subscription.go`**

#### New Logic Added:
- Checks for `OPTIN_ACTIVE_WAIT_CHARGING` status after checking for already active subscriptions
- Calls `HandleWaitingForChargingSubscription` for proper handling
- Skips normal database save but doesn't treat as error
- `OPTIN_CONFIG_NOT_FOUND`

## Benefits:
- **Proper Status Recognition**: Correctly identifies success vs error conditions
- **Configuration Error Handling**: Properly handles missing configuration errors
- **Data Consistency**: Ensures subscription records exist in local database
- **Status Tracking**: Creates notifications for charging status monitoring
- **Audit Trail**: Comprehensive logging for compliance and debugging
- **Error Resilience**: Graceful handling of database and API failures
- **Future Verification**: Provides mechanism to check charging status later

## Error Handling Strategy

### Success Codes (Only 2):
- `OPTIN_ALREADY_ACTIVE`: User already has active subscription
- `OPTIN_ACTIVE_WAIT_CHARGING`: Subscription active, waiting for charging

### Error Codes (All Others):
- `OPTIN_CONFIG_NOT_FOUND`: Configuration error, should be treated as failure
- `INVALID_MSISDN`: Validation error for invalid mobile numbers, should be treated as failure
- `INVALID_ENTRY_FLOW_CHANNEL`: Invalid entry channel configuration, should be treated as failure
- Any other subscription result not in the success list

### Implementation Logic:
1. **Strict Classification**: Only the two success codes are allowed to proceed
2. **Immediate Error Return**: Any other subscription result immediately returns an error
3. **Comprehensive Logging**: All subscription results are logged for audit purposes
4. **Consistent Error Format**: All errors return `MTResponseError` with appropriate details

### ✅ **Success Response Format:**

When `OPTIN_ALREADY_ACTIVE` or `OPTIN_ACTIVE_WAIT_CHARGING` occurs, the system returns:

**For OptinHandler (Single Subscription):**
```json
{
  "status": "success",
  "message": "Successfully processed subscription"
}
```

**For BatchOptinHandler (Batch Subscriptions):**
```json
{
  "total": 100,
  "successful": 100,
  "failed": 0
}
```

### ✅ **Error Response Format:**

When `INVALID_ENTRY_FLOW_CHANNEL` or any other error occurs, the system returns:

**For OptinHandler (Single Subscription):**
```json
{
  "status": "error",
  "message": "subscription error: INVALID_ENTRY_FLOW_CHANNEL",
  "code": "SUCCESS",
  "details": {
    "transactionId": "5a6c709d-72f7-11f0-9fd1-005056b10ac2",
    "externalTxId": "tx-12999449",
    "subscriptionResult": "INVALID_ENTRY_FLOW_CHANNEL",
    "subscriptionError": "Invalid Entry Flow Channel"
  }
}
```

**For BatchOptinHandler (Batch Subscriptions):**
```json
{
  "total": 100,
  "successful": 95,
  "failed": 5,
  "errorDetails": {
    "transactionId": "5a6c709d-72f7-11f0-9fd1-005056b10ac2",
    "externalTxId": "tx-12999449",
    "subscriptionResult": "INVALID_ENTRY_FLOW_CHANNEL",
    "subscriptionError": "Invalid Entry Flow Channel"
  }
}
```

**Note:** The batch handler includes error details for the first error encountered, while individual errors are logged but not all included in the response payload.