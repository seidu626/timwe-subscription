# Opt-Out/Opt-In Renewal System

## Overview

The Opt-Out/Opt-In Renewal System is a comprehensive solution for handling subscription renewals when TIMWE's charging endpoint is not functional. This system implements an innovative approach that triggers TIMWE's internal billing system through a controlled unsubscribe/resubscribe cycle.

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

## Key Features

- **Opt-Out/Opt-In Strategy**: Unsubscribe and resubscribe to trigger TIMWE's billing
- **Intelligent Churn Management**: Configurable policies for subscription lifecycle
- **Priority Retry Queue**: Immediate retry for failed opt-ins
- **Comprehensive Monitoring**: Metrics, alerts, and health checks
- **Edge Case Handling**: Duplicate prevention, rate limiting, circuit breakers
- **Automated Processing**: Scheduled worker with batch processing
- **Database Tracking**: Complete audit trail of renewal cycles

## Components

### 1. Domain Models (`internal/domain/renewal.go`)

- **RenewalCycle**: Tracks opt-out/opt-in cycles
- **ChurnPolicy**: Configurable churn rules
- **PriorityRetryQueue**: Failed opt-in retry management
- **RenewalConfig**: System configuration

### 2. Repository Layer (`internal/repository/renewal_repository.go`)

- Database operations for renewal tracking
- Churn management
- Priority retry queue operations
- Statistics and monitoring queries

### 3. Service Layer (`internal/service/renewal_service.go`)

- Core renewal logic
- Opt-out/opt-in orchestration
- Churn policy evaluation
- Edge case handling

### 4. Worker (`internal/worker/renewal_worker.go`)

- Automated renewal processing
- Scheduled execution
- Batch processing with concurrency control
- Health monitoring

### 5. Database Schema (`migrations/003_renewal_optout_optin.sql`)

- Renewal cycles tracking
- Churn history
- Priority retry queue
- Performance indexes

## Configuration

The system is configured through `config/renewal.yaml`:

```yaml
renewal:
  strategy: "opt_out_opt_in"
  enabled: true
  
  churn_policy:
    max_days_without_payment: 7
    max_renewal_attempts: 3
    retry_interval_hours: 24
    grace_period_days: 2
    safe_mode: true
    
  opt_out_opt_in:
    wait_between_ms: 3000
    batch_size: 50
    max_concurrent: 5
    rate_limit_ms: 500
    batch_delay_ms: 2000
```

## Edge Cases Handled

### 1. Failed Opt-In After Successful Opt-Out

**Problem**: User is unsubscribed but resubscription fails
**Solution**: 
- Immediate addition to priority retry queue
- Critical alert notification
- Exponential backoff retry strategy

### 2. Duplicate Subscription Prevention

**Problem**: Multiple renewal attempts creating duplicates
**Solution**:
- Subscription state tracking
- Unique constraint enforcement
- Rate limiting per MSISDN

### 3. High Volume Processing

**Problem**: System overload during peak renewal periods
**Solution**:
- Configurable batch sizes
- Concurrency limits
- Rate limiting between operations
- Circuit breaker pattern

### 4. Database Connection Management

**Problem**: Connection pool exhaustion
**Solution**:
- Connection pooling configuration
- Query timeouts
- Transaction management
- Connection health checks

### 5. TIMWE API Failures

**Problem**: External API unavailability
**Solution**:
- Circuit breaker implementation
- Retry with exponential backoff
- Fallback to local processing
- Alert notifications

## Deployment

### Prerequisites

- PostgreSQL database
- Redis (optional, for caching)
- Go 1.19+
- Systemd (for service management)

### Quick Start

1. **Clone and navigate to the project**:
   ```bash
   cd services/subscription-external
   ```

2. **Run database migration**:
   ```bash
   psql -U your_user -d your_db -f migrations/003_renewal_optout_optin.sql
   ```

3. **Deploy using the script**:
   ```bash
   chmod +x scripts/deploy_renewal_system.sh
   ./scripts/deploy_renewal_system.sh
   ```

### Manual Deployment

1. **Build the worker**:
   ```bash
   go build -o renewal-worker ./cmd/renewal-worker
   ```

2. **Create systemd service**:
   ```bash
   sudo cp renewal-worker /usr/local/bin/
   sudo systemctl enable renewal-worker
   sudo systemctl start renewal-worker
   ```

3. **Configure monitoring**:
   ```bash
   sudo cp config/renewal.yaml /etc/subscription/
   sudo systemctl restart renewal-worker
   ```

## Monitoring and Alerts

### Health Checks

- Worker process status
- Database connectivity
- TIMWE API availability
- Queue processing status

### Metrics

- Renewal success rate
- Opt-out/opt-in cycle time
- Churn rate
- Priority retry queue size
- Processing latency

### Alerts

- Worker down
- High failure rate (>30%)
- High churn rate (>10%)
- Stuck renewals
- Queue overflow

## API Endpoints

### Renewal Management

- `POST /api/renewal/request` - Manual renewal request
- `GET /api/renewal/status/{msisdn}` - Renewal status
- `GET /api/renewal/statistics` - System statistics
- `POST /api/renewal/retry` - Force retry of failed renewal

### Monitoring

- `GET /health` - System health check
- `GET /metrics` - Prometheus metrics
- `GET /api/renewal/worker/status` - Worker status

## Testing

### Unit Tests

```bash
go test ./internal/service -v
go test ./internal/repository -v
go test ./internal/worker -v
```

### Integration Tests

```bash
# Test with real database
go test ./internal/... -tags=integration -v
```

### Load Testing

```bash
# Simulate high volume renewals
go run cmd/load-test/main.go -concurrent=100 -duration=5m
```

## Troubleshooting

### Common Issues

1. **Worker not starting**:
   - Check database connectivity
   - Verify configuration file permissions
   - Check systemd service status

2. **High failure rate**:
   - Review TIMWE API responses
   - Check rate limiting configuration
   - Verify database performance

3. **Memory issues**:
   - Adjust batch sizes
   - Reduce concurrency limits
   - Monitor goroutine count

### Log Analysis

```bash
# View worker logs
sudo journalctl -u renewal-worker -f

# Check specific errors
sudo journalctl -u renewal-worker | grep ERROR

# Monitor renewal processing
tail -f /var/log/subscription/renewal-worker.log
```

### Database Queries

```sql
-- Check stuck renewals
SELECT COUNT(*) FROM renewal_cycles 
WHERE billing_status = 'PENDING' 
AND created_at < NOW() - INTERVAL '1 hour';

-- View renewal statistics
SELECT * FROM get_renewal_statistics(24);

-- Check churn candidates
SELECT * FROM get_churn_candidates(7, 3, 100);
```

## Performance Tuning

### Database Optimization

- Ensure indexes are created
- Monitor query performance
- Adjust connection pool size
- Use read replicas for reporting

### Worker Tuning

- Adjust batch sizes based on system capacity
- Configure appropriate concurrency limits
- Set optimal wait times between operations
- Monitor memory usage

### Network Optimization

- Use connection pooling for TIMWE API
- Implement circuit breakers
- Configure appropriate timeouts
- Monitor API response times

## Security Considerations

### Data Protection

- MSISDN encryption at rest
- Audit logging for all operations
- Access control for admin functions
- Secure configuration management

### API Security

- Rate limiting per client
- Input validation and sanitization
- Authentication and authorization
- Request/response logging

### Infrastructure Security

- Network isolation
- Service account restrictions
- File system permissions
- Regular security updates

## Maintenance

### Regular Tasks

- Monitor log files for errors
- Check system resource usage
- Review alert thresholds
- Update configuration as needed

### Backup and Recovery

- Database backups (daily)
- Configuration backups
- Log rotation and cleanup
- Disaster recovery testing

### Updates

- Monitor for security patches
- Test updates in staging
- Plan maintenance windows
- Document changes

## Support

### Documentation

- This README
- API documentation
- Configuration reference
- Troubleshooting guide

### Monitoring

- System health dashboard
- Alert notifications
- Performance metrics
- Error tracking

### Contact

For technical support or questions about the renewal system, please contact the development team or refer to the project documentation.

## License

This renewal system is part of the TIMWE Subscription Manager project. Please refer to the main project license for usage terms and conditions.

---

**Note**: This system is designed to work around TIMWE's charging endpoint limitations. When the charging endpoint becomes functional, the system can be configured to use direct charging instead of the opt-out/opt-in strategy. 