# Opt-Out/Opt-In Renewal Implementation Guide

## Overview
Since TIMWE's charging endpoint is not functional, this implementation uses an opt-out/opt-in cycle to trigger TIMWE's internal billing system for subscription renewals.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Identify   │────▶│   Opt-Out    │────▶│   Wait       │
│   Due Subs   │     │  (UNSUB)     │     │   (2-3s)     │
└──────────────┘     └──────────────┘     └──────────────┘
                                                  │
                                                  ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Monitor    │◀────│   Opt-In     │◀────│   Process    │
│   Billing    │     │   (SUB)      │     │              │
└──────────────┘     └──────────────┘     └──────────────┘
```

## Complete Implementation

### 1. Core Service Updates

```go
// subscription.go - Updated renewal logic
package service

import (
    "fmt"
    "time"
    "github.com/google/uuid"
    "go.uber.org/zap"
)

// RenewalStrategy defines how renewals are processed
type RenewalStrategy string

const (
    StrategyOptOutOptIn RenewalStrategy = "opt_out_opt_in"
    StrategyDirectCharge RenewalStrategy = "direct_charge" // Not working
)

// ChurnAction defines what to do with a subscription
type ChurnAction string

const (
    ActionAttemptRenewal ChurnAction = "attempt_renewal"
    ActionChurn         ChurnAction = "churn"
    ActionNoAction      ChurnAction = "no_action"
    ActionGracePeriod   ChurnAction = "grace_period"
)

// RenewalCycle tracks an opt-out/opt-in renewal attempt
type RenewalCycle struct {
    ID             int64
    SubscriptionID int64
    MSISDN         string
    ProductID      string
    CycleNumber    int
    OptOutTime     *time.Time
    OptOutStatus   string
    OptInTime      *time.Time
    OptInStatus    string
    BillingStatus  string
    CreatedAt      time.Time
}

// ChurnPolicy defines when to churn subscriptions
type ChurnPolicy struct {
    MaxDaysWithoutPayment int  // Days before churning (default: 7)
    MaxRenewalAttempts    int  // Max renewal cycles (default: 3)
    RetryIntervalHours    int  // Hours between attempts (default: 24)
    GracePeriodDays       int  // Grace period before first attempt (default: 2)
    SafeMode              bool // Prevent accidental mass churning
}

// Updated SendRenewalRequest using opt-out/opt-in strategy
func (s *SubscriptionService) SendRenewalRequest(msisdn string, product *domain.Product, entryChannel string) error {
    s.logger.Info("Starting opt-out/opt-in renewal cycle",
        zap.String("msisdn", msisdn),
        zap.String("productId", product.ProductId))
    
    // Check if we should use the new strategy
    if s.config.Renewal.Strategy != string(StrategyOptOutOptIn) {
        return fmt.Errorf("opt-out/opt-in strategy not enabled")
    }
    
    // Create renewal cycle record
    cycle := &RenewalCycle{
        MSISDN:    msisdn,
        ProductID: product.ProductId,
        CreatedAt: time.Now(),
    }
    
    // Step 1: Opt-Out (Unsubscribe)
    optOutErr := s.OptOutForRenewal(msisdn, product, cycle)
    if optOutErr != nil {
        s.logger.Error("Opt-out failed during renewal",
            zap.String("msisdn", msisdn),
            zap.Error(optOutErr))
        cycle.OptOutStatus = "FAILED"
        s.SaveRenewalCycle(cycle)
        return fmt.Errorf("opt-out failed: %w", optOutErr)
    }
    
    cycle.OptOutStatus = "SUCCESS"
    optOutTime := time.Now()
    cycle.OptOutTime = &optOutTime
    
    // Step 2: Wait for TIMWE to process the unsubscription
    waitTime := time.Duration(s.config.Renewal.OptOutOptIn.WaitBetweenMs) * time.Millisecond
    s.logger.Info("Waiting before opt-in",
        zap.Duration("wait", waitTime))
    time.Sleep(waitTime)
    
    // Step 3: Opt-In (Resubscribe) - This triggers TIMWE's billing
    optInErr := s.OptInForRenewal(msisdn, product, entryChannel, cycle)
    if optInErr != nil {
        s.logger.Error("Opt-in failed during renewal",
            zap.String("msisdn", msisdn),
            zap.Error(optInErr))
        cycle.OptInStatus = "FAILED"
        s.SaveRenewalCycle(cycle)
        
        // Critical: User is now unsubscribed, need to handle this
        s.HandleFailedOptIn(msisdn, product, cycle)
        return fmt.Errorf("opt-in failed: %w", optInErr)
    }
    
    cycle.OptInStatus = "SUCCESS"
    optInTime := time.Now()
    cycle.OptInTime = &optInTime
    cycle.BillingStatus = "PENDING"
    
    // Save the complete renewal cycle
    if err := s.SaveRenewalCycle(cycle); err != nil {
        s.logger.Error("Failed to save renewal cycle", zap.Error(err))
    }
    
    // Track metrics
    s.metrics.RenewalAttempts.WithLabelValues("opt_out_opt_in", "success").Inc()
    
    s.logger.Info("Renewal cycle completed successfully",
        zap.String("msisdn", msisdn),
        zap.String("productId", product.ProductId),
        zap.Duration("totalTime", time.Since(cycle.CreatedAt)))
    
    return nil
}

// OptOutForRenewal handles the unsubscription part of renewal
func (s *SubscriptionService) OptOutForRenewal(msisdn string, product *domain.Product, cycle *RenewalCycle) error {
    txId := uuid.New().String()
    
    // Create UNSUBSCRIBE MT request
    mtReq := domain.MTRequest{
        ProductID:          product.ProductId,
        PricepointID:       product.PricePointId,
        UserIdentifier:     msisdn,
        UserIdentifierType: "MSISDN",
        UnsubKeyword:       "STOP",
        Context:            "Unsubscription",
        MCC:                "620",
        MNC:                "03",
        EntryChannel:       "SYSTEM_RENEWAL",
        LargeAccount:       product.ShortCode,
        MoTransactionUUID:  txId,
        SendDate:           time.Now().Format(time.RFC3339),
        Tags:               []string{"renewal", "opt_out"},
    }
    
    s.logger.Debug("Sending opt-out request",
        zap.String("msisdn", msisdn),
        zap.String("transactionId", txId))
    
    response, err := s.SendMT(mtReq, s.config.Application.TIMWE.Realm, "SYSTEM")
    if err != nil {
        return fmt.Errorf("opt-out MT request failed: %w", err)
    }
    
    // Check response
    if response.Code != "0" && response.Code != "200" {
        return fmt.Errorf("opt-out response error: code=%s, message=%s", 
            response.Code, response.Message)
    }
    
    // Update local subscription status
    s.UpdateSubscriptionStatus(msisdn, product.ProductId, "PENDING_RENEWAL")
    
    return nil
}

// OptInForRenewal handles the resubscription part of renewal
func (s *SubscriptionService) OptInForRenewal(msisdn string, product *domain.Product, entryChannel string, cycle *RenewalCycle) error {
    txId := uuid.New().String()
    keyword := s.generateThreeLetterKeyword(product.Name)
    
    // Create SUBSCRIBE MT request
    mtReq := domain.MTRequest{
        ProductID:          product.ProductId,
        PricepointID:       product.PricePointId,
        UserIdentifier:     msisdn,
        UserIdentifierType: "MSISDN",
        SubKeyword:         keyword,
        Context:            "Subscription",
        MCC:                "620",
        MNC:                "03",
        EntryChannel:       entryChannel,
        LargeAccount:       product.ShortCode,
        MoTransactionUUID:  txId,
        SendDate:           time.Now().Format(time.RFC3339),
        Tags:               []string{"renewal", "opt_in", "resubscription"},
    }
    
    s.logger.Debug("Sending opt-in request",
        zap.String("msisdn", msisdn),
        zap.String("transactionId", txId))
    
    response, err := s.SendMT(mtReq, s.config.Application.TIMWE.Realm, entryChannel)
    if err != nil {
        return fmt.Errorf("opt-in MT request failed: %w", err)
    }
    
    // Handle different response scenarios
    if s.isSubscriptionAlreadyActive(response) {
        s.logger.Warn("Subscription already active after opt-out",
            zap.String("msisdn", msisdn))
        // This shouldn't happen but handle gracefully
        return nil
    }
    
    if s.isSubscriptionWaitingForCharging(response) {
        s.logger.Info("Resubscription successful, waiting for charging",
            zap.String("msisdn", msisdn),
            zap.String("transactionId", txId))
        
        // Update subscription record
        s.UpdateSubscriptionStatus(msisdn, product.ProductId, "ACTIVE_WAITING_CHARGE")
        cycle.BillingStatus = "WAITING_CHARGE"
        return nil
    }
    
    // Check for errors
    if response.Code != "0" && response.Code != "200" {
        return fmt.Errorf("opt-in response error: code=%s, message=%s",
            response.Code, response.Message)
    }
    
    return nil
}

// HandleFailedOptIn handles cases where opt-in fails after successful opt-out
func (s *SubscriptionService) HandleFailedOptIn(msisdn string, product *domain.Product, cycle *RenewalCycle) {
    s.logger.Error("CRITICAL: User unsubscribed but resubscription failed",
        zap.String("msisdn", msisdn),
        zap.String("productId", product.ProductId))
    
    // Add to priority retry queue
    s.AddToPriorityRetryQueue(msisdn, product, "FAILED_OPTIN")
    
    // Send alert
    s.alerting.SendCriticalAlert(fmt.Sprintf(
        "Failed opt-in for MSISDN %s, Product %s. User is currently unsubscribed!",
        msisdn, product.ProductId))
    
    // Update status
    s.UpdateSubscriptionStatus(msisdn, product.ProductId, "FAILED_RENEWAL")
}
```

### 2. Churn Policy Evaluator

```go
// churn_evaluator.go
package service

import (
    "time"
    "go.uber.org/zap"
)

// EvaluateChurnPolicy determines what action to take for a subscription
func (s *SubscriptionService) EvaluateChurnPolicy(msisdn string, productId string) ChurnAction {
    // Get subscription details
    sub, err := s.repo.GetSubscription(msisdn, productId)
    if err != nil {
        s.logger.Error("Failed to get subscription for churn evaluation", zap.Error(err))
        return ActionNoAction
    }
    
    // Get payment history
    lastPayment, err := s.repo.GetLastSuccessfulPayment(msisdn, productId)
    if err != nil {
        s.logger.Error("Failed to get last payment", zap.Error(err))
        return ActionNoAction
    }
    
    // Calculate days since last payment
    daysSincePayment := 0.0
    if lastPayment != nil {
        daysSincePayment = time.Since(*lastPayment).Hours() / 24
    }
    
    // Get renewal attempts count
    renewalAttempts, err := s.repo.GetRenewalAttemptsCount(msisdn, productId, 
        time.Now().AddDate(0, 0, -s.config.ChurnPolicy.MaxDaysWithoutPayment))
    if err != nil {
        s.logger.Error("Failed to get renewal attempts", zap.Error(err))
        renewalAttempts = 0
    }
    
    // Safety check to prevent mass churning
    if s.config.ChurnPolicy.SafeMode {
        dailyChurnCount, _ := s.repo.GetDailyChurnCount(time.Now())
        if dailyChurnCount > 1000 { // Safety threshold
            s.logger.Warn("Daily churn limit reached in safe mode",
                zap.Int("count", dailyChurnCount))
            return ActionNoAction
        }
    }
    
    // Evaluation logic
    s.logger.Debug("Evaluating churn policy",
        zap.String("msisdn", msisdn),
        zap.Float64("daysSincePayment", daysSincePayment),
        zap.Int("renewalAttempts", renewalAttempts))
    
    // If payment is recent, no action needed
    if daysSincePayment <= float64(s.config.ChurnPolicy.GracePeriodDays) {
        return ActionNoAction
    }
    
    // If in grace period, just monitor
    if daysSincePayment <= float64(s.config.ChurnPolicy.GracePeriodDays) {
        return ActionGracePeriod
    }
    
    // If beyond max days without payment
    if daysSincePayment > float64(s.config.ChurnPolicy.MaxDaysWithoutPayment) {
        // Check if we've exhausted renewal attempts
        if renewalAttempts >= s.config.ChurnPolicy.MaxRenewalAttempts {
            s.logger.Info("Subscription should be churned",
                zap.String("msisdn", msisdn),
                zap.String("reason", "max_attempts_exceeded"))
            return ActionChurn
        }
        
        // Check time since last attempt
        lastAttempt, _ := s.repo.GetLastRenewalAttempt(msisdn, productId)
        if lastAttempt != nil {
            hoursSinceLastAttempt := time.Since(*lastAttempt).Hours()
            if hoursSinceLastAttempt >= float64(s.config.ChurnPolicy.RetryIntervalHours) {
                return ActionAttemptRenewal
            }
        } else {
            // No previous attempts, try renewal
            return ActionAttemptRenewal
        }
    }
    
    // Default: attempt renewal if payment is overdue
    if daysSincePayment > float64(s.config.ChurnPolicy.GracePeriodDays) {
        return ActionAttemptRenewal
    }
    
    return ActionNoAction
}

// ChurnSubscription permanently unsubscribes a user
func (s *SubscriptionService) ChurnSubscription(msisdn string, productId string, reason string) error {
    s.logger.Info("Churning subscription",
        zap.String("msisdn", msisdn),
        zap.String("productId", productId),
        zap.String("reason", reason))
    
    // Get product details
    product, err := s.productRepo.GetProduct(productId)
    if err != nil {
        return fmt.Errorf("failed to get product: %w", err)
    }
    
    // Send final unsubscribe
    txId := uuid.New().String()
    mtReq := domain.MTRequest{
        ProductID:          productId,
        UserIdentifier:     msisdn,
        UserIdentifierType: "MSISDN",
        UnsubKeyword:       "STOP",
        Context:            "Unsubscription",
        MCC:                "620",
        MNC:                "03",
        EntryChannel:       "SYSTEM_CHURN",
        MoTransactionUUID:  txId,
        SendDate:           time.Now().Format(time.RFC3339),
        Tags:               []string{"churn", reason},
    }
    
    response, err := s.SendMT(mtReq, s.config.Application.TIMWE.Realm, "SYSTEM")
    if err != nil {
        s.logger.Error("Failed to send churn request", zap.Error(err))
        // Continue with local churn even if TIMWE fails
    }
    
    // Update database
    churnTime := time.Now()
    if err := s.repo.ChurnSubscription(msisdn, productId, reason, churnTime); err != nil {
        return fmt.Errorf("failed to update churn status: %w", err)
    }
    
    // Track metrics
    s.metrics.ChurnedSubscriptions.WithLabelValues(reason).Inc()
    
    // Log to churn tracking table
    s.repo.CreateChurnRecord(&ChurnRecord{
        MSISDN:              msisdn,
        ProductID:           productId,
        Reason:              reason,
        ChurnedAt:           churnTime,
        LastPaymentDate:     s.getLastPaymentDate(msisdn, productId),
        TotalRenewalAttempts: s.getRenewalAttemptsCount(msisdn, productId),
    })
    
    return nil
}
```

### 3. Renewal Worker

```go
// renewal_worker.go
package worker

import (
    "context"
    "sync"
    "time"
    "go.uber.org/zap"
)

type RenewalWorker struct {
    service        *service.SubscriptionService
    logger         *zap.Logger
    config         *Config
    metrics        *Metrics
    isRunning      bool
    mu             sync.RWMutex
}

func NewRenewalWorker(service *service.SubscriptionService, logger *zap.Logger, config *Config) *RenewalWorker {
    return &RenewalWorker{
        service: service,
        logger:  logger,
        config:  config,
        metrics: NewMetrics(),
    }
}

// Start begins the renewal worker
func (w *RenewalWorker) Start(ctx context.Context) error {
    w.mu.Lock()
    if w.isRunning {
        w.mu.Unlock()
        return fmt.Errorf("renewal worker already running")
    }
    w.isRunning = true
    w.mu.Unlock()
    
    w.logger.Info("Starting renewal worker")
    
    // Run based on schedule
    ticker := time.NewTicker(1 * time.Minute) // Check every minute
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            w.logger.Info("Renewal worker stopped")
            return nil
            
        case <-ticker.C:
            // Check if it's time to run
            if w.shouldRunNow() {
                w.ProcessRenewals()
            }
        }
    }
}

// shouldRunNow checks if current time matches scheduled run time
func (w *RenewalWorker) shouldRunNow() bool {
    now := time.Now()
    scheduledTime, _ := time.Parse("15:04", w.config.Renewal.DailyRunTime)
    
    return now.Hour() == scheduledTime.Hour() && 
           now.Minute() == scheduledTime.Minute()
}

// ProcessRenewals is the main renewal processing function
func (w *RenewalWorker) ProcessRenewals() {
    startTime := time.Now()
    w.logger.Info("Starting daily renewal processing")
    
    // Get subscriptions needing renewal
    subscriptions, err := w.service.GetSubscriptionsNeedingRenewal()
    if err != nil {
        w.logger.Error("Failed to get subscriptions for renewal", zap.Error(err))
        return
    }
    
    w.logger.Info("Found subscriptions for renewal evaluation",
        zap.Int("count", len(subscriptions)))
    
    // Process statistics
    stats := &ProcessingStats{
        Total:     len(subscriptions),
        Processed: 0,
        Renewed:   0,
        Churned:   0,
        Failed:    0,
        Skipped:   0,
    }
    
    // Process in batches
    batchSize := w.config.Renewal.BatchSize
    for i := 0; i < len(subscriptions); i += batchSize {
        end := i + batchSize
        if end > len(subscriptions) {
            end = len(subscriptions)
        }
        
        batch := subscriptions[i:end]
        w.processBatch(batch, stats)
        
        // Rate limiting between batches
        if i+batchSize < len(subscriptions) {
            time.Sleep(time.Duration(w.config.Renewal.BatchDelayMs) * time.Millisecond)
        }
    }
    
    // Log final statistics
    w.logger.Info("Renewal processing completed",
        zap.Int("total", stats.Total),
        zap.Int("renewed", stats.Renewed),
        zap.Int("churned", stats.Churned),
        zap.Int("failed", stats.Failed),
        zap.Int("skipped", stats.Skipped),
        zap.Duration("duration", time.Since(startTime)))
    
    // Update metrics
    w.metrics.RenewalRunDuration.Observe(time.Since(startTime).Seconds())
    w.metrics.RenewalProcessed.Add(float64(stats.Processed))
}

// processBatch processes a batch of subscriptions
func (w *RenewalWorker) processBatch(subscriptions []*Subscription, stats *ProcessingStats) {
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, w.config.Renewal.MaxConcurrent)
    
    for _, sub := range subscriptions {
        wg.Add(1)
        semaphore <- struct{}{}
        
        go func(sub *Subscription) {
            defer wg.Done()
            defer func() { <-semaphore }()
            
            w.processSubscription(sub, stats)
        }(sub)
    }
    
    wg.Wait()
}

// processSubscription handles a single subscription renewal
func (w *RenewalWorker) processSubscription(sub *Subscription, stats *ProcessingStats) {
    stats.Processed++
    
    // Evaluate churn policy
    action := w.service.EvaluateChurnPolicy(sub.MSISDN, sub.ProductID)
    
    w.logger.Debug("Processing subscription",
        zap.String("msisdn", sub.MSISDN),
        zap.String("productId", sub.ProductID),
        zap.String("action", string(action)))
    
    switch action {
    case service.ActionAttemptRenewal:
        if err := w.attemptRenewal(sub); err != nil {
            w.logger.Error("Renewal failed",
                zap.String("msisdn", sub.MSISDN),
                zap.Error(err))
            stats.Failed++
            w.metrics.RenewalAttempts.WithLabelValues("failed").Inc()
        } else {
            stats.Renewed++
            w.metrics.RenewalAttempts.WithLabelValues("success").Inc()
        }
        
    case service.ActionChurn:
        if err := w.service.ChurnSubscription(sub.MSISDN, sub.ProductID, "PAYMENT_FAILURE"); err != nil {
            w.logger.Error("Failed to churn subscription",
                zap.String("msisdn", sub.MSISDN),
                zap.Error(err))
            stats.Failed++
        } else {
            stats.Churned++
            w.metrics.ChurnedTotal.Inc()
        }
        
    case service.ActionGracePeriod:
        w.logger.Debug("Subscription in grace period",
            zap.String("msisdn", sub.MSISDN))
        stats.Skipped++
        
    default:
        stats.Skipped++
    }
}

// attemptRenewal performs the opt-out/opt-in renewal cycle
func (w *RenewalWorker) attemptRenewal(sub *Subscription) error {
    // Get product details
    product, err := w.service.GetProduct(sub.ProductID)
    if err != nil {
        return fmt.Errorf("failed to get product: %w", err)
    }
    
    // Perform renewal using opt-out/opt-in strategy
    if err := w.service.SendRenewalRequest(sub.MSISDN, product, sub.EntryChannel); err != nil {
        return fmt.Errorf("renewal request failed: %w", err)
    }
    
    // Update renewal attempt count
    w.service.IncrementRenewalAttempt(sub.MSISDN, sub.ProductID)
    
    return nil
}
```

### 4. Database Migrations

```sql
-- Migration: Add opt-out/opt-in renewal tracking
BEGIN;

-- Track renewal cycles
CREATE TABLE IF NOT EXISTS renewal_cycles (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    msisdn VARCHAR(20) NOT NULL,
    product_id VARCHAR(20) NOT NULL,
    cycle_number INT DEFAULT 1,
    opt_out_time TIMESTAMP,
    opt_out_status VARCHAR(50),
    opt_out_response TEXT,
    opt_in_time TIMESTAMP,
    opt_in_status VARCHAR(50),
    opt_in_response TEXT,
    billing_status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_subscription FOREIGN KEY (subscription_id) 
        REFERENCES subscriptions(id) ON DELETE CASCADE
);

CREATE INDEX idx_renewal_cycles_msisdn ON renewal_cycles(msisdn, product_id);
CREATE INDEX idx_renewal_cycles_status ON renewal_cycles(billing_status, created_at);
CREATE INDEX idx_renewal_cycles_created ON renewal_cycles(created_at);

-- Track churn decisions and history
CREATE TABLE IF NOT EXISTS churn_tracking (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    msisdn VARCHAR(20) NOT NULL,
    product_id VARCHAR(20) NOT NULL,
    last_payment_date TIMESTAMP,
    days_without_payment INT,
    renewal_attempts INT DEFAULT 0,
    churn_decision VARCHAR(50),
    churn_reason VARCHAR(100),
    churned_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_subscription_churn FOREIGN KEY (subscription_id) 
        REFERENCES subscriptions(id) ON DELETE CASCADE
);

CREATE INDEX idx_churn_tracking_msisdn ON churn_tracking(msisdn, product_id);
CREATE INDEX idx_churn_tracking_churned ON churn_tracking(churned_at);
CREATE INDEX idx_churn_tracking_decision ON churn_tracking(churn_decision);

-- Priority retry queue for failed opt-ins
CREATE TABLE IF NOT EXISTS priority_retry_queue (
    id BIGSERIAL PRIMARY KEY,
    msisdn VARCHAR(20) NOT NULL,
    product_id VARCHAR(20) NOT NULL,
    reason VARCHAR(50),
    priority INT DEFAULT 1,
    retry_count INT DEFAULT 0,
    next_retry_at TIMESTAMP,
    last_attempt_at TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_priority_retry_next ON priority_retry_queue(next_retry_at, status);
CREATE INDEX idx_priority_retry_priority ON priority_retry_queue(priority DESC, created_at);

-- Add renewal tracking columns to subscriptions
ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS renewal_status VARCHAR(50) DEFAULT 'active',
ADD COLUMN IF NOT EXISTS last_renewal_attempt TIMESTAMP,
ADD COLUMN IF NOT EXISTS total_renewal_attempts INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_successful_payment TIMESTAMP,
ADD COLUMN IF NOT EXISTS consecutive_payment_failures INT DEFAULT 0;

-- Function to get subscriptions needing renewal
CREATE OR REPLACE FUNCTION get_subscriptions_needing_renewal(
    p_days_threshold INT DEFAULT 7,
    p_limit INT DEFAULT 1000
) RETURNS TABLE (
    subscription_id BIGINT,
    msisdn VARCHAR,
    product_id VARCHAR,
    last_payment TIMESTAMP,
    days_since_payment INT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        s.id,
        s.msisdn,
        s.product_id,
        s.last_successful_payment,
        EXTRACT(DAY FROM NOW() - COALESCE(s.last_successful_payment, s.created_at))::INT
    FROM subscriptions s
    WHERE s.status = 'active'
        AND s.renewal_status != 'churned'
        AND (
            s.last_successful_payment IS NULL 
            OR s.last_successful_payment < NOW() - INTERVAL '2 days'
        )
        AND (
            s.last_renewal_attempt IS NULL 
            OR s.last_renewal_attempt < NOW() - INTERVAL '24 hours'
        )
    ORDER BY s.last_successful_payment ASC NULLS FIRST
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

COMMIT;
```

### 5. Configuration File

```yaml
# config.yaml - Complete configuration
application:
  name: "subscription-external"
  version: "2.0.0"
  environment: "production"

timwe:
  api_url: "https://api.timwe.com"
  api_key: "${TIMWE_API_KEY}"
  authentication_key: "${TIMWE_AUTH_KEY}"
  realm: "GH"
  partner_role_id: "12345"
  
  # Charging endpoint is broken - DO NOT USE
  charging_enabled: false

renewal:
  strategy: "opt_out_opt_in"  # New strategy since charging is broken
  enabled: true
  
  churn_policy:
    max_days_without_payment: 7
    max_renewal_attempts: 3
    retry_interval_hours: 24
    grace_period_days: 2
    safe_mode: true  # Prevent accidental mass churning
    
  opt_out_opt_in:
    wait_between_ms: 3000      # Wait 3 seconds between opt-out and opt-in
    batch_size: 50              # Process 50 at a time
    max_concurrent: 5           # Max 5 concurrent renewals
    rate_limit_ms: 500          # 500ms between each renewal
    batch_delay_ms: 2000        # 2 seconds between batches
    
  worker:
    enabled: true
    daily_run_time: "02:00"     # Run at 2 AM daily
    timezone: "UTC"
    timeout_per_renewal: 30s
    max_retries: 3
    
  monitoring:
    alert_on_failure_rate: 0.3  # Alert if >30% failures
    alert_on_churn_rate: 0.1    # Alert if >10% churned
    metrics_port: 9090

database:
  host: "${DB_HOST}"
  port: 5432
  name: "${DB_NAME}"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"
  max_connections: 100
  max_idle_connections: 10

redis:
  addr: "${REDIS_HOST}:6379"
  password: "${REDIS_PASSWORD}"
  db: 0
  pool_size: 10
  
logging:
  level: "info"
  format: "json"
  output: "stdout"
  file:
    enabled: true
    path: "/var/log/subscription/renewal.log"
    max_size: 100
    max_backups: 10
    max_age: 30
```

### 6. Deployment Script

```bash
#!/bin/bash
# deploy_renewal_system.sh

set -e

echo "=== Deploying Opt-Out/Opt-In Renewal System ==="

# Load environment
source /etc/subscription/.env

# 1. Backup current data
echo "Creating backup..."
pg_dump -U $DB_USER -d $DB_NAME > backup_$(date +%Y%m%d_%H%M%S).sql

# 2. Run migrations
echo "Running database migrations..."
psql -U $DB_USER -d $DB_NAME -f migrations/renewal_optout_optin.sql

# 3. Build and deploy services
echo "Building renewal worker..."
go build -o /usr/local/bin/renewal-worker ./cmd/renewal-worker

# 4. Create systemd service
cat > /etc/systemd/system/renewal-worker.service << EOF
[Unit]
Description=Subscription Renewal Worker (Opt-Out/Opt-In)
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=subscription
Group=subscription
WorkingDirectory=/opt/subscription
ExecStart=/usr/local/bin/renewal-worker
Restart=always
RestartSec=10
StandardOutput=append:/var/log/subscription/renewal-worker.log
StandardError=append:/var/log/subscription/renewal-worker-error.log

[Install]
WantedBy=multi-user.target
EOF

# 5. Create cron job for safety check
cat > /etc/cron.d/renewal-monitor << EOF
# Monitor renewal worker health
*/5 * * * * subscription /usr/local/bin/check-renewal-health.sh
# Daily churn evaluation
0 1 * * * subscription /usr/local/bin/evaluate-churns.sh
# Retry failed opt-ins
*/30 * * * * subscription /usr/local/bin/retry-failed-optins.sh
EOF

# 6. Setup monitoring
echo "Setting up Prometheus metrics..."
cat >> /etc/prometheus/prometheus.yml << EOF
  - job_name: 'renewal_worker'
    static_configs:
    - targets: ['localhost:9090']
      labels:
        service: 'renewal'
EOF

# 7. Start services
echo "Starting services..."
systemctl daemon-reload
systemctl enable renewal-worker
systemctl restart renewal-worker

# 8. Verify deployment
sleep 5
if systemctl is-active --quiet renewal-worker; then
    echo "✓ Renewal worker is running"
else
    echo "✗ Renewal worker failed to start"
    exit 1
fi

echo "=== Deployment Complete ==="
echo "Monitor logs at: /var/log/subscription/renewal-worker.log"
echo "Metrics available at: http://localhost:9090/metrics"
```

### 7. Monitoring Script

```bash
#!/bin/bash
# check-renewal-health.sh

# Check if worker is running
if ! systemctl is-active --quiet renewal-worker; then
    echo "ALERT: Renewal worker is not running!"
    systemctl restart renewal-worker
    
    # Send alert
    curl -X POST $ALERT_WEBHOOK -d '{
        "text": "Renewal worker was down and has been restarted"
    }'
fi

# Check for stuck renewals
STUCK_COUNT=$(psql -U $DB_USER -d $DB_NAME -t -c "
    SELECT COUNT(*) FROM renewal_cycles 
    WHERE billing_status = 'PENDING' 
    AND created_at < NOW() - INTERVAL '1 hour';
")

if [ $STUCK_COUNT -gt 100 ]; then
    echo "ALERT: $STUCK_COUNT stuck renewals detected!"
    # Send alert
fi

# Check for high failure rate
FAILURE_RATE=$(psql -U $DB_USER -d $DB_NAME -t -c "
    SELECT 
        ROUND(
            COUNT(CASE WHEN opt_in_status = 'FAILED' THEN 1 END)::NUMERIC / 
            NULLIF(COUNT(*), 0) * 100, 2
        )
    FROM renewal_cycles 
    WHERE created_at > NOW() - INTERVAL '1 hour';
")

if (( $(echo "$FAILURE_RATE > 30" | bc -l) )); then
    echo "ALERT: High failure rate detected: ${FAILURE_RATE}%"
fi
```

## Testing Guide

### Unit Tests

```go
func TestOptOutOptInRenewal(t *testing.T) {
    service := setupTestService()
    msisdn := "233123456789"
    product := &domain.Product{
        ProductId: "1001",
        Name: "Daily Bundle",
    }
    
    // Mock TIMWE responses
    mockTIMWE.ExpectOptOut().Return(successResponse)
    mockTIMWE.ExpectOptIn().Return(waitingForChargingResponse)
    
    // Execute renewal
    err := service.SendRenewalRequest(msisdn, product, "WEB")
    
    // Verify
    assert.NoError(t, err)
    assert.True(t, mockTIMWE.OptOutCalled())
    assert.True(t, mockTIMWE.OptInCalled())
    
    // Check renewal cycle saved
    cycle := getRenewalCycle(msisdn, product.ProductId)
    assert.NotNil(t, cycle)
    assert.Equal(t, "SUCCESS", cycle.OptOutStatus)
    assert.Equal(t, "SUCCESS", cycle.OptInStatus)
}
```

## Rollback Procedure

```bash
#!/bin/bash
# rollback.sh

echo "Rolling back renewal system..."

# 1. Stop worker
systemctl stop renewal-worker

# 2. Restore database
psql -U $DB_USER -d $DB_NAME < backup_latest.sql

# 3. Revert configuration
cp /etc/subscription/config.yaml.backup /etc/subscription/config.yaml

# 4. Restart with old system
systemctl start subscription-service

echo "Rollback complete"
```

## Success Metrics

- Renewal success rate > 75%
- Opt-out/Opt-in cycle time < 10 seconds
- No duplicate subscriptions
- Churn rate < expected threshold
- Failed opt-ins < 1%
