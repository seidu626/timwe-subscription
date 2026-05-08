# TMP-020 Domain Brief: Tenant Observability Baseline

## Actors

- operations-analyst: inspects health, logs, and metrics to diagnose tenant/channel issues without seeing PII (source: `slices/TMP-020-tenant-observability-baseline/slice.yaml`).
- tenant-admin: indirectly benefits when tenant-specific messaging and cadence failures can be diagnosed without cross-tenant confusion (source: `slices/TMP-008-notification-and-cadence-routing/domain-brief.md`).
- notification worker: dispatches tenant/channel-owned outbox jobs and records bounded outcome metrics/logs (source: `services/notification/internal/dispatcher/dispatcher.go`).
- cadence admin service: exposes protected admin HTTP routes that resolve tenant context and should log safe request context (source: `services/cadence-engine/internal/adminhttp/server.go`).

## Ubiquitous Language

- Tenant label: bounded operational label derived from resolved tenant context or job ownership, never from untrusted guessed request data (source: `services/cadence-engine/internal/observability/http.go`, `services/notification/internal/observability/tenant.go`).
- Channel label: bounded channel identifier used alongside tenant to diagnose routing and messaging issues (source: `services/notification/internal/dispatcher/metrics.go`).
- Safe label allowlist: approved low-cardinality labels such as `tenant_id`, `channel_id`, `worker`, and `status` (source: `services/notification/internal/observability/tenant.go`).
- PII/secret blocklist: forbidden observability keys such as `msisdn`, `click_id`, `authorization`, `token`, `secret`, `headers`, and request bodies (source: `services/notification/internal/observability/tenant.go`).
- Health observability status: lightweight health response field declaring tenant labels are enabled and PII labels are rejected (source: `services/notification/internal/transport/router.go`, `services/cadence-engine/internal/adminhttp/server.go`).

## Domain Invariants

- Operational labels must be tenant/channel-aware without including PII, secrets, rendered callback URLs, request bodies, or raw headers (source: `slices/TMP-020-tenant-observability-baseline/slice.yaml`).
- Unknown tenant denials log `unknown` tenant/trust source rather than guessing from raw headers or query parameters (source: `services/cadence-engine/internal/observability/http.go`).
- Worker metrics use bounded labels only: tenant, channel, worker, and status (source: `services/notification/internal/dispatcher/metrics.go`).
- Notification request logging must not dump full fasthttp request strings because those can include MSISDN, headers, and bodies (source: `services/notification/internal/handler/http.go`, `services/notification/internal/transport/router.go`).
- Health endpoints expose observability readiness without leaking tenant-specific values (source: `services/notification/internal/transport/router_test.go`, `services/cadence-engine/internal/adminhttp/server_test.go`).

## Failure Modes

- HTTP request log includes raw query/body/header data: rejected by replacing full-request logs with safe structured method/path/type fields and by HTTP middleware tests.
- PII metric label attempted: rejected by the label guard test.
- Worker metric emits unbounded labels: constrained to tenant/channel/worker/status.
- Protected flow fails before tenant resolution: logs `unknown` tenant and trust source with request id only.
- Health endpoint omits observability status: covered by health endpoint tests.

## User Journey

1. Operations analyst checks notification or cadence health and sees observability status declaring tenant labels enabled and PII labels rejected.
2. Cadence admin HTTP request runs through safe logging middleware, recording tenant/channel/request id when resolved, or `unknown` when denied before tenant resolution.
3. Notification worker processes a tenant/channel outbox job and increments `notification_worker_dispatch_total` with safe tenant/channel/worker/status labels.
4. Notification worker logs job outcome with job id, tenant/channel, worker, attempt, and error status, but not MSISDN or message body.

Failure journeys:
1. Developer attempts to use `msisdn` or `click_id` as a metric label -> label guard returns an error.
2. Unknown notification route includes `msisdn` in query -> response is 404 and logging avoids full request dumps.
3. Request denied before tenant resolution -> log contains `unknown` tenant/trust source and no guessed tenant.

## Open Questions

- Acquisition and subscription-external still contain older high-cardinality/PII-prone logs. This slice establishes the canonical guard and fixes the touched notification/cadence paths; broader cleanup should be handled by TMP-015 or a dedicated privacy/observability hardening slice.
- Notification worker metrics are registered but the worker process does not expose a scrape endpoint yet. Adding a worker metrics HTTP server would increase runtime surface and is better sized as a follow-up if ops needs direct worker scraping.
