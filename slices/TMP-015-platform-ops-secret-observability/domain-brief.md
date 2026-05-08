# TMP-015 Domain Brief: Platform Ops Secret Observability

Post-hoc reconciliation note: this domain brief was added after implementation to align the shipped slice artifacts with the domain-grounding contract. It summarizes existing `slice.yaml`, issue, ops, observability, and value-gate evidence; it does not introduce new runtime scope.

## Actors

- Platform operator: reviews deploy configuration, secret posture, health, metrics, and production checklist for tenant channels. Source: `slices/TMP-015-platform-ops-secret-observability/slice.yaml`.
- Operations analyst: uses dashboards and metrics to diagnose tenant/channel health without seeing secrets or PII. Source: `ops/monitoring/production-checklist.md`.
- Notification worker: emits tenant/channel dispatch metrics and must avoid unsafe labels. Source: `services/notification/internal/dispatcher/metrics.go`.
- Subscription external service: resolves provider credentials by tenant/channel and fails closed when credentials are missing. Source: `services/subscription-external/internal/service/tenant_routing.go`.

## Ubiquitous Language

- Credential reference: a non-secret identifier that points to provider credentials without storing raw secret material in business data. Source: `services/subscription-external/internal/service/tenant_routing.go`.
- Secret backend unavailable: failure mode where credential resolution cannot retrieve a provider credential and outbound work must fail closed. Source: `services/subscription-external/internal/service/tenant_routing_test.go`.
- Tenant/channel label: bounded metric label used for operational diagnosis without PII or high-cardinality values. Source: `services/notification/internal/observability/tenant.go`.
- PII/secret blocklist: observability keys such as `msisdn`, `authorization`, `token`, `secret`, and raw headers/bodies that must not become labels/log values. Source: `services/notification/internal/observability/tenant.go`.
- Production checklist: operator evidence for secret hygiene, metrics, dashboards, gateway trust, and tenant readiness. Source: `ops/monitoring/production-checklist.md`.

## Domain Invariants

- Raw credentials must not be persisted as durable business data or checked into docs/config examples.
- Missing or unresolved credential references fail closed and surface degraded/failed health rather than falling back to global credentials.
- Tenant/channel metrics must use bounded, approved labels only.
- Observability must not include MSISDN, click IDs, authorization headers, tokens, raw secrets, or request bodies.
- Local development samples must remain obviously non-production and must not require real tenant credentials for unrelated tests.

## Failure Modes

- Missing provider credential: tenant provider resolution returns an error before upstream operation.
- Credential accidentally logged: logs/tests must expose reference or failure reason only, never secret material.
- Unsafe metric label: observability guard rejects PII or secret-shaped keys.
- Missing tenant/channel label on worker path: dispatch metrics tests catch absent scope labels.
- Monitoring password unset: compose/runbook must require explicit environment-backed credentials.

## User Journey

1. Platform operator reviews environment and compose documentation before tenant-channel deploy.
2. Operator starts monitoring stack with environment-backed credentials.
3. Notification worker processes tenant/channel dispatch and records safe bounded metrics.
4. Operator inspects dashboard and production checklist to confirm tenant/channel health posture.
5. When credential backend cannot resolve a provider credential, outbound operations fail closed and evidence remains safe.

Failure journeys:

1. Developer attempts to label metrics with `msisdn` or `secret` -> label validation fails.
2. Secret backend is unavailable -> provider operation does not use fallback raw/global credentials.
3. Monitoring config lacks required password -> startup/config review blocks unsafe default.

## Open Questions

- Cloud-specific secret manager selection remains an operator decision; this slice documents and tests the adapter posture without committing to a provider.
- Broader PII log cleanup outside touched notification/subscription paths should remain a separate privacy/observability hardening slice if needed.
