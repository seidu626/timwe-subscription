---
id: TMP-051
title: "Tenant catalog admin UI and API"
class: vertical_slice
status: queued
scope_limit: "Add tenant catalog list/update admin features through acquisition-api and webspa-admin. Keep tenant catalog operations operator-scoped; tenant-scoped admins must not list or mutate other tenants."
merge_policy: "Merge only after HVC, backend handler/service/repository tests, frontend tests/build for touched tenant catalog UI, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "cd services/acquisition-api && go test ./internal/handler ./internal/service ./internal/repository ./internal/transport"
  - "cd frontend/webspa-admin && npm test -- --watch=false --browsers=ChromeHeadless --progress=false"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Operator-scoped admin can list tenants with status/default country metadata."
  - "Operator-scoped admin can update tenant status/name/default country/metadata."
  - "Tenant-scoped admin cannot list or mutate tenant catalog."
  - "webspa-admin exposes tenant catalog navigation and tenant catalog states without bypassing tenant workspace guards."
actor: operator
outcome: "operators can list and update tenant catalog records from admin API and UI."
entrypoint: "GET /v1/admin/tenants and webspa-admin tenant catalog route"
trigger: "operator requested admin features for tenant catalog records after making nrg canonical."
system_path:
  - "Operator admin authenticates."
  - "Backend authorizes operator scope."
  - "Tenant catalog list/update uses tenant repository methods."
  - "Frontend route displays tenant records and update actions."
change_layers:
  - backend-api
  - frontend-admin
  - tests
  - evidence
verification_layers:
  - backend-unit
  - frontend-unit
parallel_group: tenant-admin-catalog
non_goals:
  - "Do not add database membership tables in this slice."
  - "Do not change tenant migration tooling in this slice."
file_scope:
  allowed:
    - "services/acquisition-api/internal/domain/admin_management.go"
    - "services/acquisition-api/internal/repository/admin_management_repository.go"
    - "services/acquisition-api/internal/repository/admin_management_schema_test.go"
    - "services/acquisition-api/internal/service/admin_management_service.go"
    - "services/acquisition-api/internal/service/admin_management_service_test.go"
    - "services/acquisition-api/internal/handler/admin_management_handler.go"
    - "services/acquisition-api/internal/handler/admin_management_tenant_test.go"
    - "services/acquisition-api/internal/transport/router.go"
    - "services/acquisition-api/internal/transport/admin_test.go"
    - "frontend/webspa-admin/src/app/**"
    - "agent/backlog/issues/TMP-051-tenant-catalog-admin-ui-api.md"
    - "agent/state/TMP-051.work-order.json"
    - "agent/state/TMP-051.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-051-tenant-catalog-admin-ui-api/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "scripts/db-migrate-tenant-platform.sh"
    - "services/subscription-external/**"
    - "docker-compose*.yml"
    - "go.mod"
    - "go.sum"
---

## Operator Story

As an operator, I can view and update tenant catalog records in the admin workspace so tenant lifecycle operations are visible.
