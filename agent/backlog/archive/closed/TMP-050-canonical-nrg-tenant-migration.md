---
id: TMP-050
title: "Canonical nrg tenant migration"
class: vertical_defect_slice
status: in_progress
scope_limit: "Replace the legacy-default tenant migration and bootstrap defaults with one canonical nrg tenant path. Existing rows with missing tenant_id must migrate to nrg, including channel-scoped rows. Remove rollback-to-null and legacy-default compatibility wording from active commands/docs/config touched by this slice."
merge_policy: "Merge only after shell syntax checks, static canonical-path checks, affected Go/Angular tests, HVC, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "bash -n scripts/db-migrate-tenant-platform.sh"
  - "! rg -n \"legacy-default|LEGACY_TENANT|db-rollback-tenant-platform|--rollback|tenant_id = NULL|channel_id IS NULL|channel_id IS NOT NULL\" scripts/db-migrate-tenant-platform.sh docs/tenant-platform-migration-runbook.md docs/tenant-channel-onboarding.md frontend/webspa-admin/src/environments services/acquisition-api/internal/transport/admin_test.go services/acquisition-api/internal/handler/reports_handler_test.go"
  - "cd services/acquisition-api && go test ./internal/transport ./internal/handler"
  - "cd frontend/webspa-admin && npm test -- --watch=false --browsers=ChromeHeadless --progress=false"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Migration script creates or resolves tenant_key=nrg and backfills every tenant_id IS NULL row in the configured tables."
  - "Migration script has no rollback mode that restores tenant_id to NULL."
  - "Active frontend bootstrap defaults and backend tests use nrg instead of legacy-default."
actor: platform-operator
outcome: "existing tenantless production data is assigned to the canonical nrg tenant with no legacy-default compatibility path."
entrypoint: "make db-migrate-tenant-platform"
trigger: "operator has decided nrg is the canonical default tenant and legacy/default compatibility paths must be pruned."
broken_outcome: "existing migration tooling creates legacy-default, skips tenantless rows that already have channel_id, and exposes rollback-to-null behavior that preserves a legacy global data path."
expected_behavior: "migration tooling creates nrg, assigns every tenantless row in configured tables to nrg, and does not provide an active rollback-to-null legacy path."
reproduction: "Run bash scripts/db-migrate-tenant-platform.sh --help or inspect scripts/db-migrate-tenant-platform.sh on main; it documents LEGACY_TENANT_KEY default legacy-default and supports --rollback."
system_path:
  - "Platform operator runs dry-run for tenant migration."
  - "Script reports nrg tenant readiness and tenantless row counts."
  - "Platform operator runs apply."
  - "Script upserts nrg and backfills every configured tenantless row to nrg."
  - "Admin bootstrap defaults reference nrg."
change_layers:
  - migration-script
  - runbook
  - admin-frontend-config
  - backend-test
  - evidence
verification_layers:
  - shell-syntax
  - static-prune-check
  - backend-test
  - frontend-test
parallel_group: tenant-platform-canonicalization
non_goals:
  - "Do not run the migration against a live remote database in this agent session."
  - "Do not add multi-tenant membership tables in this slice."
  - "Do not implement tenant list/update UI in this slice; that follows after the canonical tenant migration path is safe."
file_scope:
  allowed:
    - "scripts/db-migrate-tenant-platform.sh"
    - "Makefile"
    - "docs/tenant-platform-migration-runbook.md"
    - "docs/tenant-channel-onboarding.md"
    - "frontend/webspa-admin/src/environments/environment.ts"
    - "frontend/webspa-admin/src/environments/environment.prod.ts"
    - "frontend/webspa-admin/src/app/core/services/tenant-workspace.service.spec.ts"
    - "services/acquisition-api/internal/transport/admin_test.go"
    - "services/acquisition-api/internal/handler/reports_handler_test.go"
    - "agent/backlog/issues/TMP-050-canonical-nrg-tenant-migration.md"
    - "agent/state/TMP-050.work-order.json"
    - "agent/state/TMP-050.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-050-canonical-nrg-tenant-migration/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/acquisition-api/internal/service/**"
    - "services/acquisition-api/internal/repository/**"
    - "services/subscription-external/internal/**"
    - "services/*/go.mod"
    - "services/*/go.sum"
    - "docker-compose*.yml"
---

## Operator Story

As a platform operator, I can migrate existing global data into the `nrg` tenant so production data has one canonical tenant owner and no legacy nullable tenant path remains in active migration tooling.

## Acceptance Criteria

- `scripts/db-migrate-tenant-platform.sh` defaults to `tenant_key=nrg`.
- `--apply` backfills configured tables where `tenant_id IS NULL` without excluding rows that already have `channel_id`.
- The active script and Make targets do not offer rollback-to-null.
- Web admin bootstrap defaults and backend tests use `nrg` instead of `legacy-default`.
