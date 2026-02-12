# Postback Dispatcher

Background worker that processes postback queue and delivers conversion notifications to ad providers.

## Features

- Async postback delivery from outbox queue
- Exponential backoff retry logic
- Circuit breaker for fault tolerance
- Comprehensive attempt logging
- DLQ for failed postbacks

## Configuration

See `config.yaml` for database settings.

## Running

```bash
go run cmd/main.go
```

Or with Docker:
```bash
docker build -t postback-dispatcher .
docker run postback-dispatcher
```

## Monitoring

Check `postback_outbox` and `postback_attempts` tables for delivery status.
