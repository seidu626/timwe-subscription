---
id: TMP-055
title: "Tenant nullable runtime enforcement"
class: vertical_defect_slice
status: queued
parent_vertical_slice_id: TMP-050
depends_on:
  - TMP-053
  - TMP-054
scope_limit: "Collapse active runtime nullable tenant paths into canonical tenant-aware logic and add forward-only schema cleanup only after TMP-053/TMP-054 proof exists."
merge_policy: "Merge only after HVC, affected Go tests, migration static checks, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "rg -n \"tenant_id IS NULL|idx_.*legacy|legacyProviderConfig|falling back to legacy campaign slug\" services/acquisition-api services/subscription-external services/cadence-engine -g '!**/vendor/**'"
  - "cd services/acquisition-api && go test ./internal/repository ./internal/service ./internal/handler"
  - "cd services/cadence-engine && go test ./internal/repository ./internal/adminhttp"
  - "cd services/subscription-external && go test ./internal/service ./internal/repository ./internal/handler"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Acquisition slug-only campaign lookup no longer depends on tenant_id IS NULL as the canonical path."
  - "Acquisition reports no longer join tenant-owned transactions through nullable campaign ownership."
  - "Cadence due/missing-state queries no longer accept NULL tenant matches after proof."
  - "Forward migrations clean legacy partial indexes or nullable constraints only where proof exists."
actor: platform-operator
outcome: "active runtime codepaths use tenant-aware canonical ownership instead of tenantless compatibility matching."
entrypoint: "acquisition, subscription, and cadence tenant runtime enforcement"
trigger: "TMP-052 classified active runtime nullable paths after nrg canonical backfill."
broken_outcome: "Active acquisition, reporting, subscription, and cadence runtime paths still accept tenantless ownership through tenant_id IS NULL or legacy fallback routes after canonical nrg migration."
expected_behavior: "Runtime paths use tenant-aware canonical ownership, default only through an explicit nrg policy where still required, and forward migrations remove legacy nullable lanes after proof."
reproduction: "Run rg -n \"tenant_id IS NULL|idx_.*legacy|legacyProviderConfig|falling back to legacy campaign slug\" services/acquisition-api services/subscription-external services/cadence-engine -g '!**/vendor/**' and inspect the runtime matches identified by TMP-052."
system_path:
  - "Read TMP-053 and TMP-054 proof artifacts."
  - "Update runtime tenant lookup and join predicates."
  - "Add forward-only schema cleanup for proven table groups."
  - "Verify affected service tests."
change_layers:
  - backend-runtime
  - migration
  - tests
  - evidence
verification_layers:
  - backend-unit
  - static-audit
  - hvc
parallel_group: tenant-platform-canonicalization
non_goals:
  - "Do not rewrite historical migrations."
  - "Do not mutate a remote database in the agent session."
file_scope:
  allowed:
    - "services/acquisition-api/internal/repository/campaign_repository.go"
    - "services/acquisition-api/internal/repository/reports_repository.go"
    - "services/acquisition-api/internal/service/transaction_service.go"
    - "services/acquisition-api/internal/handler/**"
    - "services/acquisition-api/internal/repository/*test.go"
    - "services/acquisition-api/internal/service/*test.go"
    - "services/acquisition-api/internal/handler/*test.go"
    - "services/acquisition-api/migrations/**"
    - "services/subscription-external/internal/service/**"
    - "services/subscription-external/internal/repository/**"
    - "services/subscription-external/internal/handler/**"
    - "services/subscription-external/migrations/**"
    - "services/cadence-engine/internal/repository/**"
    - "services/cadence-engine/internal/adminhttp/**"
    - "agent/backlog/issues/TMP-055-tenant-null-runtime-enforcement.md"
    - "agent/state/TMP-055.work-order.json"
    - "agent/state/TMP-055.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-055-tenant-null-runtime-enforcement/**"
    - "docs/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "docker-compose*.yml"
---

## Operator Story

As a platform operator, I can remove active tenantless runtime ownership paths after proof shows canonical `nrg` ownership is complete.
