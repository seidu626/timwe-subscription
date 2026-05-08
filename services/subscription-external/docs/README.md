# Subscription Batch Processor

A background service/cron job for processing subscription batch opt-ins with incrementing count values.

## Overview

This service calls the `BatchOptinHandler` endpoint with varying request data, incrementing the count from 1,000 upwards with an increment of 1,000, up to a maximum of 5,000,000.

## Features

- **Incremental Processing**: Processes batches with configurable count increments
- **Configurable Parameters**: Telco, entry channel, product IDs, and more
- **Retry Logic**: Automatic retry with configurable attempts and delays
- **Async Job Polling**: Enqueues job (202 Accepted) then polls `/api/v1/subscription-external/batch?jobId=...` until completion
- **Progress Tracking**: Real-time logging and progress reporting
- **Result Persistence**: Saves batch results to JSON files
- **Graceful Shutdown**: Handles system signals for clean termination
- **Multiple Run Modes**: One-time, continuous, or dry-run modes
- **Prometheus Metrics**: Exposes `/metrics` with counters, histograms, and gauges
- **Pause/Resume Windows**: Configurable daily windows to pause processing (e.g., 22:00-06:00)

## Installation

### Building the Binary

```bash
cd /home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/cmd/batch-processor
make build
```

## Configuration

### Configuration File (config.json)

```json
{
  "base_url": "http://localhost:8083",
  "start_count": 1000,
  "max_count": 5000000,
  "increment": 1000,
  "telco": "AirtelTigo",
  "entry_channel": "USSD",
  "product_ids": ["8509"],
  "wait_between_calls": "30s",
  "max_retries": 3,
  "retry_delay": "5s",
  "poll_interval": "2s"
}
```

### Command Line Options

```
-config string        Path to configuration file
-url string           Base URL of the subscription service (default "http://localhost:8080")
-start int            Starting count value (default 1000)
-max int              Maximum count value (default 5000000)
-increment int        Increment value (default 1000)
-telco string         Telco name (default "AirtelTigo")
-channel string       Entry channel (default "USSD")
-channels string      Comma-separated entry channels for rotation (e.g., USSD,WEB,SMS)
-products string      Comma-separated product IDs (default "8509")
-wait duration        Wait time between calls (default 30s)
-poll duration        Polling interval for job status (default 2s)
-metrics              Enable Prometheus metrics endpoint (default true)
-metrics-addr string  Metrics bind address (default ":9101")
-timezone string      IANA timezone for pause windows (e.g. Africa/Accra)
-max-poll duration    Optional maximum duration to poll a job before failing (0=disabled)
-pause-windows string Semicolon-separated HH:MM-HH:MM windows (e.g. "22:00-06:00;12:00-13:00")
-debug                Enable debug logging
-once                 Run only once for the start count value
-dry-run              Dry run mode - only log what would be done
```

### Entry Channel Rotation

The batch processor supports rotating between multiple entry channels to distribute load across different subscription channels:

#### Configuration File
```json
{
  "entry_channels": ["USSD", "WEB", "SMS"]
}
```

#### Command Line
```bash
# Rotate between USSD, WEB, and SMS channels
./batch-processor -channels "USSD,WEB,SMS"

# Use single channel (legacy behavior)
./batch-processor -channel "USSD"
```

#### How It Works
- Each batch request uses the next channel in the rotation sequence
- Channels rotate in order: USSD → WEB → SMS → USSD → ...
- If only one channel is specified, it behaves as before (no rotation)
- The `-channels` flag takes precedence over `-channel` when both are specified

## Usage

### 1. Manual Execution

```bash
# Run with default configuration
make run

# Run with custom parameters
./batch-processor -start 1000 -max 10000 -increment 500 -telco "AirtelTigo"

# Dry run to see what would be executed
make dry-run

# Run once for testing
make run-once

# Test with small batch
make test
```

### 2. Systemd Service (Background Service)

#### Install the service:
```bash
sudo make install-service
```

#### Start the service:
```bash
sudo systemctl start batch-processor
```

#### Check service status:
```bash
sudo systemctl status batch-processor
```

#### View logs:
```bash
sudo journalctl -u batch-processor -f
```

#### Stop the service:
```bash
sudo systemctl stop batch-processor
```

#### Uninstall the service:
```bash
sudo make uninstall-service
```

### 3. Cron Job (Scheduled Execution)

#### Make the script executable:
```bash
chmod +x run_batch_processor.sh
```

#### Add to crontab:
```bash
crontab -e
```

Add one of these lines:
```bash
# Run every day at 2 AM
0 2 * * * /home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/cmd/batch-processor/run_batch_processor.sh

# Run every 6 hours
0 */6 * * * /home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/cmd/batch-processor/run_batch_processor.sh
```

## Output

### Log Files

- **batch_processor.log**: Main application logs
- **batch_results_*.json**: Individual batch processing results

### Result File Format

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "count": 1000,
  "telco": "AirtelTigo",
  "productIds": ["8509"],
  "entryChannel": "USSD",
  "duration": "45.2s",
  "response": {
    "total": 1000,
    "successful": 950,
    "failed": 50,
    "errorDetails": {...}
  }
}
```

## Monitoring

### Prometheus/Grafana
- Prometheus scrape target added at `host.docker.internal:9101` in `ops/monitoring/prometheus/prometheus.yml`.
- Grafana dashboard provisioned at `ops/monitoring/grafana/provisioning/dashboards/dashboard-batch-processor.json`.

Metrics exposed:
- `batch_processor_batches_total{outcome="success|failure"}`
- `batch_processor_batch_duration_seconds_bucket|sum|count`
- `batch_processor_retries_total`
- `batch_processor_current_count`
- `batch_processor_paused`

### Check if the processor is running:
```bash
ps aux | grep batch-processor
```

### View real-time logs:
```bash
make logs
```

### Check processed results:
```bash
ls -la batch_results_*.json
```

## Example Scenarios

### Scenario 1: Process 10,000 subscriptions in batches of 1,000
```bash
./batch-processor -start 1000 -max 10000 -increment 1000 -wait 10s
```

### Scenario 2: Process large volume overnight
```bash
./batch-processor -start 1000 -max 1000000 -increment 5000 -wait 1m
```

### Scenario 3: Test with different product IDs
```bash
./batch-processor -products "8509,8510,8511" -start 1000 -max 5000
```

## Performance Considerations

- **Batch Size**: Larger increments process more subscriptions per call but may take longer
- **Wait Time**: Adjust based on server capacity and rate limits
- **Retries**: Configure based on network reliability
- **Concurrent Workers**: The handler uses optimized worker pools for high-volume processing

## Troubleshooting

### Service won't start
- Check if the port is already in use
- Verify the configuration file path
- Check system logs: `sudo journalctl -xe`

### High failure rate
- Increase wait time between calls
- Check server capacity
- Review error details in result files

### Memory issues
- Reduce batch size (increment value)
- Increase wait time between calls
- Monitor system resources with `htop` or `top`

## Safety Features

- **Graceful Shutdown**: Properly handles SIGINT and SIGTERM signals
- **Duplicate Prevention**: Cron script checks if already running
- **Error Logging**: Detailed error reporting for debugging
- **Dry Run Mode**: Test configuration without making actual calls

## Development

### Running Tests
```bash
go test ./...
```

### Building for Different Platforms
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o batch-processor-linux

# Windows
GOOS=windows GOARCH=amd64 go build -o batch-processor.exe

# macOS
GOOS=darwin GOARCH=amd64 go build -o batch-processor-mac
```

## License

Internal use only - Timwe Subscription System

## Support

For issues or questions, contact the development team.
