# INVALID_MSISDN Response Logging

## Overview
This document describes the implementation of automatic logging for INVALID_MSISDN responses from the TIMWE API. When the system receives an INVALID_MSISDN response, it automatically logs the details to a dedicated database table for reference and analysis.

## Problem Statement
When the TIMWE API returns an INVALID_MSISDN response, it indicates that the provided mobile number is invalid or not recognized. These responses need to be tracked for:
- **Audit purposes**: Understanding which MSISDNs are being rejected
- **Data quality analysis**: Identifying patterns in invalid MSISDNs
- **Troubleshooting**: Debugging issues with MSISDN validation
- **Compliance**: Maintaining records of failed attempts

## Solution Implementation

### 1. Database Schema
A new table `invalid_msisdn_logs` has been created to store INVALID_MSISDN responses:

```sql
CREATE TABLE IF NOT EXISTS invalid_msisdn_logs (
    id SERIAL PRIMARY KEY,
    msisdn VARCHAR(15) NOT NULL,
    product_id INTEGER,
    pricepoint_id INTEGER,
    partner_role_id INTEGER,
    entry_channel VARCHAR(50),
    request_id VARCHAR(100),
    response_code VARCHAR(50),
    response_message TEXT,
    subscription_result VARCHAR(100),
    subscription_error TEXT,
    external_tx_id VARCHAR(255),
    transaction_id VARCHAR(255),
    request_data JSONB,
    response_data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Indexes for Performance:**
- `idx_invalid_msisdn_logs_msisdn` - For MSISDN lookups
- `idx_invalid_msisdn_logs_product_id` - For product-based queries
- `idx_invalid_msisdn_logs_created_at` - For time-based queries
- `idx_invalid_msisdn_logs_response_code` - For response code filtering

### 2. Domain Model
New domain model `InvalidMSISDNLog` has been added to `internal/domain/subscription.go`:

```go
type InvalidMSISDNLog struct {
    ID                int                    `json:"id"`
    MSISDN            string                 `json:"msisdn"`
    ProductID         *int                   `json:"productId,omitempty"`
    PricepointID      *int                   `json:"pricepointId,omitempty"`
    PartnerRoleID     *int                   `json:"partnerRoleId,omitempty"`
    EntryChannel      string                 `json:"entryChannel,omitempty"`
    RequestID         string                 `json:"requestId,omitempty"`
    ResponseCode      string                 `json:"responseCode,omitempty"`
    ResponseMessage   string                 `json:"responseMessage,omitempty"`
    SubscriptionResult string                `json:"subscriptionResult,omitempty"`
    SubscriptionError string                 `json:"subscriptionError,omitempty"`
    ExternalTxID      string                 `json:"externalTxId,omitempty"`
    TransactionID     string                 `json:"transactionId,omitempty"`
    RequestData       map[string]interface{} `json:"requestData,omitempty"`
    ResponseData      map[string]interface{} `json:"responseData,omitempty"`
    CreatedAt         time.Time              `json:"createdAt"`
}
```

### 3. Repository Layer
New method added to `SubscriptionRepositoryInterface`:

```go
CreateInvalidMSISDNLog(log *domain.InvalidMSISDNLog) error
```

Implementation in `internal/repository/postgres.go` handles:
- JSON serialization of request/response data
- Database insertion with proper error handling
- Logging of successful insertions

### 4. Service Layer
New helper method `detectAndLogInvalidMSISDN` in `SubscriptionService`:

**Detection Logic:**
- Checks main response code for `INVALID_MSISDN`
- Checks `subscriptionResult` field for `INVALID_MSISDN`
- Checks `subscriptionError` field for `Invalid MSISDN`

**Logging Features:**
- Non-blocking operation (doesn't affect main flow)
- Comprehensive data capture including request and response details
- Structured logging with appropriate log levels
- Error handling for database failures

### 5. Integration Points
The detection is integrated into the `validateMTResponse` method:
- Called before any error processing
- Non-blocking - doesn't affect response validation
- Captures all relevant context for analysis

## Usage Examples

### Detecting INVALID_MSISDN in Response Code
```json
{
  "code": "INVALID_MSISDN",
  "message": "Invalid MSISDN",
  "responseData": {
    "subscriptionResult": "null",
    "subscriptionError": "null"
  }
}
```

### Detecting INVALID_MSISDN in Subscription Result
```json
{
  "code": "SUCCESS",
  "message": "Success",
  "responseData": {
    "subscriptionResult": "INVALID_MSISDN",
    "subscriptionError": "null"
  }
}
```

### Detecting INVALID_MSISDN in Subscription Error
```json
{
  "code": "SUCCESS",
  "message": "Success",
  "responseData": {
    "subscriptionResult": "null",
    "subscriptionError": "Invalid MSISDN"
  }
}
```

## Benefits

### 1. **Comprehensive Tracking**
- All INVALID_MSISDN responses are automatically logged
- Essential metadata preserved; request/response payloads omitted to reduce storage volume
- Timestamp tracking for temporal analysis

### 2. **Non-Intrusive**
- Logging doesn't affect the main application flow
- Database failures don't impact subscription processing
- Minimal performance overhead

### 3. **Rich Data Capture**
- Complete request context (product, channel, etc.)
- Full response details for debugging
- Structured data for easy querying

### 4. **Audit Compliance**
- Permanent record of all INVALID_MSISDN attempts
- Traceable to specific transactions and requests
- Supports regulatory and compliance requirements

## Query Examples

### Find all INVALID_MSISDN logs for a specific MSISDN
```sql
SELECT * FROM invalid_msisdn_logs 
WHERE msisdn = '233123456789' 
ORDER BY created_at DESC;
```

### Find INVALID_MSISDN logs by date range
```sql
SELECT * FROM invalid_msisdn_logs 
WHERE created_at BETWEEN '2024-01-01' AND '2024-01-31'
ORDER BY created_at DESC;
```

### Find INVALID_MSISDN logs by product
```sql
SELECT * FROM invalid_msisdn_logs 
WHERE product_id = 123 
ORDER BY created_at DESC;
```

### Count INVALID_MSISDN responses by day
```sql
SELECT DATE(created_at) as date, COUNT(*) as count 
FROM invalid_msisdn_logs 
GROUP BY DATE(created_at) 
ORDER BY date DESC;
```

## Testing
Unit tests have been added to verify:
- Detection of INVALID_MSISDN in different response fields
- Proper logging of detected responses
- Non-interference with valid responses
- Error handling for database failures

## Future Enhancements
Potential improvements for the future:
1. **Analytics Dashboard**: Web interface for viewing INVALID_MSISDN trends
2. **Alerting**: Notifications when INVALID_MSISDN rates exceed thresholds
3. **Pattern Analysis**: Machine learning to identify common patterns in invalid MSISDNs
4. **Export Functionality**: Ability to export logs for external analysis
5. **Retention Policies**: Automatic cleanup of old logs based on configurable retention periods 