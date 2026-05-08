# Strategy for Handling 25+ Million Failed Subscriptions
## Charging Issue Re-subscription Process

---

## Executive Summary

The system needs to process over 25,169,944 subscriptions that failed due to charging issues. This document provides a comprehensive strategy for safely and efficiently handling these re-subscriptions while preventing duplicate processing and system overload.

## Current State Analysis

### Existing Infrastructure
1. **Resubscribe Processor**: Located at `/cmd/resubscribe-processor/`
   - Supports batch processing with configurable workers
   - Has windowing support for processing subsets
   - Includes entry channel rotation
   - Prometheus metrics integration
   - Async job processing with polling

2. **Database Schema**
   - `subscriptions` table tracks active subscriptions
   - `invalid_msisdn_logs` table for failed attempts
   - No specific tracking for charging failures
   - No deduplication mechanism for re-subscription attempts

3. **Current Limitations**
   - Query only fetches 'active' subscriptions
   - No differentiation between charging failures and other failures
   - No checkpoint/recovery mechanism
   - No duplicate prevention for already processed MSISDNs
   - Limited visibility into charging failure reasons

## Critical Issues to Address

### 1. Identifying Charging-Failed Subscriptions
**Problem**: The current query fetches active subscriptions, not failed ones.
**Solution**: Create a specific query to identify subscriptions with charging issues.

### 2. Duplicate Processing Prevention
**Problem**: No mechanism to prevent re-processing already handled subscriptions.
**Solution**: Implement tracking table and status updates.

### 3. Scale and Performance
**Problem**: 25+ million records require careful resource management.
**Solution**: Implement proper batching, windowing, and rate limiting.

### 4. Recovery and Checkpointing
**Problem**: No recovery mechanism if process fails midway.
**Solution**: Implement checkpoint-based recovery system.

## Proposed Solution Architecture

### Phase 1: Database Schema Updates

```sql
-- 1. Add charging failure tracking to subscriptions table
ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS last_charging_failure_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS charging_failure_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS charging_failure_reason VARCHAR(255),
ADD COLUMN IF NOT EXISTS last_resubscribe_attempt_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS resubscribe_attempt_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS resubscribe_status VARCHAR(50) DEFAULT NULL;

-- 2. Create resubscription tracking table
CREATE TABLE IF NOT EXISTS resubscription_tracking (
    id SERIAL PRIMARY KEY,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id),
    msisdn VARCHAR(15) NOT NULL,
    product_id INTEGER NOT NULL,
    original_status VARCHAR(50),
    attempt_number INTEGER DEFAULT 1,
    process_batch_id VARCHAR(100),
    unsubscribe_status VARCHAR(50),
    unsubscribe_at TIMESTAMP,
    resubscribe_status VARCHAR(50),
    resubscribe_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subscription_id, process_batch_id)
);

-- 3. Create indexes for performance
CREATE INDEX idx_subscriptions_charging_failure ON subscriptions(last_charging_failure_at, charging_failure_count);
CREATE INDEX idx_subscriptions_resubscribe_status ON subscriptions(resubscribe_status);
CREATE INDEX idx_resubscription_tracking_batch ON resubscription_tracking(process_batch_id);
CREATE INDEX idx_resubscription_tracking_status ON resubscription_tracking(resubscribe_status);

-- 4. Create checkpoint table for recovery
CREATE TABLE IF NOT EXISTS resubscription_checkpoints (
    id SERIAL PRIMARY KEY,
    batch_id VARCHAR(100) UNIQUE NOT NULL,
    total_count INTEGER NOT NULL,
    processed_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    last_processed_id INTEGER,
    last_processed_msisdn VARCHAR(15),
    status VARCHAR(50) DEFAULT 'pending',
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);
```

### Phase 2: Identify Target Subscriptions

```sql
-- Query to identify subscriptions with charging issues
-- This needs to be adapted based on how charging failures are logged
WITH charging_failed_subs AS (
    SELECT DISTINCT 
        s.id,
        s.user_identifier as msisdn,
        s.product_id,
        s.entry_channel,
        s.status,
        s.created_at,
        s.charging_failure_count,
        s.last_charging_failure_at
    FROM subscriptions s
    LEFT JOIN invalid_msisdn_logs iml ON s.user_identifier = iml.msisdn 
        AND s.product_id = iml.product_id
    WHERE 
        -- Subscriptions with charging issues
        (
            -- Check for specific charging error codes
            iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE', 'CHARGING_ERROR')
            OR iml.response_message LIKE '%charging%'
            OR iml.subscription_error LIKE '%charging%'
            -- Or subscriptions marked with charging failures
            OR s.charging_failure_count > 0
            OR s.last_charging_failure_at IS NOT NULL
        )
        -- Exclude already processed
        AND (s.resubscribe_status IS NULL OR s.resubscribe_status = 'pending')
        -- Exclude recent attempts (within 24 hours)
        AND (s.last_resubscribe_attempt_at IS NULL 
             OR s.last_resubscribe_attempt_at < NOW() - INTERVAL '24 hours')
    ORDER BY s.id
)
SELECT * FROM charging_failed_subs;
```

## Implementation Strategy

### Step 1: Pre-Processing Analysis

```bash
#!/bin/bash
# Pre-processing analysis script

echo "=== Subscription Charging Failure Analysis ==="

# 1. Get total count of subscriptions with potential charging issues
psql -h localhost -U sm_admin -d subscription_manager <<EOF
SELECT COUNT(DISTINCT s.user_identifier) as total_failed_subs
FROM subscriptions s
LEFT JOIN invalid_msisdn_logs iml ON s.user_identifier = iml.msisdn
WHERE iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE')
   OR iml.response_message LIKE '%charging%';
EOF

# 2. Group by product to understand distribution
psql -h localhost -U sm_admin -d subscription_manager <<EOF
SELECT s.product_id, p.name, COUNT(*) as failure_count
FROM subscriptions s
JOIN products p ON s.product_id::text = p.product_id
LEFT JOIN invalid_msisdn_logs iml ON s.user_identifier = iml.msisdn
WHERE iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE')
GROUP BY s.product_id, p.name
ORDER BY failure_count DESC;
EOF

# 3. Check for duplicates or multiple products per MSISDN
psql -h localhost -U sm_admin -d subscription_manager <<EOF
SELECT user_identifier, COUNT(*) as product_count
FROM subscriptions
WHERE status = 'active'
GROUP BY user_identifier
HAVING COUNT(*) > 1
LIMIT 10;
EOF
```

### Step 2: Enhanced Resubscribe Processor

Key modifications needed to the existing processor:

#### A. Add Duplicate Prevention

```go
// internal/service/subscription.go - Add method to check if already processed
func (s *SubscriptionService) IsAlreadyProcessed(msisdn string, productId string, batchId string) (bool, error) {
    query := `
        SELECT EXISTS(
            SELECT 1 FROM resubscription_tracking 
            WHERE msisdn = $1 
            AND product_id = $2 
            AND process_batch_id = $3
            AND resubscribe_status IN ('success', 'in_progress')
        )
    `
    var exists bool
    err := s.db.QueryRow(query, msisdn, productId, batchId).Scan(&exists)
    return exists, err
}

// Add tracking record before processing
func (s *SubscriptionService) RecordResubscriptionAttempt(msisdn string, productId int, batchId string) error {
    query := `
        INSERT INTO resubscription_tracking 
        (msisdn, product_id, process_batch_id, resubscribe_status, created_at)
        VALUES ($1, $2, $3, 'in_progress', NOW())
        ON CONFLICT (subscription_id, process_batch_id) 
        DO UPDATE SET updated_at = NOW()
    `
    _, err := s.db.Exec(query, msisdn, productId, batchId)
    return err
}
```

#### B. Implement Checkpointing

```go
// internal/handler/subscription_handler.go - Add checkpoint support
type CheckpointManager struct {
    db       *sql.DB
    batchId  string
    interval int // Save checkpoint every N records
}

func (cm *CheckpointManager) SaveCheckpoint(processedCount, successCount, failureCount int, lastId int, lastMsisdn string) error {
    query := `
        UPDATE resubscription_checkpoints 
        SET processed_count = $1,
            success_count = $2,
            failure_count = $3,
            last_processed_id = $4,
            last_processed_msisdn = $5,
            updated_at = NOW()
        WHERE batch_id = $6
    `
    _, err := cm.db.Exec(query, processedCount, successCount, failureCount, lastId, lastMsisdn, cm.batchId)
    return err
}

func (cm *CheckpointManager) LoadCheckpoint() (*Checkpoint, error) {
    query := `
        SELECT processed_count, success_count, failure_count, 
               last_processed_id, last_processed_msisdn
        FROM resubscription_checkpoints
        WHERE batch_id = $1 AND status = 'in_progress'
    `
    var cp Checkpoint
    err := cm.db.QueryRow(query, cm.batchId).Scan(
        &cp.ProcessedCount, &cp.SuccessCount, &cp.FailureCount,
        &cp.LastProcessedId, &cp.LastProcessedMsisdn,
    )
    return &cp, err
}
```

#### C. Add Rate Limiting and Circuit Breaking

```go
// internal/service/rate_limiter.go
type AdaptiveRateLimiter struct {
    baseRate      int           // Base requests per second
    currentRate   int           // Current adaptive rate
    errorThreshold float64      // Error rate threshold (e.g., 0.1 for 10%)
    window        time.Duration // Monitoring window
    ticker        *time.Ticker
    mu            sync.RWMutex
}

func (rl *AdaptiveRateLimiter) AdjustRate(errorRate float64) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    if errorRate > rl.errorThreshold {
        // Reduce rate by 20% if error rate is high
        rl.currentRate = int(float64(rl.currentRate) * 0.8)
        if rl.currentRate < 1 {
            rl.currentRate = 1
        }
    } else if errorRate < rl.errorThreshold/2 {
        // Increase rate by 10% if error rate is low
        rl.currentRate = int(float64(rl.currentRate) * 1.1)
        if rl.currentRate > rl.baseRate*2 {
            rl.currentRate = rl.baseRate * 2
        }
    }
    
    // Reset ticker with new rate
    rl.ticker.Reset(time.Second / time.Duration(rl.currentRate))
}
```

### Step 3: Operational Procedures

#### A. Batch Processing Configuration

```json
{
  "processing_config": {
    "total_records": 25169944,
    "batch_size": 10000,
    "parallel_workers": 50,
    "rate_limit_per_second": 100,
    "checkpoint_interval": 1000,
    "error_threshold": 0.05,
    "retry_attempts": 3,
    "retry_delay": "5s",
    "pause_windows": [
      {"start": "00:00", "end": "06:00"},
      {"start": "12:00", "end": "14:00"}
    ]
  }
}
```

#### B. Monitoring Dashboard Queries

```sql
-- Real-time progress monitoring
SELECT 
    batch_id,
    total_count,
    processed_count,
    ROUND(processed_count::numeric / total_count * 100, 2) as progress_pct,
    success_count,
    failure_count,
    ROUND(failure_count::numeric / NULLIF(processed_count, 0) * 100, 2) as error_rate,
    EXTRACT(EPOCH FROM (NOW() - started_at))/3600 as hours_elapsed,
    CASE 
        WHEN processed_count > 0 THEN 
            ROUND((total_count - processed_count) / 
                  (processed_count / EXTRACT(EPOCH FROM (NOW() - started_at))) / 3600, 2)
        ELSE NULL
    END as estimated_hours_remaining
FROM resubscription_checkpoints
WHERE status = 'in_progress'
ORDER BY started_at DESC;

-- Error analysis
SELECT 
    error_message,
    COUNT(*) as error_count,
    ROUND(COUNT(*)::numeric / SUM(COUNT(*)) OVER() * 100, 2) as error_pct
FROM resubscription_tracking
WHERE process_batch_id = 'current_batch_id'
AND resubscribe_status = 'failed'
GROUP BY error_message
ORDER BY error_count DESC
LIMIT 10;
```

### Step 4: Validation and Rollback Strategy

#### A. Pre-validation Checks

```bash
#!/bin/bash
# pre_validation.sh

echo "Running pre-validation checks..."

# 1. Check database connectivity
pg_isready -h localhost -p 5432 -U sm_admin || exit 1

# 2. Check downstream service health
curl -f http://localhost:8083/health || exit 1

# 3. Check disk space (need at least 50GB for logs and tracking)
available_space=$(df -BG /var/log | awk 'NR==2 {print $4}' | sed 's/G//')
if [ "$available_space" -lt 50 ]; then
    echo "ERROR: Insufficient disk space. Need at least 50GB"
    exit 1
fi

# 4. Create backup of current subscription states
pg_dump -h localhost -U sm_admin -d subscription_manager \
    -t subscriptions -t invalid_msisdn_logs \
    -f /backup/subscriptions_backup_$(date +%Y%m%d_%H%M%S).sql

echo "Pre-validation completed successfully"
```

#### B. Rollback Procedure

```sql
-- Rollback script in case of critical issues
BEGIN;

-- 1. Mark batch as failed
UPDATE resubscription_checkpoints 
SET status = 'failed',
    completed_at = NOW()
WHERE batch_id = 'problematic_batch_id';

-- 2. Revert subscription statuses
UPDATE subscriptions s
SET resubscribe_status = NULL,
    last_resubscribe_attempt_at = NULL
FROM resubscription_tracking rt
WHERE s.id = rt.subscription_id
AND rt.process_batch_id = 'problematic_batch_id'
AND rt.resubscribe_status = 'success';

-- 3. Mark tracking records as rolled back
UPDATE resubscription_tracking
SET resubscribe_status = 'rolled_back',
    updated_at = NOW()
WHERE process_batch_id = 'problematic_batch_id';

COMMIT;
```

## Edge Cases and Special Handling

### 1. Already Active Subscriptions
- **Check**: Verify subscription status before unsubscribe
- **Action**: Skip if already active and working

### 2. Blacklisted/Invalid MSISDNs
- **Check**: Cross-reference with userbase table
- **Action**: Skip and log for analysis

### 3. Concurrent Processing
- **Prevention**: Use database row-level locking
- **Implementation**: SELECT FOR UPDATE SKIP LOCKED

### 4. Partial Failures
- **Scenario**: Unsubscribe succeeds but resubscribe fails
- **Recovery**: Track both operations separately, implement compensation

### 5. Network Timeouts
- **Handling**: Implement exponential backoff
- **Timeout**: 30 seconds for individual requests, circuit breaker after 5 consecutive failures

## Performance Optimizations

### 1. Database Optimizations
```sql
-- Analyze and vacuum tables before processing
ANALYZE subscriptions;
VACUUM ANALYZE invalid_msisdn_logs;

-- Increase work_mem for this session
SET work_mem = '256MB';

-- Use parallel queries
SET max_parallel_workers_per_gather = 4;
```

### 2. Application-Level Optimizations
- Use connection pooling (min: 10, max: 100)
- Implement bulk operations where possible
- Use prepared statements
- Cache product information in memory
- Implement request coalescing for same MSISDN

### 3. Infrastructure Recommendations
- Dedicated database read replica for queries
- Separate worker nodes for processing
- Redis for distributed locking and caching
- Increase database connection limits
- Monitor CPU, memory, and I/O throughout

## Monitoring and Alerting

### Key Metrics to Track
1. **Processing Rate**: Records/second
2. **Error Rate**: Failures/Total
3. **Database Performance**: Query latency, connection pool usage
4. **API Response Times**: P50, P95, P99
5. **System Resources**: CPU, Memory, Disk I/O
6. **Business Metrics**: Successful resubscriptions, Revenue impact

### Alert Thresholds
- Error rate > 5%: Warning
- Error rate > 10%: Critical
- Processing rate < 50/sec: Warning
- Database connections > 80%: Warning
- API timeout rate > 1%: Critical

## Testing Strategy

### 1. Pilot Testing (1000 records)
- Select diverse sample
- Full monitoring enabled
- Manual verification of results
- Performance baseline establishment

### 2. Gradual Rollout
- Phase 1: 10,000 records (0.04%)
- Phase 2: 100,000 records (0.4%)
- Phase 3: 1,000,000 records (4%)
- Phase 4: 5,000,000 records (20%)
- Phase 5: Remaining records

### 3. Validation Checkpoints
After each phase:
- Verify success rate > 95%
- Check for unexpected side effects
- Validate billing system integration
- Customer complaint monitoring

## Success Criteria

1. **Completion Rate**: > 95% of eligible subscriptions processed
2. **Success Rate**: > 90% successful resubscriptions
3. **Performance**: Average processing rate > 100 records/second
4. **Reliability**: No unplanned downtime > 5 minutes
5. **Data Integrity**: Zero data corruption incidents

## Risk Mitigation

### High-Risk Areas
1. **Database Overload**: Implement connection pooling and rate limiting
2. **Downstream Service Failure**: Circuit breaker pattern with fallback
3. **Data Corruption**: Comprehensive audit logging and checksums
4. **Customer Impact**: Gradual rollout with monitoring
5. **Billing Issues**: Coordination with finance team, reconciliation processes

### Contingency Plans
1. **Emergency Stop**: Kill switch to halt all processing
2. **Rollback**: Automated rollback procedures
3. **Manual Intervention**: Admin interface for individual corrections
4. **Communication**: Customer service scripts and FAQs prepared

## Timeline Estimate

### Preparation Phase (1 week)
- Database schema updates
- Code modifications and testing
- Infrastructure setup
- Monitoring dashboard creation

### Pilot Phase (3 days)
- Small-scale testing
- Performance tuning
- Issue resolution

### Production Rollout (2-3 weeks)
- Gradual processing of 25M records
- Assuming 100 records/second average
- 24/7 processing with pause windows
- Buffer for issue resolution

### Post-Processing (3 days)
- Verification and reconciliation
- Report generation
- Clean-up activities

**Total Estimated Timeline: 4-5 weeks**

## Conclusion

This comprehensive strategy addresses the critical aspects of processing 25+ million failed subscriptions:
- **Safety**: Duplicate prevention, checkpointing, rollback capability
- **Efficiency**: Optimized queries, parallel processing, adaptive rate limiting
- **Reliability**: Error handling, monitoring, gradual rollout
- **Visibility**: Comprehensive logging, real-time dashboards, alerting

The implementation should proceed cautiously with continuous monitoring and the ability to pause or rollback at any stage. The gradual rollout approach minimizes risk while allowing for performance optimization and issue resolution.
