# MCC/MNC Configuration Guide

## Overview

The subscription system now supports configurable Mobile Country Code (MCC) and Mobile Network Code (MNC) values. This allows the system to work with different network operators and countries without code changes.

## What are MCC and MNC?

- **MCC (Mobile Country Code)**: A 3-digit code that identifies the country of the mobile network
- **MNC (Mobile Network Code)**: A 2-3 digit code that identifies the specific mobile network operator within a country

## Current Configuration

### Main Configuration (`config.yaml`)

The MCC and MNC values are configured in the `TIMWE_MA` section:

```yaml
TIMWE_MA:
  # ... other settings ...
  
  # Network operator settings
  MCC: "620"  # Mobile Country Code for Ghana
  MNC: "03"   # Mobile Network Code for AirtelTigo
```

### Renewal Configuration (`config/renewal.yaml`)

Additional network operator settings are available in the renewal configuration:

```yaml
# Network operator configuration
network_operator:
  mcc: "620"  # Mobile Country Code for Ghana
  mnc: "03"   # Mobile Network Code for AirtelTigo
  country: "Ghana"
  operator: "AirtelTigo"
  description: "Ghana AirtelTigo network configuration"
```

## Default Values

If MCC or MNC values are not specified in the configuration, the system will use these fallback values:

- **MCC**: "620" (Ghana)
- **MNC**: "03" (AirtelTigo)

## Where MCC/MNC are Used

The configurable MCC/MNC values are used in the following operations:

1. **Subscription Creation**: When creating new subscriptions
2. **Charging Status Checks**: When checking subscription charging status
3. **Notification Creation**: When creating charging status notifications
4. **Renewal Operations**: When processing renewal requests

## Configuration Examples

### Ghana - AirtelTigo
```yaml
MCC: "620"  # Ghana
MNC: "03"   # AirtelTigo
```

### Ghana - MTN
```yaml
MCC: "620"  # Ghana
MNC: "01"   # MTN Ghana
```

### Ghana - Vodafone
```yaml
MCC: "620"  # Ghana
MNC: "02"   # Vodafone Ghana
```

### Nigeria - MTN
```yaml
MCC: "621"  # Nigeria
MNC: "30"   # MTN Nigeria
```

### Kenya - Safaricom
```yaml
MCC: "639"  # Kenya
MNC: "01"   # Safaricom
```

## Environment Variables

You can also set MCC and MNC values using environment variables:

```bash
export APPLICATION_TIMWE_MA_MCC="620"
export APPLICATION_TIMWE_MA_MNC="03"
```

## Validation

The system validates MCC/MNC values:

- **MCC**: Must be a 3-digit string
- **MNC**: Must be a 2-3 digit string
- Invalid values will fall back to defaults

## Migration from Hardcoded Values

If you're upgrading from a version with hardcoded MCC/MNC values:

1. **Before**: Values were hardcoded as "620" and "03"
2. **After**: Values are configurable with the same defaults
3. **No Breaking Changes**: Existing functionality continues to work

## Best Practices

1. **Document Your Values**: Always document which MCC/MNC values you're using
2. **Environment-Specific Configs**: Use different config files for different environments
3. **Validation**: Test your MCC/MNC configuration in a staging environment first
4. **Monitoring**: Monitor logs to ensure the correct values are being used

## Troubleshooting

### Common Issues

1. **Wrong Network**: If you're getting network errors, check your MCC/MNC values
2. **Fallback Values**: If you see "620"/"03" in logs, check your configuration
3. **Configuration Not Loaded**: Ensure your config file is being read correctly

### Debug Commands

Check current configuration values:

```bash
# View current config
curl http://localhost:8083/api/v1/renewal/health

# Check logs for MCC/MNC usage
grep "MCC\|MNC" /path/to/logs/subscription-external-app.log
```

## Related Documentation

- [Renewal System Guide](./RENEWAL_SYSTEM.md)
- [Configuration Reference](./CONFIGURATION.md)
- [Network Operator Integration](./NETWORK_OPERATOR_INTEGRATION.md) 