# Renewal & Charging Implementation Guide

## Overview
This document provides a complete implementation for handling subscription renewals with proper charging/billing retry logic for the TIMWE subscription system.

## Core Problem
The current `SendRenewalRequest` function only creates a notification record but doesn't trigger actual billing. This results in subscriptions appearing active but not being charged.

## Solution Architecture

### 1. Enhanced Renewal Request with Charging

```go
// subscription.go - Enhanced SendRenewalRequest
func (s *SubscriptionService) SendRenewalRequest(msisdn string, product *domain.Product, entryChannel string) error {
    txId := uuid.New().String()
    reqDate := time.Now().Format(time.RFC3339)
    
    productId, err := strconv.Atoi(product.ProductId)
    if err != nil {
        return fmt.Errorf("invalid ProductId: %w", err)
    }

    // Step 1: Send MT Renewal Request
    keyword := s.generateThreeLetterKeyword(product.Name)
    mtReq := domain.MTRequest{
        ProductID:          productId,
        PricepointID:       product.PricePointId,
        UserIdentifier:     msisdn,
        UserIdentifierType: "MSISDN",
        SubKeyword:         keyword,
        Context:            "Renewal",
        MCC:                "620",
        MNC:                "03",
        EntryChannel:       entryChannel,
        LargeAccount:       product.ShortCode,
        MoTransactionUUID:  txId,
        SendDate:           reqDate,
    }

    s.logger.Info("Sending renewal MT request", zap.Any("mtRequest", mtReq))
    
    realm := s.config.Application.TIMWE.Realm
    response, err := s.SendMT(mtReq, realm, entryChannel)
    if err != nil {
        s.logger.Error("Error sending renewal MT", zap.Error(err))
        return fmt.Errorf("error sending renewal MT: %w", err)
    }

    // Step 2: Save Renewal Notification
    partnerRole, _ := strconv.Atoi(s.config.Application.TIMWE.PartnerRoleID)
    notification := domain.NotificationRequest{
        PartnerRole:     partnerRole,
        ExternalTxID:    txId,
        ProductID:       product.PricePointId,
        PricepointID:    product.PricePointId,
        MCC:             "620",
        MNC:             "03",
        MSISDN:          msisdn,
        LargeAccount:    product.ShortCode,
        TransactionUUID: txId,
        EntryChannel:    entryChannel,
        MessageType:     "Renewal",
        Message:         "Subscription renewal request",
        Tags:            []string{"renewal", "subscription"},
        Type:            "RENEWAL",
    }

    if err := s.repo.CreateNotification(&notification); err != nil {
        s.logger.Error("Error saving renewal notification", zap.Error(err))
        return fmt.Errorf("error saving renewal notification: %w", err)
    }

    // Step 3: CRITICAL - Trigger Charging
    if err := s.TriggerRenewalCharging(msisdn, product, txId); err != nil {
        s.logger.Error("Failed to trigger renewal charging", 
            zap.String("msisdn", msisdn),
            zap.Error(err))
        
        // Queue for retry even if initial charging fails
        s.QueueForRetry(msisdn, product, txId, 1)
    }

    return nil
}
```

### 2. Charging Trigger Implementation

```go
// charging.go - New charging trigger method
func (s *SubscriptionService) TriggerRenewalCharging(msisdn string, product *domain.Product, txId string) error {
    // Prepare charge request
    chargeRequest := domain.ChargeRequest{
        ProductID:          product.ProductId,
        PricepointID:       product.PricePointId,
        UserIdentifier:     msisdn,
        UserIdentifierType: "MSISDN",
        TransactionID:      txId,
        Amount:             product.Price,
        Currency:           product.Currency,
        ChargeType:         "RENEWAL",
        Timestamp:          time.Now().Format(time.RFC3339),
    }

    // Use existing ChargeWithRetry with exponential backoff
    chargeResponse, err := s.ChargeWithRetry(chargeRequest)
    if err != nil {
        return fmt.Errorf("charging failed after retries: %w", err)
    }

    // Update subscription with successful charge
    if chargeResponse.Code == "0" || chargeResponse.Code == "200" {
        s.UpdateLastSuccessfulCharge(msisdn, product.ProductId)
        s.logger.Info("Renewal charging successful",
            zap.String("msisdn", msisdn),
            zap.String("transactionId", chargeResponse.TransactionID))
        return nil
    }

    return fmt.Errorf("charging returned error code: %s", chargeResponse.Code)
}
```

### 3. Retry Queue Implementation

```go
// retry_queue.go - Redis-based retry queue
package worker

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "github.com/redis/go-redis/v9"
)

type RetryQueueItem struct {
    MSISDN         string    `json:"msisdn"`
    ProductID      string    `json:"product_id"`
    TransactionID  string    `json:"transaction_id"`
    AttemptNumber  int       `json:"attempt_number"`
    NextRetryTime  time.Time `json:"next_retry_time"`
    LastAttemptAt  time.Time `json:"last_attempt_at"`
    EntryChannel   string    `json:"entry_channel"`
}

type RetryQueue struct {
    client *redis.Client
    logger *zap.Logger
}

func NewRetryQueue(redisClient *redis.Client, logger *zap.Logger) *RetryQueue {
    return &RetryQueue{
        client: redisClient,
        logger: logger,
    }
}

func (rq *RetryQueue) Enqueue(item *RetryQueueItem) error {
    ctx := context.Background()
    
    // Calculate next retry time with exponential backoff
    item.NextRetryTime = rq.calculateNextRetryTime(item.AttemptNumber)
    
    data, err := json.Marshal(item)
    if err != nil {
        return err
    }
    
    // Use sorted set with retry time as score
    score := float64(item.NextRetryTime.Unix())
    key := fmt.Sprintf("retry:queue:%s", time.Now().Format("2006-01-02"))
    
    return rq.client.ZAdd(ctx, key, redis.Z{
        Score:  score,
        Member: string(data),
    }).Err()
}

func (rq *RetryQueue) GetDueItems(limit int) ([]*RetryQueueItem, error) {
    ctx := context.Background()
    now := time.Now().Unix()
    
    // Get items due for retry
    key := fmt.Sprintf("retry:queue:%s", time.Now().Format("2006-01-02"))
    results, err := rq.client.ZRangeByScoreWithScores(ctx, key,
        &redis.ZRangeBy{
            Min:   "0",
            Max:   fmt.Sprintf("%d", now),
            Count: int64(limit),
        }).Result()
    
    if err != nil {
        return nil, err
    }
    
    items := make([]*RetryQueueItem, 0, len(results))
    for _, result := range results {
        var item RetryQueueItem
        if err := json.Unmarshal([]byte(result.Member.(string)), &item); err != nil {
            rq.logger.Error("Failed to unmarshal retry item", zap.Error(err))
            continue
        }
        items = append(items, &item)
    }
    
    return items, nil
}

func (rq *RetryQueue) calculateNextRetryTime(attemptNumber int) time.Time {
    // Exponential backoff: 30s, 1m, 2m, 4m, 8m, 16m, 32m, 1h, 2h, 4h
    delays := []time.Duration{
        30 * time.Second,
        1 * time.Minute,
        2 * time.Minute,
        4 * time.Minute,
        8 * time.Minute,
        16 * time.Minute,
        32 * time.Minute,
        1 * time.Hour,
        2 * time.Hour,
        4 * time.Hour,
    }
    
    index := attemptNumber - 1
    if index >= len(delays) {
        index = len(delays) - 1
    }
    
    return time.Now().Add(delays[index])
}
```

### 4. Retry Worker Implementation

```go
// retry_worker.go - Background worker for processing retries
package worker

import (
    "context"
    "sync"
    "time"
)

type RetryWorker struct {
    queue           *RetryQueue
    service         *service.SubscriptionService
    logger          *zap.Logger
    maxConcurrency  int
    maxRetries      int
    churnThreshold  int // days without successful charge
}

func NewRetryWorker(queue *RetryQueue, service *service.SubscriptionService, logger *zap.Logger) *RetryWorker {
    return &RetryWorker{
        queue:          queue,
        service:        service,
        logger:         logger,
        maxConcurrency: 10,
        maxRetries:     10,
        churnThreshold: 7,
    }
}

func (rw *RetryWorker) Start(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            rw.processRetries()
        }
    }
}

func (rw *RetryWorker) processRetries() {
    items, err := rw.queue.GetDueItems(100)
    if err != nil {
        rw.logger.Error("Failed to get retry items", zap.Error(err))
        return
    }
    
    if len(items) == 0 {
        return
    }
    
    rw.logger.Info("Processing retry items", zap.Int("count", len(items)))
    
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, rw.maxConcurrency)
    
    for _, item := range items {
        wg.Add(1)
        semaphore <- struct{}{}
        
        go func(item *RetryQueueItem) {
            defer wg.Done()
            defer func() { <-semaphore }()
            
            rw.processItem(item)
        }(item)
    }
    
    wg.Wait()
}

func (rw *RetryWorker) processItem(item *RetryQueueItem) {
    // Get product details
    product, err := rw.service.GetProduct(item.ProductID)
    if err != nil {
        rw.logger.Error("Failed to get product", zap.Error(err))
        return
    }
    
    // Attempt charging
    err = rw.service.TriggerRenewalCharging(item.MSISDN, product, item.TransactionID)
    
    if err == nil {
        // Success - remove from queue
        rw.logger.Info("Retry charging successful",
            zap.String("msisdn", item.MSISDN),
            zap.Int("attempts", item.AttemptNumber))
        rw.queue.Remove(item)
        return
    }
    
    // Failed - check if we should retry or churn
    item.AttemptNumber++
    item.LastAttemptAt = time.Now()
    
    if item.AttemptNumber >= rw.maxRetries {
        // Check if we should churn
        if rw.shouldChurn(item) {
            rw.churnSubscription(item)
        } else {
            // Reset attempts for next day
            item.AttemptNumber = 1
            rw.queue.Enqueue(item)
        }
    } else {
        // Re-queue for next attempt
        rw.queue.Enqueue(item)
    }
}

func (rw *RetryWorker) shouldChurn(item *RetryQueueItem) bool {
    // Check last successful charge date
    lastCharge, err := rw.service.GetLastSuccessfulCharge(item.MSISDN, item.ProductID)
    if err != nil {
        return false
    }
    
    daysSinceCharge := time.Since(lastCharge).Hours() / 24
    return daysSinceCharge >= float64(rw.churnThreshold)
}

func (rw *RetryWorker) churnSubscription(item *RetryQueueItem) {
    rw.logger.Info("Churning subscription due to payment failures",
        zap.String("msisdn", item.MSISDN),
        zap.String("productId", item.ProductID))
    
    // Update subscription status to churned
    rw.service.ChurnSubscription(item.MSISDN, item.ProductID, "PAYMENT_FAILURE")
    
    // Remove from retry queue
    rw.queue.Remove(item)
}
```

### 5. Daily Renewal Scheduler

```go
// renewal_scheduler.go - Daily renewal processing
package worker

import (
    "context"
    "time"
)

type RenewalScheduler struct {
    service *service.SubscriptionService
    logger  *zap.Logger
}

func (rs *RenewalScheduler) RunDailyRenewals() error {
    rs.logger.Info("Starting daily renewal processing")
    
    // Get all active subscriptions due for renewal
    subscriptions, err := rs.service.GetSubscriptionsDueForRenewal()
    if err != nil {
        return err
    }
    
    rs.logger.Info("Processing renewals", zap.Int("count", len(subscriptions)))
    
    successCount := 0
    failureCount := 0
    
    for _, sub := range subscriptions {
        product, err := rs.service.GetProduct(sub.ProductID)
        if err != nil {
            rs.logger.Error("Failed to get product", zap.Error(err))
            failureCount++
            continue
        }
        
        err = rs.service.SendRenewalRequest(sub.MSISDN, product, sub.EntryChannel)
        if err != nil {
            rs.logger.Error("Renewal failed", 
                zap.String("msisdn", sub.MSISDN),
                zap.Error(err))
            failureCount++
        } else {
            successCount++
        }
        
        // Rate limiting
        time.Sleep(100 * time.Millisecond)
    }
    
    rs.logger.Info("Daily renewal complete",
        zap.Int("success", successCount),
        zap.Int("failures", failureCount))
    
    return nil
}
```

### 6. Database Migrations

```sql
-- Migration: Add renewal and retry tracking
BEGIN;

-- Track renewal attempts
CREATE TABLE IF NOT EXISTS renewal_attempts (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    msisdn VARCHAR(20) NOT NULL,
    product_id INT NOT NULL,
    transaction_id VARCHAR(100),
    attempt_number INT DEFAULT 1,
    status VARCHAR(20) DEFAULT 'pending',
    next_retry_at TIMESTAMP,
    last_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_renewal_retry ON renewal_attempts(next_retry_at, status);
CREATE INDEX idx_renewal_msisdn ON renewal_attempts(msisdn, product_id);

-- Track charging health
ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS last_successful_charge TIMESTAMP,
ADD COLUMN IF NOT EXISTS consecutive_failures INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS churned_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS churn_reason VARCHAR(50);

-- Create function to update charging health
CREATE OR REPLACE FUNCTION update_charging_health(
    p_msisdn VARCHAR,
    p_product_id INT,
    p_success BOOLEAN
) RETURNS VOID AS $$
BEGIN
    IF p_success THEN
        UPDATE subscriptions 
        SET last_successful_charge = CURRENT_TIMESTAMP,
            consecutive_failures = 0
        WHERE msisdn = p_msisdn AND product_id = p_product_id;
    ELSE
        UPDATE subscriptions 
        SET consecutive_failures = consecutive_failures + 1
        WHERE msisdn = p_msisdn AND product_id = p_product_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMIT;
```

### 7. Configuration

```yaml
# config.yaml
timwe:
  api_key: "your-api-key"
  authentication_key: "your-auth-key"
  charge_retry:
    max_duration: 2m
    base_delay: 200ms
    max_delay: 5s

renewal:
  enabled: true
  batch_size: 100
  max_concurrency: 10
  retry:
    max_attempts: 10
    delays: [30s, 1m, 2m, 4m, 8m, 16m, 32m, 1h, 2h, 4h]
    daily_limit: 3
  churn:
    threshold_days: 7
    auto_churn: true
  scheduler:
    daily_run_time: "02:00"
    timezone: "UTC"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 10

monitoring:
  metrics:
    enabled: true
    port: 9090
  alerts:
    renewal_success_threshold: 0.7
    retry_queue_threshold: 10000
    processing_time_threshold: 5m
```

### 8. Deployment Scripts

```bash
#!/bin/bash
# deploy_renewal_system.sh

echo "Deploying Renewal System..."

# 1. Run database migrations
psql -U $DB_USER -d $DB_NAME -f migrations/renewal_tracking.sql

# 2. Deploy retry worker
systemctl stop retry-worker
cp ./bin/retry-worker /usr/local/bin/
systemctl start retry-worker
systemctl enable retry-worker

# 3. Setup cron for daily renewals
cat > /etc/cron.d/renewal-scheduler << EOF
# Daily renewal processing at 2 AM
0 2 * * * subscription /usr/local/bin/renewal-scheduler >> /var/log/renewal.log 2>&1
EOF

# 4. Setup monitoring
cp ./configs/prometheus.yml /etc/prometheus/
systemctl reload prometheus

echo "Deployment complete!"
```

### 9. Monitoring & Alerts

```go
// metrics.go - Prometheus metrics
package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    RenewalAttempts = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "renewal_attempts_total",
            Help: "Total number of renewal attempts",
        },
        []string{"status", "product_id"},
    )
    
    ChargingLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "charging_latency_seconds",
            Help: "Latency of charging operations",
        },
        []string{"type"},
    )
    
    RetryQueueSize = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "retry_queue_size",
            Help: "Current size of retry queue",
        },
    )
    
    ChurnedSubscriptions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "churned_subscriptions_total",
            Help: "Total churned subscriptions",
        },
        []string{"reason"},
    )
)

func init() {
    prometheus.MustRegister(RenewalAttempts)
    prometheus.MustRegister(ChargingLatency)
    prometheus.MustRegister(RetryQueueSize)
    prometheus.MustRegister(ChurnedSubscriptions)
}
```

## Testing

### Integration Test

```go
func TestRenewalWithChargingFlow(t *testing.T) {
    // Setup
    service := setupTestService()
    msisdn := "233123456789"
    product := getTestProduct()
    
    // Mock TIMWE responses
    mockTIMWE.ExpectMTRequest().Return(successResponse)
    mockTIMWE.ExpectChargeRequest().Return(chargeSuccessResponse)
    
    // Execute
    err := service.SendRenewalRequest(msisdn, product, "WEB")
    
    // Verify
    assert.NoError(t, err)
    
    // Check notification saved
    notification := getNotification(msisdn)
    assert.NotNil(t, notification)
    
    // Check charging attempted
    assert.True(t, mockTIMWE.ChargeRequestCalled())
    
    // Check subscription updated
    sub := getSubscription(msisdn)
    assert.NotNil(t, sub.LastSuccessfulCharge)
}
```

## Rollback Plan

If issues arise:
1. Disable renewal worker via feature flag
2. Stop retry worker systemctl
3. Revert database migrations
4. Clear Redis retry queue
5. Restore previous version

## Success Criteria

- [ ] Renewal success rate > 80%
- [ ] Retry queue processing < 5 minutes
- [ ] Auto-churn working correctly
- [ ] No memory leaks in workers
- [ ] Monitoring dashboards showing metrics

## Timeline

- Day 1: Deploy enhanced SendRenewalRequest
- Day 2: Deploy retry queue and worker
- Day 3: Enable daily renewal scheduler
- Day 4-5: Monitor and tune
- Day 6-7: Full rollout
