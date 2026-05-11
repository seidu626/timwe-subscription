# TMP-056 Domain Brief: Acquisition Postback Tenant Routing Bootstrap

## Actors

- Platform operator: starts acquisition-api and inspects service logs during release/runtime verification. Source: `agent/backlog/issues/TMP-056-acquisition-postback-tenant-routing-bootstrap.md`.
- Acquisition API runtime: bootstraps database schema before serving requests and running the in-process postback dispatcher. Source: `services/acquisition-api/cmd/main.go`.
- Postback dispatcher: polls `postback_outbox` through `PostbackRepository.ClaimPendingPostbacks`. Source: `services/acquisition-api/internal/worker/postback_dispatcher.go`.

## Ubiquitous Language

- Admin management schema bootstrap: the startup migration sequence invoked by `AdminManagementRepository.EnsureSchema`. Source: `services/acquisition-api/internal/repository/admin_management_schema.go`.
- Postback outbox: acquisition-owned queue table used for async postback delivery. Source: `services/acquisition-api/migrations/create_postback_tables.sql`.
- Tenant postback routing: `tenant_id`, `channel_id`, and `failure_reason` columns plus tenant indexes on `postback_outbox`. Source: `services/acquisition-api/migrations/add_tenant_postback_routing.sql`.
- Single canonical postback path: acquisition-api owns postback table shape; subscription-external duplicate DDL is not part of runtime bootstrap. Source: `slices/TMP-045-compose-runtime-schema-bootstrap/spec.md`.

## Domain Invariants

- The dispatcher must not start polling a schema that lacks columns selected by `PostbackRepository`. Source: `services/acquisition-api/internal/repository/postback_repository.go`.
- Acquisition-owned postback migrations remain canonical; duplicate subscription-external postback DDL is not added to this path. Source: `slices/TMP-045-compose-runtime-schema-bootstrap/spec.md`.
- Startup bootstrap should be idempotent because migration files use `IF NOT EXISTS` for additive schema changes. Source: `services/acquisition-api/migrations/add_tenant_postback_routing.sql`.

## Failure Modes

- Missing required: `postback_outbox.tenant_id` is absent, so `ClaimPendingPostbacks` fails with `pq: column "tenant_id" does not exist`.
- Ordering failure: admin schema bootstrap completes, but tenant postback routing is not applied before dispatcher polling begins.
- Duplicate/conflict: a fix adds subscription-external postback DDL instead of acquisition-owned tenant routing, reintroducing multiple postback table definitions.
- Dependency failure: the database is unavailable, causing startup to fail before schema bootstrap; this is outside the missing-column defect.

## User Journey

1. Platform operator starts acquisition-api.
2. Acquisition API connects to PostgreSQL and runs startup schema bootstrap.
3. Bootstrap applies tenant/admin schema and the acquisition-owned postback tenant-routing migration.
4. Postback dispatcher starts and `ClaimPendingPostbacks` can select `tenant_id`, `channel_id`, and `failure_reason` from `postback_outbox`.

## Failure Journeys

1. Bootstrap omits tenant postback routing, then dispatcher polling returns a missing-column error in the service log.
2. Bootstrap uses duplicate subscription-external postback DDL, then canonical acquisition table ownership is split across services.

## Open Questions

- Live database mutation proof is intentionally not run in this agent session because prior tenant proof slices recorded missing documented DB credentials.
