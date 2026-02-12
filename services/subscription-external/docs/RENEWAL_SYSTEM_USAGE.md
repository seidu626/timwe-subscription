# Renewal System Usage Guide

## Overview

The Opt-Out/Opt-In Renewal System is a comprehensive solution for handling subscription renewals efficiently while managing edge cases. This system provides automated renewal processing, churn management, and comprehensive monitoring.

## 🚀 Quick Start

### 1. Deploy the System

```bash
# Run the deployment script
cd services/subscription-external
sudo ./scripts/deploy_renewal_system.sh
```

This script will:
- Create necessary directories and set permissions
- Run database migrations
- Copy configuration files
- Build and install the renewal worker
- Set up systemd service and cron jobs
- Configure monitoring

### 2. Start the Service

```bash
# Start the main subscription service
cd services/subscription-external
go run cmd/main.go

# Or start the renewal worker separately
sudo systemctl start renewal-worker
sudo systemctl enable renewal-worker
```

### 3. Test the System

```bash
# Run the test script
./scripts/test_renewal_system.sh
```

## 📋 API Endpoints

### Worker Management

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/renewal/worker/start` | POST | Start the renewal worker |
| `/api/v1/renewal/worker/stop` | POST | Stop the renewal worker |
| `/api/v1/renewal/worker/status` | GET | Get worker status and metrics |

### Monitoring & Statistics

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/renewal/statistics` | GET | Get renewal statistics (days parameter) |
| `/api/v1/renewal/churn-candidates` | GET | Get subscriptions eligible for churn |
| `/api/v1/renewal/health` | GET | Get system health status |
| `/api/v1/renewal/priority-retry/process` | POST | Process priority retry queue |

### Manual Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/renewal/manual` | POST | Trigger manual renewal for a subscription |
| `/api/v1/renewal/force-churn-evaluation` | POST | Force churn evaluation for all subscriptions |

## 🔧 Configuration

### Configuration File: `config/renewal.yaml`

```yaml
strategy: "opt_out_opt_in"
enabled: true

churn_policy:
  max_days_without_payment: 30
  max_renewal_attempts: 3
  retry_interval_hours: 24
  grace_period_days: 7
  safe_mode: true

opt_out_opt_in:
  wait_between_ms: 5000      # 5 seconds between opt-out and opt-in
  batch_size: 100            # Process 100 subscriptions per batch
  max_concurrent: 10         # Maximum concurrent renewals
  rate_limit_ms: 1000        # 1 second between API calls
  batch_delay_ms: 5000       # 5 seconds between batches

worker:
  enabled: true
  daily_run_time: "02:00"    # Run at 2 AM
  timezone: "UTC"
  timeout_per_renewal: 30s
  max_retries: 3

monitoring:
  alert_on_failure_rate: 0.1  # Alert if >10% failures
  alert_on_churn_rate: 0.05   # Alert if >5% churn rate
  metrics_port: 9090
```

### Environment Variables

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=subscription_db
DB_USER=subscription_user
DB_PASSWORD=your_password

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# TIMWE API
TIMWE_API_URL=https://api.timwe.com
TIMWE_API_KEY=your_api_key
```

## 📊 Monitoring & Metrics

### Prometheus Metrics

The system exposes the following metrics:

- `renewal_cycles_total` - Total renewal cycles processed
- `renewal_success_total` - Successful renewals
- `renewal_failure_total` - Failed renewals
- `churn_total` - Total churned subscriptions
- `priority_retry_queue_size` - Current retry queue size
- `renewal_worker_running` - Worker status (1=running, 0=stopped)

### Health Checks

```bash
# Check worker health
curl http://localhost:8083/api/v1/renewal/health

# Check system health
curl http://localhost:8083/health
```

### Logs

```bash
# View renewal worker logs
sudo journalctl -u renewal-worker -f

# View application logs
tail -f logs/renewal_worker.log
```

## 🗄️ Database Schema

### New Tables

1. **`renewal_cycles`** - Tracks each renewal attempt
2. **`churn_tracking`** - Records churned subscriptions
3. **`priority_retry_queue`** - Manages failed opt-ins

### New Columns in `subscriptions`

- `renewal_status` - Current renewal status
- `last_renewal_attempt` - Timestamp of last attempt
- `total_renewal_attempts` - Count of attempts
- `last_successful_payment` - Last payment date
- `consecutive_payment_failures` - Payment failure count

### Database Functions

- `get_subscriptions_needing_renewal()` - Find subscriptions for renewal
- `get_renewal_statistics()` - Calculate renewal metrics
- `get_churn_candidates()` - Identify churn candidates
- `increment_renewal_attempt()` - Update attempt counter
- `churn_subscription()` - Mark subscription as churned

## 🔄 Renewal Process Flow

### 1. Daily Processing
- Worker runs at configured time (default: 2 AM)
- Identifies subscriptions needing renewal
- Processes in configurable batches

### 2. Opt-Out/Opt-In Cycle
```
Subscription → Opt-Out → Wait (5s) → Opt-In → Monitor Billing
```

### 3. Churn Evaluation
- Evaluates each subscription against churn policy
- Applies grace periods if configured
- Churns subscriptions that exceed thresholds

### 4. Priority Retry Queue
- Failed opt-ins are added to priority queue
- Immediate retry with exponential backoff
- Alerts sent for critical failures

## 🚨 Alerting & Notifications

### Critical Alerts
- Failed opt-in after successful opt-out
- High failure rates (>10%)
- High churn rates (>5%)
- Worker stopped unexpectedly

### Alert Channels
- Webhook notifications
- Email alerts
- Slack/Teams integration
- SMS notifications

## 🧪 Testing

### Unit Tests

```bash
# Run renewal service tests
go test ./internal/service -v

# Run repository tests
go test ./internal/repository -v

# Run worker tests
go test ./internal/worker -v
```

### Integration Tests

```bash
# Test with real database
go test ./internal/service -v -tags=integration

# Test API endpoints
./scripts/test_renewal_system.sh
```

### Load Testing

```bash
# Test with high volume
go run cmd/load_test/main.go --subscriptions=10000 --concurrent=100
```

## 🔍 Troubleshooting

### Common Issues

1. **Worker Not Starting**
   ```bash
   # Check service status
   sudo systemctl status renewal-worker
   
   # Check logs
   sudo journalctl -u renewal-worker -f
   
   # Check configuration
   cat /etc/subscription/renewal.yaml
   ```

2. **Database Connection Issues**
   ```bash
   # Test database connectivity
   psql -h localhost -U subscription_user -d subscription_db
   
   # Check connection pool
   SELECT * FROM pg_stat_activity WHERE datname = 'subscription_db';
   ```

3. **High Failure Rates**
   ```bash
   # Check TIMWE API status
   curl -H "Authorization: Bearer $TIMWE_API_KEY" $TIMWE_API_URL/health
   
   # Review recent failures
   SELECT * FROM renewal_cycles WHERE opt_in_status = 'FAILED' ORDER BY created_at DESC LIMIT 10;
   ```

### Performance Tuning

1. **Database Optimization**
   ```sql
   -- Add indexes for better performance
   CREATE INDEX CONCURRENTLY idx_renewal_cycles_status ON renewal_cycles(status);
   CREATE INDEX CONCURRENTLY idx_subscriptions_renewal_status ON subscriptions(renewal_status);
   ```

2. **Worker Tuning**
   ```yaml
   opt_out_opt_in:
     batch_size: 200          # Increase batch size
     max_concurrent: 20       # Increase concurrency
     rate_limit_ms: 500       # Reduce rate limiting
   ```

3. **Memory Management**
   ```yaml
   worker:
     timeout_per_renewal: 60s  # Increase timeout
     max_retries: 5           # Increase retries
   ```

## 📈 Scaling Considerations

### Horizontal Scaling
- Multiple worker instances
- Load balancer for API endpoints
- Database read replicas
- Redis cluster for caching

### Vertical Scaling
- Increase worker memory limits
- Optimize database queries
- Use connection pooling
- Implement caching layers

## 🔐 Security

### Authentication
- API key authentication
- JWT tokens for web interfaces
- Rate limiting per client
- IP whitelisting

### Data Protection
- Encrypted database connections
- Secure API communication
- Audit logging
- Data retention policies

## 📚 Additional Resources

### Documentation
- [OPTOUT_OPTIN_RENEWAL_GUIDE.md](./OPTOUT_OPTIN_RENEWAL_GUIDE.md) - Detailed implementation guide
- [API Documentation](./docs/) - Swagger/OpenAPI specs
- [Database Schema](./migrations/) - Migration scripts

### Support
- Development Team: dev@company.com
- Operations Team: ops@company.com
- Emergency Contact: +1-555-0123

### Monitoring Dashboards
- Grafana: http://localhost:3000
- Prometheus: http://localhost:9090
- Application: http://localhost:8083

## 🎯 Best Practices

1. **Start Small**: Begin with a small batch size and increase gradually
2. **Monitor Closely**: Watch metrics and logs during initial deployment
3. **Test Thoroughly**: Use staging environment for testing
4. **Backup Data**: Regular database backups before major changes
5. **Document Changes**: Keep track of configuration changes
6. **Plan Rollbacks**: Have rollback procedures ready

## 🚀 Deployment Checklist

- [ ] Database migrations applied
- [ ] Configuration files in place
- [ ] Worker service installed and configured
- [ ] Monitoring and alerting set up
- [ ] API endpoints tested
- [ ] Load testing completed
- [ ] Documentation updated
- [ ] Team trained on new system
- [ ] Rollback plan ready
- [ ] Go-live approved

---

**Note**: This system is designed to handle high-volume subscription renewals efficiently while maintaining data integrity and providing comprehensive monitoring. Always test thoroughly in a staging environment before deploying to production. 