---
id: TMP-011
title: "Legacy data migration isolation"
class: operational_slice
status: ready
parent_vertical_slice_id: TMP-011
scope_limit: "Add a safe operational migration path for legacy global rows: dry-run verification, default tenant backfill to tenant_key=legacy-default, idempotent batch execution, rollback posture, and value-gate evidence. Do not implement unrelated UI, partner contract, or secret hardening work."
merge_policy: "Merge only after HVC, supervisor preflight, migration verification evidence, value-gate report, and slice-harness status pass."
evidence_required:
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "agent-supervisor --config .harness/config.json preflight"
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "make db-migrate-tenant-platform-dry-run or equivalent SQL dry-run verification"
  - "TMP-011 value-gate report"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "make db-migrate-tenant-platform-dry-run"
non_goals:
  - "No admin portal tenant workspace changes."
  - "No partner onboarding contract breadth."
  - "No production database execution from this agent session."
actor: platform-operator
outcome: "Existing production data can be backfilled and verified under tenant isolation before multi-tenant enforcement."
entrypoint: "make db-migrate-tenant-platform"
trigger: "Platform operator runs the tenant platform migration workflow"
system_path:
  - "Dry-run verification checks tenant existence, row counts, unmapped rows, conflict counts, and constraint readiness."
  - "Migration creates or resolves tenant_key=legacy-default and backfills eligible legacy rows in idempotent batches."
  - "Rollback target or reviewed rollback SQL restores nullable tenant compatibility."
  - "Value-gate report maps every criterion to named evidence."
change_layers:
  - migrations
  - scripts
  - docs
  - tests
  - harness
verification_layers:
  - migrations
  - tests
  - docs
blocked_by:
  - TMP-001
  - TMP-002
  - TMP-005
  - TMP-006
  - TMP-008
  - TMP-009
blocks: []
parallel_group: tenant-platform-p3
file_scope:
  allowed:
    - "Makefile"
    - "docs/**"
    - "scripts/**"
    - "slices/TMP-011-migration-and-data-isolation/**"
    - "slices/manifest.json"
    - "services/acquisition-api/migrations/**"
    - "services/subscription-external/migrations/**"
    - "services/**/internal/**"
    - "services/**/cmd/**"
    - "agent/backlog/issues/TMP-011-legacy-data-migration-isolation.md"
    - "agent/state/TMP-011.work-order.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/webspa-admin/**"
    - "package.json"
    - "pnpm-lock.yaml"
---

## Operator story

As a platform operator, I can dry-run, execute, verify, and roll back a tenant-platform migration so legacy global data becomes tenant-safe without data loss.

## Acceptance criteria

- Dry-run reports table-level row counts, unmapped rows, conflict counts, and constraint readiness without mutating data.
- Migration backfills eligible legacy rows to `tenant_key=legacy-default` through an idempotent batched path.
- Rerunning after a partial migration leaves already migrated ownership unchanged and completes remaining eligible rows.
- Rollback target or reviewed rollback SQL preserves original data and restores nullable tenant compatibility.
- Value-gate report maps happy, failure, edge, and invariant criteria to named evidence.
- Existing single-tenant Ghana/TIMWE compatibility is covered by smoke, repository, or SQL verification evidence.

## Required proof

```bash
hvc check agent/backlog/issues/*.md --fail-on block
agent-supervisor --config .harness/config.json preflight
jq empty slices/manifest.json
slice-harness status
```
