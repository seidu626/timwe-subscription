---
id: TMP-048
title: "Admin tenant account mapping"
class: vertical_slice
status: done
scope_limit: "Grant explicit all-tenant admin workspace access for almauricin@gmail.com and seidu.abdulai@hotmail.com, and document the tenant/admin mapping contract. Preserve tenant isolation by accepting selected tenant headers only from platform-scoped identities."
merge_policy: "Merge only after frontend tenant workspace tests, Auth0 claim tests, acquisition admin auth tests, HVC, supervisor preflight, and value-gate evidence pass."
evidence_required:
  - "cd frontend/webspa-admin && npm test -- --watch=false --browsers=ChromeHeadless"
  - "cd common && go test ./auth/..."
  - "cd services/acquisition-api && go test ./internal/transport ./internal/handler ./internal/service"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slices/TMP-048-admin-tenant-account-mapping/value-gate-report.md"
acceptance_tests:
  - "Configured bootstrap admin emails resolve as platform-scoped webspa users with selectable tenant workspaces."
  - "Backend enriches only platform-scoped admin identities with the selected tenant header."
  - "Backend does not trust raw tenant headers for unscoped or tenant-scoped identities."
  - "Mapping contract explains how new tenant admins and platform admins are assigned."
actor: platform-operator
outcome: "The named admin users can enter the tenant workspace and operate across configured tenants without weakening tenant isolation."
entrypoint: "frontend/webspa-admin/src/app/core/services/tenant-workspace.service.ts"
trigger: "Named Auth0 users reach the tenant workspace unavailable page because their profile has no tenant assignment claims."
system_path:
  - "Auth0 authenticates an admin user."
  - "webspa-admin resolves tenant workspace claims and bootstrap admin configuration."
  - "Platform-scoped admin selects the active tenant workspace."
  - "API requests carry the selected tenant header."
  - "acquisition-api accepts selected tenant headers only from platform-scoped JWT identities."
change_layers:
  - frontend-auth
  - backend-auth
  - contract-documentation
  - evidence
verification_layers:
  - frontend-unit
  - backend-unit
  - hvc
parallel_group: admin-tenant-workspace
non_goals:
  - "Do not introduce a database admin membership subsystem in this slice."
  - "Do not create or migrate tenant rows."
  - "Do not replace Auth0 claim-based tenant assignment as the primary mapping model."
file_scope:
  allowed:
    - "frontend/webspa-admin/src/app/core/services/tenant-workspace.service.ts"
    - "frontend/webspa-admin/src/app/core/services/tenant-workspace.service.spec.ts"
    - "frontend/webspa-admin/src/environments/environment.ts"
    - "frontend/webspa-admin/src/environments/environment.prod.ts"
    - "common/auth/auth0jwt/claims.go"
    - "common/auth/auth0jwt/claims_test.go"
    - "common/auth/tenantctx/identity.go"
    - "services/acquisition-api/internal/transport/admin.go"
    - "services/acquisition-api/internal/transport/admin_test.go"
    - "services/acquisition-api/internal/handler/reports_handler.go"
    - "services/acquisition-api/internal/handler/reports_handler_test.go"
    - "services/acquisition-api/internal/repository/reports_repository.go"
    - "services/acquisition-api/internal/handler/admin_management_tenant_test.go"
    - "docs/admin-tenant-account-mapping.md"
    - "agent/backlog/issues/TMP-048-admin-tenant-account-mapping.md"
    - "agent/state/TMP-048.work-order.json"
    - "agent/state/TMP-048.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-048-admin-tenant-account-mapping/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/migrations/**"
    - "services/landing-web/**"
    - "docker-compose*.yml"
    - "Makefile"
    - "go.mod"
    - "go.sum"
---

## Operator Story

As a platform operator, I can add approved admin users to the tenant workspace without manually editing every tenant row, while keeping tenant-scoped admins constrained to their assigned tenant.

## Acceptance Criteria

- `almauricin@gmail.com` and `seidu.abdulai@hotmail.com` resolve as platform-scoped webspa-admin users.
- Platform-scoped users receive configured tenant workspace options and must select a tenant when more than one option exists.
- acquisition-api applies `X-Tenant-Key` / `X-Tenant-Id` only when the authenticated JWT identity is already platform scoped.
- Tenant-scoped or unscoped identities cannot escalate by sending tenant headers.
- The documented mapping contract explains Auth0 claim-based tenant mapping, platform admin mapping, and the configured bootstrap path for the named users.
