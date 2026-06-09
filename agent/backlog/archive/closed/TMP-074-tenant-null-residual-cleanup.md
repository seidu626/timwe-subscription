---
id: TMP-074
title: "Tenant nullable residual cleanup"
class: vertical_defect_slice
status: queued
parent_vertical_slice_id: TMP-050
depends_on:
  - TMP-055
scope_limit: "Resolve the remaining tenantless admin activity and notification rows, remove exposed slug-only public campaign compatibility, and add forward-only cleanup for proven nullable lanes."
merge_policy: "Merge only after live row proof, affected Go tests, HVC, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "Live SQL proof for admin_activity_logs, notifications, campaigns, and legacy partial indexes."
  - "cd services/acquisition-api && go test ./internal/repository ./internal/service ./internal/handler"
  - "cd services/subscription-external && go test ./internal/repository ./internal/service"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "admin_activity_logs has zero tenantless live rows and tenant-create audit rows bind to the created tenant, not blanket nrg."
  - "notifications has zero tenantless live rows and notification tenant nullable cleanup is represented by a forward migration."
  - "Public campaign GET /v1/campaigns/{slug} and GET /v1/campaigns no longer expose tenant_id IS NULL compatibility."
  - "Existing tenant-aware public campaign GET /v1/campaigns/{tenant_key}/{slug} remains supported."
actor: platform-operator
outcome: "documented residual tenant-null gaps are converted into tenant-owned rows and fail-closed runtime behavior."
entrypoint: "tenant residual cleanup and public campaign route enforcement"
trigger: "TMP-055 shipped with documented non-blocking residual gaps after canonical tenant runtime enforcement."
broken_outcome: "Residual tenantless rows and public tenantless campaign routes keep nullable ownership compatibility alive after TMP-055."
expected_behavior: "Residual rows are tenant-owned, future tenant-create audit logs carry tenant ownership, and public campaign access requires explicit tenant context."
reproduction: "Check live tenantless counts for admin_activity_logs, notifications, and campaigns; request /v1/campaigns/{slug} without tenant context."
system_path:
  - "Refresh live proof for residual rows and legacy indexes."
  - "Backfill verified residual rows with table-specific semantics."
  - "Patch runtime and migration scripts to prevent recurrence."
  - "Remove public slug-only campaign compatibility."
  - "Verify affected services and harness classification."
change_layers:
  - data-migration
  - backend-runtime
  - migration
  - tests
  - evidence
verification_layers:
  - live-sql-proof
  - backend-unit
  - hvc
parallel_group: tenant-platform-canonicalization
non_goals:
  - "Do not rewrite historical migrations."
  - "Do not change unrelated tenant onboarding or admin UI behavior."
file_scope:
  allowed:
    - "scripts/db-migrate-tenant-platform.sh"
    - "services/acquisition-api/internal/handler/campaign_handler.go"
    - "services/acquisition-api/internal/handler/campaign_handler_test.go"
    - "services/acquisition-api/internal/service/campaign_service.go"
    - "services/acquisition-api/internal/service/campaign_service_test.go"
    - "services/acquisition-api/internal/repository/campaign_repository.go"
    - "services/acquisition-api/internal/repository/admin_management_repository.go"
    - "services/acquisition-api/internal/repository/admin_management_repository_test.go"
    - "services/acquisition-api/migrations/**"
    - "services/subscription-external/migrations/**"
    - "docs/**"
    - "agent/backlog/issues/TMP-074-tenant-null-residual-cleanup.md"
    - "agent/state/TMP-074.work-order.json"
    - "agent/state/TMP-074.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-074-tenant-null-residual-cleanup/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "docker-compose*.yml"
---

## Operator Story

As a platform operator, I can close the remaining tenant-null residuals without assigning unrelated tenant-create audit evidence to `nrg`.
