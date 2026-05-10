---
id: TMP-049
title: "Acquisition campaign slug migration startup fix"
class: vertical_defect_slice
status: in_progress
scope_limit: "Fix acquisition-api admin schema bootstrap failure when add_tenant_zz_acquisition_flow.sql drops the legacy campaigns_slug_key constraint while FK constraints still depend on campaigns(slug). Keep the change limited to acquisition-api migration safety, tests, harness evidence, and slice artifacts."
merge_policy: "Merge only after migration static tests, acquisition repository tests, HVC, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "cd services/acquisition-api && go test ./internal/repository"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slices/TMP-049-acquisition-campaign-slug-migration-startup/value-gate-report.md"
acceptance_tests:
  - "Migration drops all legacy foreign keys that depend on campaigns(slug) before dropping campaigns_slug_key."
  - "Migration does not use DROP CONSTRAINT ... CASCADE."
  - "Acquisition repository migration tests pass."
actor: platform-operator
outcome: "acquisition-api can bootstrap tenant schema on databases that still have legacy campaign slug foreign keys."
entrypoint: "services/acquisition-api/migrations/add_tenant_zz_acquisition_flow.sql"
trigger: "acquisition-api startup fails with pq cannot drop constraint campaigns_slug_key because other objects depend on it."
broken_outcome: "acquisition-api exits during tenant schema bootstrap when an existing database has any legacy foreign key that references campaigns(slug), because the migration drops campaigns_slug_key while dependent constraints still exist."
expected_behavior: "acquisition-api tenant schema bootstrap removes all legacy campaigns(slug) foreign keys explicitly before dropping campaigns_slug_key, then continues to tenant-scoped acquisition flow schema creation."
reproduction: "Start acquisition-api against a database containing the legacy campaigns_slug_key unique constraint and legacy foreign keys such as landing_versions(campaign_slug) or acquisition_transactions(campaign_slug) referencing campaigns(slug). Startup logs fail in add_tenant_zz_acquisition_flow.sql with pq cannot drop constraint campaigns_slug_key because other objects depend on it."
system_path:
  - "acquisition-api starts and connects to PostgreSQL."
  - "Tenant schema bootstrap runs acquisition migrations."
  - "Tenant acquisition flow migration removes legacy slug FKs before dropping the legacy unique slug constraint."
  - "Schema bootstrap continues to tenant-scoped acquisition indexes."
change_layers:
  - database-migration
  - backend-test
  - evidence
verification_layers:
  - migration-static-test
  - hvc
parallel_group: acquisition-runtime-schema
non_goals:
  - "Do not change runtime campaign or transaction business logic."
  - "Do not add new migrations outside acquisition-api."
  - "Do not connect to or mutate production data from tests."
file_scope:
  allowed:
    - "services/acquisition-api/migrations/add_tenant_zz_acquisition_flow.sql"
    - "services/acquisition-api/internal/repository/admin_management_schema_test.go"
    - "agent/backlog/issues/TMP-049-acquisition-campaign-slug-migration-startup.md"
    - "agent/state/TMP-049.work-order.json"
    - "agent/state/TMP-049.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-049-acquisition-campaign-slug-migration-startup/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/acquisition-api/cmd/**"
    - "services/acquisition-api/internal/service/**"
    - "services/acquisition-api/internal/handler/**"
    - "services/acquisition-api/internal/repository/*_repository.go"
    - "services/subscription-external/**"
    - "frontend/**"
    - "docker-compose*.yml"
    - "go.mod"
    - "go.sum"
---

## Operator Story

As a platform operator, I can restart acquisition-api against an existing database with legacy campaign slug foreign keys, so tenant schema bootstrap completes instead of crashing at startup.

## Acceptance Criteria

- The tenant acquisition flow migration removes legacy foreign keys that reference `campaigns(slug)` before dropping `campaigns_slug_key`.
- The migration remains explicit and does not rely on `DROP CONSTRAINT ... CASCADE`.
- `go test ./internal/repository` passes for acquisition-api.
