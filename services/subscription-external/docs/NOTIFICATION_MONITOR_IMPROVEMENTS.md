# Notification Monitor Improvements

## Overview

The `NotificationMonitor` has been significantly improved to provide better configuration management, enhanced error handling, and more flexible product and entry channel processing.

## Key Improvements

### 1. Enhanced Configuration

#### New Configuration Fields
- **ProductIds**: List of product IDs to process for opt-out notifications
- **EntryChannels**: List of entry channels to use for opt-in attempts
- **DefaultEntryChannel**: Default entry channel if none specified

#### Configuration File Support
- YAML configuration file support (`config/notification-monitor.yaml`)
- Environment variable override (`NOTIFICATION_MONITOR_CONFIG`)
- Runtime configuration validation
- Default fallback values

### 2. Improved Opt-Out Processing

#### Product Filtering
- Only processes notifications for configured products
- Skips unconfigured products with detailed logging
- Configurable product list via configuration file

#### Entry Channel Strategy
- Tries original entry channel first (if configured)
- Falls back to configured entry channels in order
- Removes duplicate channels while preserving order
- Better error handling for channel failures

#### Enhanced Error Handling
- Detailed error categorization
- Better logging with context
- Metrics for different error types
- Batch processing summaries

### 3. Configuration Management

#### YAML Configuration Structure
```yaml
monitor:
  # Processing settings
  batch_size: 2000
  max_in_flight_batches: 20
  scan_lookback_days: 90
  
  # Product configuration
  products:
    product_ids:
      - "8509"    # Default product
      - "14392"   # Premium product
      - "14396"   # Standard product
  
  # Entry channel configuration
  entry_channels:
    channels:
      - "USSD"    # Primary channel
      - "SMS"     # Secondary channel
      - "WEB"     # Tertiary channel
    default: "USSD"
    strategy: "fallback"
```

#### Runtime Configuration Updates
- `UpdateConfiguration()` method for dynamic updates
- Configuration validation before updates
- Logging of configuration changes

### 4. Better Observability

#### Enhanced Metrics
- Product-specific metrics
- Entry channel success/failure tracking
- Batch processing summaries
- Error categorization

#### Improved Logging
- Structured logging with context
- Configurable log levels
- Batch processing summaries
- Channel attempt details

#### Configuration Monitoring
- `GetConfigurationSummary()` method
- `IsProductSupported()` helper
- `GetSupportedProducts()` and `GetSupportedEntryChannels()`

## Usage Examples

### Basic Configuration
```go
config := &NotificationMonitorConfig{
    BatchSize:           2000,
    MaxInFlightBatches:  20,
    ScanLookbackDays:    90,
    ProductIds:          []string{"8509", "14392", "14396"},
    EntryChannels:       []string{"USSD", "SMS", "WEB"},
    DefaultEntryChannel: "USSD",
}

monitor := NewNotificationMonitor(logger, repo, svc, redis, config)
```

### Using Configuration File
```go
// Load from YAML file
config, err := LoadNotificationMonitorConfig("config/notification-monitor.yaml")
if err != nil {
    // Use defaults
    config = &NotificationMonitorConfig{...}
}

monitor := NewNotificationMonitor(logger, repo, svc, redis, *config)
```

### Runtime Configuration Updates
```go
// Update configuration at runtime
newConfig := monitor.GetConfigurationSummary()
newConfig["product_ids"] = []string{"8509", "14392", "14393"}

if err := monitor.UpdateConfiguration(newConfig); err != nil {
    logger.Error("failed to update configuration", zap.Error(err))
}
```

## Migration Guide

### From Previous Version

1. **Update Configuration Structure**
   ```go
   // Old way
   config := NotificationMonitorConfig{
       BatchSize: 1000,
       // ... other fields
   }
   
   // New way
   config := NotificationMonitorConfig{
       BatchSize:           1000,
       ProductIds:          []string{"8509"},           // New
       EntryChannels:       []string{"USSD"},           // New
       DefaultEntryChannel: "USSD",                     // New
       // ... other fields
   }
   ```

2. **Add Configuration File** (Optional)
   - Create `config/notification-monitor.yaml`
   - Define product IDs and entry channels
   - Set environment variable `NOTIFICATION_MONITOR_CONFIG`

3. **Update Main Function**
   ```go
   // Old way
   mon := worker.NewNotificationMonitor(logger, repo, svc, redis, config)
   
   // New way
   monitorConfig, err := worker.LoadNotificationMonitorConfig(configPath)
   if err != nil {
       // Use defaults
   }
   mon := worker.NewNotificationMonitor(logger, repo, svc, redis, *monitorConfig)
   ```

## Configuration Options

### Environment Variables
- `NOTIFICATION_MONITOR_CONFIG`: Path to configuration file

### Default Values
- **ProductIds**: `["8509"]`
- **EntryChannels**: `["USSD"]`
- **DefaultEntryChannel**: `"USSD"`
- **BatchSize**: `1000`
- **MaxInFlightBatches**: `10`
- **ScanLookbackDays**: `60`
- **RenewalWindowMonths**: `2`
- **IdleSleep**: `2s`
- **LeaseTTL**: `30s`
- **RedisKeyPrefix**: `"notifmon"`

## Error Handling

### Error Categories
- `fetch_notifications`: Failed to fetch notifications from database
- `invalid_notification`: Notification with empty MSISDN or ProductID
- `unconfigured_product`: Product not in configured list
- `upsert_inactive`: Failed to mark subscription inactive
- `optin_all_channels_failed`: All opt-in attempts failed
- `mark_active_failed`: Failed to mark subscription active after opt-in

### Error Recovery
- Automatic retry with exponential backoff
- Batch-level error tracking
- Detailed error logging with context
- Graceful degradation for non-critical errors

## Monitoring and Metrics

### Available Metrics
- `notifications_processed_total`: Total notifications processed
- `notifications_errors_total`: Total errors by category
- `optin_success_total`: Successful opt-ins
- `resubscribe_success_total`: Successful resubscriptions

### Custom Labels
- `product_id`: Product ID being processed
- `entry_channel`: Entry channel being used
- `msisdn_prefix`: MSISDN prefix for categorization

## Best Practices

### Configuration Management
1. Use YAML configuration files for production
2. Set appropriate product IDs based on business requirements
3. Configure entry channels in order of preference
4. Validate configuration before deployment

### Error Handling
1. Monitor error rates by category
2. Set appropriate alerting thresholds
3. Review logs for configuration issues
4. Test with different product configurations

### Performance Tuning
1. Adjust batch sizes based on system capacity
2. Monitor Redis lease TTL for multi-instance deployments
3. Set appropriate scan lookback periods
4. Use batch delays to avoid overwhelming downstream services

## Troubleshooting

### Common Issues

#### Configuration Not Loading
- Check file path and permissions
- Verify YAML syntax
- Check environment variable settings
- Review default fallback values

#### Products Not Processing
- Verify product IDs in configuration
- Check product ID format (string vs int)
- Review product filtering logic
- Check logs for "unconfigured_product" errors

#### Entry Channel Failures
- Verify entry channel configuration
- Check channel order and fallback logic
- Review opt-in service configuration
- Monitor channel-specific error rates

### Debug Mode
Enable debug logging to see detailed processing information:
```yaml
monitor:
  logging:
    level: "debug"
    log_skipped_notifications: true
    log_channel_failures: true
```

## Future Enhancements

### Planned Features
- Dynamic product configuration via API
- A/B testing for entry channel strategies
- Machine learning for optimal channel selection
- Integration with external configuration services

### Configuration Extensions
- Product-specific entry channel preferences
- Time-based channel availability
- Geographic channel restrictions
- Performance-based channel selection 