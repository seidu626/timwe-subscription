# Task.md - Subscription Renewal via Opt-Out/Opt-In Strategy

## Current Status: ✅ Implementation Guide Complete - Ready for Development

### Notes
- 2026-01-21: Updated KrakenD admin routing/CORS/header passthrough for WebSPA admin access.
- 2026-01-21: Added KrakenD admin input_headers + lowercase token passthrough.
- 2026-01-21: Investigating KrakenD admin 401 for `/v1/admin/campaigns`.

---

## NEW TASK: Extract First 6 Digits from user_identifier

### Task: Create PostgreSQL script to select distinct first 6 numbers from user_identifier
### Status: 🔄 In Progress
### Created: 2025-01-27

#### Requirements:
- Extract first 6 digits from user_identifier field
- Get distinct values only
- From subscriptions table

#### Implementation:
- Script created: `extract_first_6_digits.sql`
- Includes multiple query variations for different use cases

---

## Issue: TIMWE Charging Endpoint Not Working

### Problem Analysis
1. TIMWE's charging endpoint is non-functional
2. Creating notifications doesn't trigger billing
3. Need alternative approach to force billing attempts

### Solution: Opt-Out/Opt-In Cycle
**Strategy**: Unsubscribe and immediately resubscribe users to trigger TIMWE's internal billing system

---

## Implementation Strategy: Forced Renewal via Resubscription

### Core Concept
```
1. Identify subscriptions needing renewal
2. Send UNSUBSCRIBE request (opt-out)
3. Wait for confirmation
4. Send SUBSCRIBE request (opt-in) 
5. This triggers TIMWE's first-charge billing
6. Track success/failure for retry
```

### 1. ✅ Modified Renewal Logic

```go
// Instead of charging, use opt-out/opt-in cycle
func (s *SubscriptionService) SendRenewalRequest(msisdn string, product *domain.Product, entryChannel string) error {
    // Step 1: Opt-Out (Unsubscribe)
    if err := s.OptOutSubscription(msisdn, product, "RENEWAL_CYCLE"); err != nil {
        return fmt.Errorf("failed to opt-out for renewal: %w", err)
    }
    
    // Step 2: Wait briefly for TIMWE to process
    time.Sleep(2 * time.Second)
    
    // Step 3: Opt-In (Resubscribe) - triggers billing
    if err := s.OptInSubscription(msisdn, product, entryChannel); err != nil {
        return fmt.Errorf("failed to opt-in for renewal: %w", err)
    }
    
    // Step 4: Track renewal attempt
    s.TrackRenewalAttempt(msisdn, product.ProductId, "OPTIN_RENEWAL")
    
    return nil
}
```

### 2. 🔄 Churn Policy Implementation

```go
type ChurnPolicy struct {
    MaxDaysWithoutPayment int     // Default: 7 days
    MaxRenewalAttempts    int     // Default: 3 attempts
    RetryIntervalHours    int     // Default: 24 hours
    GracePeriodDays       int     // Default: 2 days
}

// Evaluate if subscription should be churned
func (s *SubscriptionService) EvaluateChurnPolicy(msisdn string, productId string) ChurnAction {
    last