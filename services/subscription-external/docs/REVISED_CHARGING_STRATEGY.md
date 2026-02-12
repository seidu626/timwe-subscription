# REVISED: Strategy for Handling 25M+ Subscriptions with Unknown Charging Status

## Problem Statement Clarification

The core issue is that we have 25,169,944 subscriptions that reportedly failed charging, but:
- ❌ Cannot identify them from `invalid_msisdn_logs` table
- ❌ Cannot identify them from `notifications` table  
- ❌ TIMWE API responses don't indicate charging success/failure
- ❌ Charging happens asynchronously in TIMWE's backend
- ❌ We only get status like `OPTIN_ACTIVE_WAIT_CHARGING`, `OPTIN_ALREADY_ACTIVE`

## Critical Questions That Need Answers

1. **How was the 25,169,944 number determined?**
   - Was this from TIMWE's internal reports?
   - From telco billing reconciliation?
   - From revenue analysis?

2. **What data is available to identify these subscriptions?**
   - Does TIMWE have a report/export of failed subscriptions?
   - Can the telco provide CDRs (Call Detail Records) showing charging failures?
   - Are there billing system logs we can access?

3. **What constitutes a "charging failure"?**
   - Insufficient balance?
   - Technical failure?
   - Subscription in grace period?
   - All of the above?

## Revised Approaches Based on Available Data

### Approach A: If TIMWE Can Provide Failed MSISDN List

If TIMWE can export the list of 25M failed subscriptions:

```bash
# Expected format: CSV with MSISDN, ProductID, FailureReason, Date
# Example:
# 233244123456,8509,INSUFFICIENT_BALANCE,2024-12-15
# 233244123457,14392,CHARGING_TIMEOUT,2024-12-16
```

**Implementation**:
1. Import the list into a staging table
2. Process in batches using existing resubscribe logic
3. Track processing to avoid duplicates

```sql
-- Create staging table for TIMWE data
CREATE TABLE IF NOT EXISTS charging_failed_staging (
    id SERIAL PRIMARY KEY,
    msisdn VARCHAR(15) NOT NULL,
    product_id INTEGER,
    failure_reason VARCHAR(100),
    failure_date DATE,
    processing_status VARCHAR(50) DEFAULT 'pending',
    processed_at TIMESTAMP,
    batch_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_staging_status (processing_status),
    INDEX idx_staging_msisdn_product (msisdn, product_id)
);
```

### Approach B: If We Need to Query TIMWE API for Status

If we need to check each subscription's status via TIMWE API:

```go
// internal/service/timwe_status_checker.go
package service

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type TIMWEStatusChecker struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

type SubscriptionStatus struct {
    MSISDN         string    `json:"msisdn"`
    ProductID      string    `json:"product_id"`
    Status         string    `json:"status"`
    ChargingStatus string    `json:"charging_status"`
    LastCharged    time.Time `json:"last_charged"`
    NextCharge     time.Time `json:"next_charge"`
    FailureReason  string    `json:"failure_reason"`
}

func (t *TIMWEStatusChecker) CheckSubscriptionStatus(msisdn string, productID string) (*SubscriptionStatus, error) {
    // Call TIMWE subscription status API
    url := fmt.Sprintf("%s/api/v1/subscription/status?msisdn=%s&product_id=%s", 
        t.baseURL, msisdn, productID)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Authorization", "Bearer " + t.apiKey)
    
    resp, err := t.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var status SubscriptionStatus
    if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
        return nil, err
    }
    
    return &status, nil
}

func (t *TIMWEStatusChecker) IdentifyChargingFailures(subscriptions []Subscription) []FailedSubscription {
    var failed []FailedSubscription
    
    for _, sub := range subscriptions {
        status, err := t.CheckSubscriptionStatus(sub.MSISDN, sub.ProductID)
        if err != nil {
            continue
        }
        
        // Check if charging failed based on TIMWE's response
        if status.ChargingStatus == "FAILED" || 
           status.FailureReason != "" ||
           time.Since(status.LastCharged) > 30*24*time.Hour {
            failed = append(failed, FailedSubscription{
                MSISDN:        sub.MSISDN,
                ProductID:     sub.ProductID,
                FailureReason: status.FailureReason,
            })
        }
    }
    
    return failed
}
```

### Approach C: Infer from Subscription Age and Activity

If no direct data is available, we might need to infer failures:

```sql
-- Identify potentially failed subscriptions based on patterns
WITH potentially_failed AS (
    SELECT 
        s.id,
        s.user_identifier as msisdn,
        s.product_id,
        s.created_at,
        s.status,
        -- Calculate days since creation
        EXTRACT(DAY FROM NOW() - s.created_at) as days_old,
        -- Check for recent activity (assuming we track this)
        s.last_activity_at
    FROM subscriptions s
    WHERE 
        -- Active subscriptions older than 30 days
        s.status = 'active'
        AND s.created_at < NOW() - INTERVAL '30 days'
        -- No recent activity (might indicate charging failure)
        AND (s.last_activity_at IS NULL 
             OR s.last_activity_at < NOW() - INTERVAL '30 days')
)
SELECT 
    COUNT(*) as total_potentially_failed,
    MIN(created_at) as oldest_subscription,
    MAX(created_at) as newest_subscription
FROM potentially_failed;
```

### Approach D: Process Based on Business Rules

If the 25M number comes from business analysis (e.g., subscriptions not generating revenue):

```python
# Script to identify non-revenue generating subscriptions
import pandas as pd
from datetime import datetime, timedelta

def identify_non_revenue_subscriptions():
    """
    Identify subscriptions not generating revenue
    based on business rules
    """
    
    # Load subscription data
    subscriptions = pd.read_sql("""
        SELECT 
            user_identifier as msisdn,
            product_id,
            created_at,
            status
        FROM subscriptions
        WHERE status = 'active'
    """, connection)
    
    # Load revenue data (if available)
    revenue = pd.read_sql("""
        SELECT 
            msisdn,
            product_id,
            MAX(transaction_date) as last_payment
        FROM billing_transactions
        WHERE transaction_type = 'CHARGE'
        AND status = 'SUCCESS'
        GROUP BY msisdn, product_id
    """, connection)
    
    # Merge to find subscriptions without recent payments
    merged = subscriptions.merge(
        revenue, 
        on=['msisdn', 'product_id'], 
        how='left'
    )
    
    # Identify failed based on business rules
    threshold_date = datetime.now() - timedelta(days=30)
    failed = merged[
        (merged['last_payment'].isna()) |  # Never charged
        (merged['last_payment'] < threshold_date)  # Not charged recently
    ]
    
    return failed
```

## Recommended Implementation Path

### Step 1: Clarify Data Source (IMMEDIATE)

```bash
# Questions to answer:
1. How was 25,169,944 determined?
2. What system has the actual charging status?
3. What APIs/reports are available?
4. What defines a "charging failure"?
5. What time period does this cover?
```

### Step 2: Create Data Import Pipeline

Based on the data source, create appropriate import mechanism:

```go
// internal/service/failed_subscription_importer.go
package service

type FailedSubscriptionImporter interface {
    // Import from CSV file
    ImportFromCSV(filepath string) error
    
    // Import from TIMWE API
    ImportFromTIMWEAPI(startDate, endDate time.Time) error
    
    // Import from database query
    ImportFromQuery(query string) error
    
    // Get import statistics
    GetStats() ImportStats
}

type ImportStats struct {
    TotalRecords      int
    ImportedRecords   int
    DuplicateRecords  int
    ErrorRecords      int
    ProcessingTime    time.Duration
}
```

### Step 3: Modified Processing Logic

Since we can't detect charging failures automatically, modify the approach:

```go
// Process subscriptions from imported list
func (s *SubscriptionService) ProcessImportedFailures(batchID string) error {
    // 1. Read from staging table
    failures, err := s.repo.GetUnprocessedFailures(batchID)
    if err != nil {
        return err
    }
    
    // 2. Group by product for efficiency
    grouped := groupByProduct(failures)
    
    // 3. Process each group
    for productID, msisdns := range grouped {
        // Unsubscribe all in batch
        if err := s.batchUnsubscribe(msisdns, productID); err != nil {
            s.logger.Error("Batch unsubscribe failed", 
                zap.String("productID", productID),
                zap.Error(err))
            continue
        }
        
        // Wait for unsubscribe to process
        time.Sleep(5 * time.Second)
        
        // Resubscribe all in batch
        if err := s.batchResubscribe(msisdns, productID); err != nil {
            s.logger.Error("Batch resubscribe failed",
                zap.String("productID", productID),
                zap.Error(err))
            continue
        }
        
        // Mark as processed in staging table
        s.repo.MarkAsProcessed(msisdns, productID, batchID)
    }
    
    return nil
}
```

## Alternative: Blanket Resubscription Approach

If we cannot identify specific failures, consider resubscribing ALL active subscriptions:

### Pros:
- No need to identify failures
- Ensures all subscriptions are refreshed
- TIMWE handles deduplication

### Cons:
- Unnecessary processing load
- Risk of disrupting working subscriptions
- Higher API usage

```sql
-- Get all active subscriptions for blanket resubscription
SELECT 
    user_identifier as msisdn,
    product_id,
    entry_channel
FROM subscriptions
WHERE status = 'active'
ORDER BY created_at DESC
LIMIT 1000000 OFFSET 0;  -- Process in 1M chunks
```

## Critical Next Steps

1. **URGENT: Identify Data Source**
   - Contact TIMWE for charging failure reports
   - Check if billing system has CDRs
   - Verify how 25M number was calculated

2. **Design Import Process**
   - Create staging tables
   - Build import scripts
   - Validate data quality

3. **Test with Small Sample**
   - Get 1000 confirmed failures
   - Test resubscription process
   - Verify charging resumes

4. **Scale Gradually**
   - Process in manageable batches
   - Monitor success rates
   - Adjust approach based on results

## Risk Assessment

### High Risk
- Processing without knowing actual failures
- Disrupting working subscriptions
- Overwhelming TIMWE API

### Medium Risk
- Duplicate processing
- Incorrect failure identification
- Performance impact

### Low Risk
- Database storage
- Monitoring overhead
- Rollback complexity

## Recommendation

**DO NOT PROCEED** with blanket resubscription until:
1. ✅ Source of 25M number is verified
2. ✅ Actual failed subscription list is obtained
3. ✅ Test with confirmed failures succeeds
4. ✅ TIMWE confirms approach is safe

The safest approach is to obtain the actual list of failed subscriptions from TIMWE or the billing system rather than trying to infer or guess which subscriptions have charging issues.
