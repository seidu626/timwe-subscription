# TMP-045 Spec: Compose Runtime Schema Bootstrap

## Story

As a platform operator, I want the compose runtime to apply required database schema before acquisition-api, notification-worker, cadence-engine, and postback-dispatcher start, so release verification does not fail on empty-database missing relation errors.

## Scope

In scope:
- Add a compose-owned one-shot `db-bootstrap` service.
- Add minimal runtime base SQL for cross-service prerequisite tables.
- Keep acquisition-api migrations canonical for postback tables.
- Keep subscription-external cadence migrations canonical for message outbox tables.
- Verify against a clean disposable PostgreSQL database.

Out of scope:
- Service-local self-migration code.
- Duplicate message_outbox or postback_outbox definitions.
- Production deployment migration ownership.
- Webspa-admin submodule or local main branch integration.

## Acceptance Criteria

1. `docker-compose.yml` renders and runtime database consumers depend on `db-bootstrap` completion.
2. `scripts/compose-db-bootstrap.sh` validates required DB environment and applies SQL with `ON_ERROR_STOP=1`.
3. Clean PostgreSQL bootstrap creates `products`, `userbase`, `message_outbox`, `postback_outbox`, `postback_attempts`, `tenant_channels`, and `acquisition_transactions`.
4. Notification-worker, cadence-engine, and postback-dispatcher empty-poll query shapes execute against the bootstrapped database and return zero rows.
5. Targeted Go tests pass for acquisition repository bootstrap, notification package, and postback-dispatcher package.

## Architecture Notes

This is a deepening change at the compose runtime seam. The interface is small: dependent services require one completed module, `db-bootstrap`, rather than each service carrying ad hoc startup ordering logic. The implementation keeps locality in one bootstrap script and one base SQL file, while preserving leverage from existing service-owned migrations.

Prune decision: do not run `services/subscription-external/migrations/006_web_acquisition_campaigns.sql` for postback provisioning. It duplicates the acquisition-owned postback tables with a different JSONB/TEXT shape. The canonical path is acquisition base prerequisites, acquisition postback migrations, then tenant postback routing.
