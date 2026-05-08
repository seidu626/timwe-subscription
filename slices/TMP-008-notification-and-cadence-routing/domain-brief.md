# TMP-008 Domain Brief: Notification and Cadence Routing

## Actors

- tenant-admin: lists notifications and manages cadence series/content for the tenant workspace (source: `slices/TMP-008-notification-and-cadence-routing/slice.yaml`, `services/notification/internal/handler/http.go`, `services/cadence-engine/internal/adminhttp/server.go`).
- notification worker: claims message outbox jobs and dispatches MT payloads while preserving tenant/channel headers and retry state (source: `services/notification/internal/dispatcher/dispatcher.go`, `services/notification/internal/repository/outbox.go`).
- cadence engine: claims due subscription message state, selects series/content, and writes message outbox jobs (source: `services/cadence-engine/internal/planner/planner.go`, `services/cadence-engine/internal/repository/postgres.go`).
- platform-operator: owns migrations and legacy/global-row compatibility until TMP-011 backfills default tenant data (source: `services/subscription-external/migrations/011_message_cadence_engine.sql`, `services/subscription-external/migrations/017_tenant_notification_cadence_routing.sql`).

## Ubiquitous Language

- Tenant context: trusted tenant identity from auth context; platform-scoped identities may choose an explicit tenant selection header/query for admin reads and writes (source: `common/auth/tenantctx/identity.go`, `services/cadence-engine/internal/adminhttp/server.go`, `services/notification/internal/transport/router.go`).
- Channel context: optional tenant channel identifier carried as `channel_id`/`channelId` or `X-Tenant-Channel-Id` to filter routing and worker dispatch (source: `services/notification/internal/handler/http.go`, `services/cadence-engine/internal/adminhttp/server.go`).
- Notification list: read model over `notifications`, filterable by date, partner role, MSISDN, entry channel, notification type, tenant, and channel (source: `services/notification/internal/service/notification.go`, `services/notification/internal/repository/postgres.go`).
- Message series: cadence definition keyed by tenant, partner role, product, and name with optional channel ownership (source: `services/cadence-engine/internal/domain/types.go`, `services/cadence-engine/internal/repository/postgres.go`).
- Subscription message state: per-subscription/per-series cursor and due-time state; only `ACTIVE` rows can be claimed for planning (source: `services/cadence-engine/internal/repository/postgres.go`).
- Message outbox: planned cadence sends with existing global `idempotency_key` uniqueness and tenant/channel ownership for dispatch/retry visibility; tenant/channel are encoded into planned keys rather than enforced by a second partial uniqueness rule (source: `services/subscription-external/migrations/011_message_cadence_engine.sql`, `services/cadence-engine/internal/planner/planner.go`, `services/subscription-external/migrations/017_tenant_notification_cadence_routing.sql`).

## Domain Invariants

- Tenant admin notification reads require verified tenant identity and must not accept raw caller-supplied tenant headers as tenant-admin authority (source: `slices/TMP-008-notification-and-cadence-routing/slice.yaml`, `services/notification/internal/transport/router.go`, `services/notification/internal/handler/http.go`).
- Cadence admin list/import/read/edit operations require tenant scope and must not resolve another tenant's series by id or key (source: `services/cadence-engine/internal/adminhttp/server.go`).
- Series uniqueness is tenant-scoped for tenant rows while legacy global rows stay unique until migration isolation is complete (source: `services/subscription-external/migrations/017_tenant_notification_cadence_routing.sql`).
- Planner idempotency keys include tenant and channel parts and are still protected by the existing global `message_outbox.idempotency_key` uniqueness (source: `services/subscription-external/migrations/011_message_cadence_engine.sql`, `services/cadence-engine/internal/planner/planner.go`).
- Paused or inactive `subscription_message_state` rows must not create jobs (source: `services/cadence-engine/internal/repository/postgres.go`).
- Worker failures stay attached to the claimed outbox row, including tenant/channel context for retry and failure inspection (source: `services/notification/internal/repository/outbox.go`, `services/notification/internal/dispatcher/dispatcher.go`).

## Failure Modes

- Notification list:
  - Missing required: absent verified tenant context returns 403.
  - Authorization: tenant/channel filters are injected before repository access; cross-tenant rows are excluded.
  - Header spoofing: raw tenant header/query without verified identity is rejected for admin reads.
  - Invalid input: existing date/page parsing errors remain handled by the service layer.
  - Cache leakage: cache key includes tenant/channel filters.
- Cadence series list/import:
  - Missing required: absent tenant context returns 403.
  - Authorization: series lookup by id/key is tenant-scoped; channel mismatch returns 404.
  - Duplicate/conflict: same tenant/partner/product/name upserts the tenant row; old global uniqueness is replaced by a legacy-only partial index.
  - Invalid input: malformed CSV rows remain rejected by parser validation.
- Cadence planning:
  - Duplicate/conflict: duplicate idempotency key returns false and does not plan a second job.
  - Concurrent access: due-state claim uses `FOR UPDATE SKIP LOCKED`.
  - Authorization/data integrity: due-state claim requires tenant/channel compatibility among state, subscription, and series.
  - Paused state: non-`ACTIVE` state is not claimed.
- Notification worker dispatch:
  - Dependency failure: failed MT send marks retry or failed status on the same outbox job.
  - Authorization: tenant/channel headers are propagated with dispatch payloads.

## User Journey

1. Tenant admin calls `GET /api/v1/notification/list` with a valid admin bearer token and optional channel filter.
2. Notification router validates the token, stores tenant identity in the fasthttp context, and the handler passes tenant/channel into service/repository filters.
3. Tenant admin calls `GET /v1/admin/cadence/series` or imports CSV content with tenant context.
4. Cadence admin handler resolves tenant/channel scope, list/import queries persist and filter rows by tenant/channel, and cross-tenant series ids return not found.
5. Cadence planner claims only active, tenant-compatible state and writes outbox jobs with tenant/channel-aware idempotency keys.
6. Notification worker claims jobs, sends MT with tenant/channel context, and records retry/failure against the claimed row.

Failure journeys:
1. Tenant admin omits verified tenant context on notification or cadence admin route -> 403 and no unscoped repository query.
2. Caller supplies only `X-Tenant-Id` to notification list without verified identity -> 403 and no repository query.
3. Tenant admin supplies another tenant's series id -> 404 and no content/rule mutation.
4. Planner sees paused state or duplicate idempotency key -> no duplicate message job.
5. Worker send fails -> outbox row remains tenant/channel attributed and is retried or marked failed.

## Open Questions

- The notification service now validates the admin list route with Auth0 claims and stores `tenantctx.Identity` in fasthttp user values. Full shared middleware convergence and observability labels remain owned by auth/observability slices.
- `go test -mod=readonly` fails in `services/notification` because the checked-in module metadata wants dependency updates unrelated to this slice. Validation used `go test -mod=mod ./...` and excluded the generated dependency churn from the diff.
