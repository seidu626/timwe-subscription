# TMP-045 Domain Brief: Compose Runtime Schema Bootstrap

## Actors

- Platform operator: starts the compose runtime, observes startup health, and needs deterministic schema provisioning before service startup. Source: `docker-compose.yml`.
- Runtime worker: notification-worker, cadence-engine, and postback-dispatcher poll queue tables after DB connectivity is established. Sources: `services/notification/internal/repository/outbox.go`, `services/cadence-engine/internal/repository/postgres.go`, `services/postback-dispatcher/internal/repository/postback_repository.go`.
- Acquisition API: bootstraps admin management schema on startup and requires base products/userbase tables before tenant admin migrations run. Sources: `services/acquisition-api/cmd/main.go`, `services/acquisition-api/internal/repository/admin_management_schema.go`.

## Ubiquitous Language

- Compose database bootstrap: one-shot compose service that waits for PostgreSQL, applies ordered SQL with `ON_ERROR_STOP=1`, then exits successfully. Source: `scripts/compose-db-bootstrap.sh`.
- Runtime base schema: cross-service prerequisite DDL for tables that older service-owned migrations assume already exist. Source: `ops/db/bootstrap/001_runtime_base.sql`.
- Products: base offer catalog table altered by acquisition admin tenant migrations. Sources: `ops/db/bootstrap/001_runtime_base.sql`, `services/acquisition-api/migrations/add_admin_management_tables.sql`.
- Userbase: base MSISDN segment table altered by acquisition admin tenant migrations. Sources: `ops/db/bootstrap/001_runtime_base.sql`, `services/acquisition-api/migrations/add_admin_management_tables.sql`.
- Message outbox: notification/cadence queue owned by subscription-external cadence migration. Sources: `services/subscription-external/migrations/011_message_cadence_engine.sql`, `services/subscription-external/migrations/017_tenant_notification_cadence_routing.sql`.
- Postback outbox: acquisition-owned postback delivery queue consumed by acquisition-api and postback-dispatcher. Sources: `services/acquisition-api/migrations/create_postback_tables.sql`, `services/acquisition-api/migrations/add_tenant_postback_routing.sql`.

## Domain Invariants

- Service-owned migrations remain canonical: runtime base schema supplies prerequisites only and does not duplicate `message_outbox` or `postback_outbox`. Source: `ops/db/bootstrap/001_runtime_base.sql`.
- Database consumers must start after bootstrap completion, not merely after PostgreSQL accepts connections. Source: `docker-compose.yml`.
- Postback schema ownership stays with acquisition-api; subscription-external `006_web_acquisition_campaigns.sql` remains legacy/compat material for this path. Sources: `services/acquisition-api/migrations/create_postback_tables.sql`, `slices/TMP-036-postback-outbox-schema/value-gate-report.md`.
- Worker empty-poll queries must return zero rows on an empty database rather than missing relation or missing column errors. Sources: `services/notification/internal/repository/outbox.go`, `services/cadence-engine/internal/repository/postgres.go`, `services/postback-dispatcher/internal/repository/postback_repository.go`.

## Failure Modes

- Invalid input: required PostgreSQL environment is absent, so `compose-db-bootstrap.sh` exits before attempting partial provisioning.
- Missing required: base `products` or `userbase` is absent, so acquisition admin bootstrap fails while altering those tables.
- Missing required: `message_outbox` or its tenant/channel columns are absent, so notification-worker or cadence-engine polling fails.
- Missing required: `postback_outbox` or `postback_attempts` is absent, so postback-dispatcher polling and attempt logging fail.
- Duplicate/conflict: legacy subscription-external postback DDL and acquisition-owned postback DDL both attempt to own the table; this slice avoids the duplicate by ordering only the acquisition-owned postback migration.
- Dependency failure: PostgreSQL is not ready, so bootstrap waits via `pg_isready` instead of racing worker startup.

## User Journey

1. Platform operator starts the compose database.
2. `db-bootstrap` waits for PostgreSQL readiness and validates required DB environment.
3. `db-bootstrap` applies runtime base schema, acquisition admin/tenant/postback migrations, and subscription-external cadence migrations in a deterministic order.
4. Acquisition API, notification-worker, cadence-engine, and postback-dispatcher start only after `db-bootstrap` exits successfully.
5. Workers poll empty queues and return zero rows without schema errors.

Failure journeys:

1. Platform operator omits DB credentials -> bootstrap exits with a missing environment variable error before services start.
2. Bootstrap ordering regresses -> disposable PostgreSQL proof fails with the exact SQL file and error.
3. A worker query adds a new required column without migration order update -> empty-poll query proof fails before release handoff.

## Open Questions

- Production deployment still needs an owned migration runner or release process; this slice proves local compose/runtime verification only.
- Subscription-external `006_web_acquisition_campaigns.sql` remains duplicate legacy material for postback tables and should be pruned or split in a later cleanup slice.
