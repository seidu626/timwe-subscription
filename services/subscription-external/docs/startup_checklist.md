# MSISDN Generator Startup Checklist

## Pre-Startup Checks
- [ ] Redis server is running (for Bloom Filter)
- [ ] PostgreSQL database is accessible
- [ ] Configuration file is properly set
- [ ] Required ports are available (8083)

## Startup Sequence
1. **System initialization**
   - Check Redis availability
   - Validate database connection
   - Load configuration

2. **MSISDN Generator setup**
   - Initialize Bloom Filter (if Redis available)
   - Set up worker pools
   - Configure batch sizes

3. **Performance optimization**
   - Preload Bloom Filter with existing invalid MSISDNs
   - Warm up caches
   - Initialize monitoring

## Post-Startup Verification
- [ ] Health endpoint responds (/health)
- [ ] MSISDN generation is working
- [ ] Performance metrics are being collected
- [ ] Error logs are clean

## Performance Targets
- **Single MSISDN generation**: < 5ms
- **Batch generation (100 MSISDNs)**: < 50ms
- **Bloom Filter hit rate**: > 90%
- **Memory usage**: < 500MB
- **CPU usage**: < 30% under normal load

## Troubleshooting
- If Bloom Filter is disabled, check Redis connection
- If performance is poor, check database indexes
- If memory usage is high, adjust batch sizes
- Monitor logs for any errors or warnings
