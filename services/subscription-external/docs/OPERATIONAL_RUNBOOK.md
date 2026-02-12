# Operational Runbook for Charging Failed Resubscriptions

## Table of Contents
1. [Overview](#overview)
2. [Pre-Production Checklist](#pre-production-checklist)
3. [Pilot Test Procedure](#pilot-test-procedure)
4. [Production Rollout](#production-rollout)
5. [Monitoring](#monitoring)
6. [Troubleshooting](#troubleshooting)
7. [Emergency Procedures](#emergency-procedures)
8. [Post-Processing](#post-processing)

## Overview

This runbook covers the operational procedures for processing 25+ million subscriptions with charging failures.

### Key Metrics
- **Total Failed Subscriptions**: 25,169,944
- **Target Success Rate**: > 90%
- **Processing Rate**: 100-200 records/second
- **Estimated Duration**: 2-3 weeks

### System Components
- Resubscription Processor
- Tracking Database
- Monitoring Dashboard
- Rate Limiter
- Circuit Breaker

## Pre-Production Checklist

### 1. Database Preparation
```bash
# Apply migration
cd /home/xper626/Documents/repositories/timwe-subscription/services/subscription-external
./scripts/apply_migration.sh

# Verify tables created
psql -d subscription_manager -c "
    SELECT table_name FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name LIKE 'resubscription%'
"
```

### 2. System Health Check
```bash
# Run pre-flight checks
./scripts/preflight_check.sh

# Expected output:
# ✅ Database connectivity
# ✅ Service health
# ✅ Sufficient disk space (>50GB)
# ✅ Memory available (>8GB)
```

### 3. Backup Current State
```bash
# Create database backup
pg_dump -h localhost -U sm_admin -d subscription_manager \
    -t subscriptions -t invalid_msisdn_logs \
    -f backup_$(date +%Y%m%d).sql

# Verify backup
ls -lh backup_*.sql
```

## Pilot Test Procedure

### Step 1: Run Small Pilot (1,000 records)
```bash
# Execute pilot test
./scripts/pilot_test.sh 1000

# Monitor progress
./scripts/monitor_resubscription.sh --auto
```

### Step 2: Analyze Results
```sql
-- Check success rate
SELECT 
    batch_id,
    total_count,
    success_count,
    failure_count,
    ROUND(success_count::numeric / processed_count * 100, 2) as success_rate
FROM resubscription_checkpoints
WHERE batch_id LIKE 'pilot-%'
ORDER BY started_at DESC;

-- Analyze errors
SELECT 
    error_message,
    COUNT(*) as count
FROM resubscription_tracking
WHERE process_batch_id LIKE 'pilot-%'
AND resubscribe_status = 'failed'
GROUP BY error_message
ORDER BY count DESC;
```

### Step 3: Validation Criteria
- [ ] Success rate > 85%
- [ ] No critical errors
- [ ] Processing rate > 50/sec
- [ ] No system overload
- [ ] Customer complaints < 5

## Production Rollout

### Phase 1: Initial Batch (10,000)
```bash
# Create request
cat > batch_10k.json << EOF
{
    "batch_id": "prod-10k-$(date +%Y%m%d)",
    "telco": "AirtelTigo",
    "entry_channels": ["USSD", "SMS", "WEB"],
    "use_charging_failures": true,
    "batch_size": 10000,
    "max_workers": 20,
    "rate_limit_per_second": 50,
    "checkpoint_interval": 500
}
EOF

# Submit request
curl -X POST \
    -H "Content-Type: application/json" \
    -d @batch_10k.json \
    http://localhost:8083/api/v1/subscription-external/resubscribe/enhanced
```

### Phase 2: Scale Up (100,000)
```bash
# Increase batch size and workers
cat > batch_100k.json << EOF
{
    "batch_id": "prod-100k-$(date +%Y%m%d)",
    "telco": "AirtelTigo",
    "entry_channels": ["USSD", "SMS", "WEB"],
    "use_charging_failures": true,
    "batch_size": 100000,
    "max_workers": 50,
    "rate_limit_per_second": 100,
    "checkpoint_interval": 1000
}
EOF

# Submit and monitor
curl -X POST -H "Content-Type: application/json" -d @batch_100k.json \
    http://localhost:8083/api/v1/subscription-external/resubscribe/enhanced
```

### Phase 3: Full Production (Remaining)
```bash
# Process all remaining with windowing
for i in {1..20}; do
    START=$((($i - 1) * 1000000))
    END=$(($i * 1000000))
    
    cat > batch_${i}m.json << EOF
    {
        "batch_id": "prod-batch-${i}m",
        "telco": "AirtelTigo",
        "entry_channels": ["USSD", "SMS", "WEB"],
        "use_charging_failures": true,
        "batch_size": 1000000,
        "max_workers": 100,
        "rate_limit_per_second": 150,
        "checkpoint_interval": 5000,
        "start_index": $START,
        "end_index": $END
    }
EOF
    
    curl -X POST -H "Content-Type: application/json" -d @batch_${i}m.json \
        http://localhost:8083/api/v1/subscription-external/resubscribe/enhanced
    
    # Wait between batches
    sleep 3600
done
```

## Monitoring

### Real-Time Dashboard
```bash
# Start monitoring dashboard
./scripts/monitor_resubscription.sh --auto

# Monitor specific batch
./scripts/monitor_resubscription.sh "prod-batch-1m"
```

### Key Metrics to Watch
1. **Processing Rate**: Should be > 100/sec
2. **Error Rate**: Should be < 5%
3. **Database Connections**: Should be < 80% of max
4. **Memory Usage**: Should be < 90%
5. **API Response Time**: Should be < 500ms

### Alert Thresholds
```sql
-- Check for high error rates
SELECT batch_id, 
       ROUND(failure_count::numeric / processed_count * 100, 2) as error_rate
FROM resubscription_checkpoints
WHERE status = 'in_progress'
AND failure_count::numeric / NULLIF(processed_count, 0) > 0.05;

-- Check for stalled batches
SELECT batch_id, 
       EXTRACT(EPOCH FROM (NOW() - updated_at))/60 as minutes_since_update
FROM resubscription_checkpoints
WHERE status = 'in_progress'
AND updated_at < NOW() - INTERVAL '10 minutes';
```

## Troubleshooting

### Issue: High Error Rate (>10%)

**Symptoms**: Error rate exceeds 10%

**Check**:
```sql
-- Identify error patterns
SELECT error_message, COUNT(*) 
FROM resubscription_tracking
WHERE process_batch_id = 'current_batch_id'
AND resubscribe_status = 'failed'
GROUP BY error_message
ORDER BY COUNT(*) DESC;
```

**Actions**:
1. Reduce rate limit
2. Check downstream service health
3. Verify network connectivity
4. Review error logs

### Issue: Processing Stalled

**Symptoms**: No progress for >10 minutes

**Check**:
```bash
# Check process status
ps aux | grep resubscribe

# Check database locks
psql -d subscription_manager -c "
    SELECT pid, usename, query, state
    FROM pg_stat_activity
    WHERE state != 'idle'
    AND query LIKE '%resubscription%'
"
```

**Actions**:
1. Check for database locks
2. Restart from checkpoint
3. Reduce worker count
4. Check system resources

### Issue: Database Connection Exhaustion

**Symptoms**: "too many connections" errors

**Check**:
```sql
-- Current connections
SELECT COUNT(*) FROM pg_stat_activity;

-- Kill idle connections
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'idle'
AND state_change < NOW() - INTERVAL '10 minutes';
```

**Actions**:
1. Reduce max_workers
2. Implement connection pooling
3. Increase max_connections
4. Restart database if needed

## Emergency Procedures

### Emergency Stop
```bash
# Stop specific batch
curl -X POST \
    -H "Content-Type: application/json" \
    -d '{"batch_id": "prod-batch-1m", "reason": "Emergency stop"}' \
    http://localhost:8083/api/v1/subscription-external/batch/stop

# Stop all processing
psql -d subscription_manager -c "
    UPDATE resubscription_checkpoints
    SET status = 'cancelled'
    WHERE status = 'in_progress'
"
```

### Rollback Procedure
```sql
BEGIN;

-- Mark batch as failed
UPDATE resubscription_checkpoints 
SET status = 'failed',
    completed_at = NOW()
WHERE batch_id = 'problematic_batch_id';

-- Revert subscription statuses
UPDATE subscriptions s
SET resubscribe_status = NULL,
    last_resubscribe_attempt_at = NULL
FROM resubscription_tracking rt
WHERE s.id = rt.subscription_id
AND rt.process_batch_id = 'problematic_batch_id';

-- Mark tracking as rolled back
UPDATE resubscription_tracking
SET resubscribe_status = 'rolled_back'
WHERE process_batch_id = 'problematic_batch_id';

COMMIT;
```

### Recovery from Checkpoint
```bash
# Resume from last checkpoint
cat > resume.json << EOF
{
    "batch_id": "existing_batch_id",
    "resume_from_checkpoint": true
}
EOF

curl -X POST -H "Content-Type: application/json" -d @resume.json \
    http://localhost:8083/api/v1/subscription-external/resubscribe/enhanced
```

## Post-Processing

### Generate Final Report
```bash
# Generate comprehensive report
./scripts/monitor_resubscription.sh "batch_id" --report

# Export detailed statistics
psql -d subscription_manager -c "
    SELECT 
        batch_id,
        total_count,
        processed_count,
        success_count,
        failure_count,
        started_at,
        completed_at,
        EXTRACT(EPOCH FROM (completed_at - started_at))/3600 as duration_hours
    FROM resubscription_checkpoints
    WHERE batch_id LIKE 'prod-%'
    ORDER BY started_at
" > final_report.csv
```

### Reconciliation
```sql
-- Verify all targeted subscriptions were processed
WITH target_subs AS (
    SELECT COUNT(DISTINCT s.id) as total
    FROM subscriptions s
    LEFT JOIN invalid_msisdn_logs iml ON s.user_identifier = iml.msisdn
    WHERE iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE')
),
processed_subs AS (
    SELECT COUNT(DISTINCT subscription_id) as processed
    FROM resubscription_tracking
    WHERE process_batch_id LIKE 'prod-%'
)
SELECT 
    t.total as target_total,
    p.processed as actual_processed,
    ROUND(p.processed::numeric / t.total * 100, 2) as coverage_pct
FROM target_subs t, processed_subs p;
```

### Clean Up
```bash
# Archive logs
tar -czf resubscription_logs_$(date +%Y%m%d).tar.gz logs/

# Clean up temporary files
rm -f /tmp/pilot_*.json
rm -f /tmp/batch_*.json

# Vacuum database
psql -d subscription_manager -c "VACUUM ANALYZE resubscription_tracking;"
```

## Contact Information

### Escalation Path
1. **Level 1**: DevOps Team - devops@company.com
2. **Level 2**: Database Admin - dba@company.com
3. **Level 3**: Engineering Lead - lead@company.com

### External Dependencies
- TIMWE API Team: api-support@timwe.com
- Telco Support: support@airteltigo.com

## Appendix

### Useful Queries
```sql
-- Daily processing summary
SELECT 
    DATE(created_at) as date,
    COUNT(*) as total_processed,
    SUM(CASE WHEN resubscribe_status = 'success' THEN 1 ELSE 0 END) as success,
    SUM(CASE WHEN resubscribe_status = 'failed' THEN 1 ELSE 0 END) as failed
FROM resubscription_tracking
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- Product-wise success rate
SELECT 
    product_id,
    COUNT(*) as total,
    SUM(CASE WHEN resubscribe_status = 'success' THEN 1 ELSE 0 END) as success,
    ROUND(SUM(CASE WHEN resubscribe_status = 'success' THEN 1 ELSE 0 END)::numeric / COUNT(*) * 100, 2) as success_rate
FROM resubscription_tracking
GROUP BY product_id
ORDER BY total DESC;
```

### Configuration Templates
- [Pilot Configuration](./configs/pilot_config.json)
- [Production Configuration](./configs/prod_config.json)
- [Emergency Stop Configuration](./configs/emergency_config.json)

---

**Document Version**: 1.0
**Last Updated**: 2025-01-20
**Author**: DevOps Team
