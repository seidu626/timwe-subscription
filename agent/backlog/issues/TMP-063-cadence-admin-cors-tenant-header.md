---
id: TMP-063
title: Cadence admin CORS tenant header
class: vertical_defect_slice
status: queued
scope_limit: "Fix only the cadence admin HTTP CORS preflight response for tenant-scoped admin UI requests; do not change frontend interceptors, auth behavior, service business logic, database schema, migrations, dependency manifests, or deploy configuration."
merge_policy: "Merge only after HVC, cadence admin HTTP tests, targeted preflight evidence, and supervisor preflight pass."
evidence_required:
  - "go test ./internal/adminhttp"
  - "OPTIONS preflight includes X-Tenant-Key"
  - "hvc check agent/backlog/issues/TMP-063-cadence-admin-cors-tenant-header.md --fail-on block"
acceptance_tests:
  - "hvc check agent/backlog/issues/TMP-063-cadence-admin-cors-tenant-header.md --fail-on block"
  - "cd services/cadence-engine && go test ./internal/adminhttp"
actor: admin user
outcome: "The Angular admin cadence page can preflight tenant-scoped requests to the local cadence admin API without browser CORS rejection."
entrypoint: "GET http://localhost:8091/v1/admin/cadence/series?limit=500 from http://localhost:4200"
trigger: "Admin navigates to the cadence page in the local admin UI."
broken_outcome: "Browser preflight fails because X-Tenant-Key is absent from Access-Control-Allow-Headers."
expected_behavior: "Cadence admin CORS preflight allows the tenant headers emitted by the admin UI tenant workspace interceptor."
desired_outcome: "The cadence admin API responds to OPTIONS preflight with Access-Control-Allow-Headers containing X-Tenant-Key, X-Tenant-Id, and channel headers needed by tenant-scoped cadence requests."
reproduction:
  command: "OPTIONS /v1/admin/cadence/series with Origin: http://localhost:4200 and Access-Control-Request-Headers: x-admin-token,x-tenant-key"
  observed: "Access-Control-Allow-Headers omits X-Tenant-Key, so the browser blocks the follow-up GET."
  expected: "Access-Control-Allow-Headers includes X-Tenant-Key while preserving existing allowed auth headers."
system_path:
  - "frontend/webspa-admin tenant workspace interceptor adds X-Tenant-Key."
  - "frontend/webspa-admin cadence API service calls cadence-engine /v1/admin/cadence endpoints."
  - "services/cadence-engine/internal/adminhttp/access.go handles CORS preflight."
change_layers:
  - cadence-engine
  - admin-http
verification_layers:
  - unit-test
  - cors-preflight
blocked_by: []
blocks: []
parallel_group: admin-cors
file_scope:
  allowed:
    - "services/cadence-engine/internal/adminhttp/access.go"
    - "services/cadence-engine/internal/adminhttp/access_test.go"
    - "agent/backlog/issues/TMP-063-cadence-admin-cors-tenant-header.md"
    - "agent/state/TMP-063.work-order.json"
    - "agent/state/TMP-063.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/webspa-admin/**"
    - "services/cadence-engine/go.mod"
    - "services/cadence-engine/go.sum"
    - "services/cadence-engine/internal/repository/**"
    - "services/cadence-engine/internal/scheduler/**"
    - "services/cadence-engine/internal/planner/**"
    - "services/cadence-engine/migrations/**"
    - "docker-compose*.yml"
---

## Reproduction

The browser reports:

`Request header field x-tenant-key is not allowed by Access-Control-Allow-Headers in preflight response.`

The failing request is:

`GET http://localhost:8091/v1/admin/cadence/series?limit=500`

from:

`http://localhost:4200/#/cadence`

## Acceptance Criteria

- Cadence admin CORS allows `X-Tenant-Key` for admin UI tenant workspace requests.
- Cadence admin CORS also allows `X-Tenant-Id`, `X-Tenant-Channel-Id`, and `X-Channel-Id` because the cadence admin handler consumes tenant/channel scope from those headers.
- Existing `Content-Type`, `Authorization`, and `X-Admin-Token` CORS support remains intact.
- Unit tests cover the preflight response for local admin origin and tenant headers.
