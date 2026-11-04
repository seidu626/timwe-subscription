---
name: verify
description: Run notification-service locally against a throwaway Postgres with the production schema and drive TIMWE callbacks through the real routes.
---

# Verify notification-service

## Schema

The repo migrations do not rebuild the production tables. Generate DDL from the live catalog,
read-only, on the droplet (its pg_dump is 15, the server is 18, so pg_dump refuses). The query
selects columns via `format_type`, defaults via `pg_get_expr`, constraints `p/u/c` via
`pg_get_constraintdef`, non-constraint indexes via `pg_get_indexdef`, and sequences named in
`nextval` defaults. Tables the service touches: notifications, tenant_event_outbox, tenants,
tenant_channels, tenant_channel_credentials, products, subscriptions, message_outbox,
tenant_product_sms_templates and the app_* / campaign tables. Two trigger functions need stubs:
`update_subscriptions_updated_at()` and `update_updated_at_column()`.

```bash
docker run -d --name ncb-verify-pg -e POSTGRES_PASSWORD=verify -e POSTGRES_USER=sm_admin \
  -e POSTGRES_DB=subscription_manager -p 127.0.0.1:55497:5432 docker.io/library/postgres:18
psql ... -v ON_ERROR_STOP=1 -f schema.sql
```

Seed: an ACTIVE tenant, a product (`products.product_id` is varchar, e.g. '33261') owned by it, an
ACTIVE tenant_channel with `capabilities '{charge}'`, and a `tenant_channel_credentials` row with
`purpose 'tenant_events'`, `secret_ref 'env://NCB_VERIFY_EVENTS'`, `secret_fingerprint repeat('a',64)`.

## Launch

Build like the Dockerfile: `CGO_ENABLED=0 go build -mod=vendor -o /tmp/notification ./cmd/main.go`.
Run from this directory (config.yaml is read from cwd):

```bash
APPLICATION_PORT=55499 APP_DATABASE_POSTGRESQL_HOST=127.0.0.1 APP_DATABASE_POSTGRESQL_PORT=55497 \
APP_DATABASE_POSTGRESQL_USER=sm_admin APP_DATABASE_POSTGRESQL_PASSWORD=verify \
APP_DATABASE_POSTGRESQL_DB_NAME=subscription_manager APP_DATABASE_POSTGRESQL_SSL_MODE=disable \
PGMAX_OPEN_CONNS=10 APP_CACHE_REDIS_HOST=127.0.0.1 APP_CACHE_REDIS_PORT=6399 JWT_SECRET=verify \
TENANT_EVENTS_ALLOW_PRIVATE_URLS=1 NOTIFICATION_CALLBACK_TIMEOUT=2s \
NCB_VERIFY_EVENTS='{"url":"http://127.0.0.1:55498/events","signing_secret":"0123456789abcdef","default_amount":0.8,"default_currency":"GHS"}' \
/tmp/notification
```

Redis may be absent (in-memory fallback). Run any HTTP sink on 55498 to receive tenant events.

## Drive

`POST /api/v1/notification/{charge,user-renewed,user-optout}/2117` with a bare TIMWE body
(`productId`, `msisdn`, `transactionUUID`, `mnoDeliveryCode`); the tenant comes from the product owner.

- `ALTER SYSTEM SET log_statement='all'` + `pg_reload_conf()` and read `docker logs` to count
  INSERTs and owner lookups; the `DETAIL: Parameters` max `$N` / 17 is the rows per batch.
- Store stall: `BEGIN; LOCK TABLE notifications IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(4)` in a
  second session; callbacks must answer 503 `NOTIFICATION_STORE_UNAVAILABLE` at the deadline.
- Outbox stall: `LOCK TABLE tenant_event_outbox IN SHARE MODE` for over 30s; the log shows
  `tenant event claim timed out after 30s` and the dispatcher resumes after its 2 minute backoff.

## Gotchas

- Parallel curls writing bodies to stdout interleave; write each body to a temp file.
- The owner cache is per process: restart the service for a cold-cache run.
- A body the handler cannot resolve to a tenant (malformed JSON, unknown product) answers 422
  `TENANT_CONTEXT_REQUIRED` before any store work.
