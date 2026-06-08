---
id: TMP-041
title: "Runtime schema blocker source inventory"
class: operational_slice
status: done
scope_limit: "Record exact existing SQL sources behind TMP-034, TMP-035, and TMP-036 runtime schema blockers. Do not change schema, migrations, runtime code, compose files, dependencies, package manifests, credentials, or branch state."
merge_policy: "Merge only after HVC, slice-harness, supervisor preflight, JSON validity, value-gate evidence, and file-scope checks pass."
evidence_required:
  - "services/pg_schema.sql products and userbase table definitions"
  - "services/subscription-external/migrations/011_message_cadence_engine.sql message_outbox definition"
  - "services/acquisition-api/migrations/create_postback_tables.sql postback_outbox definition"
  - "services/subscription-external/migrations/006_web_acquisition_campaigns.sql duplicate postback_outbox definition"
  - "slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md"
acceptance_tests:
  - "test -f slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md"
  - "jq empty slices/manifest.json agent/state/TMP-041.work-order.json agent/state/TMP-041.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Runtime schema blockers identify the exact existing SQL sources and duplicate-source risks before any migration orchestration decision."
entrypoint: "TMP-034, TMP-035, and TMP-036 release-verification blocker evidence"
trigger: "Verifier audits schema-related blocked slices after supervisor reports no ready tasks."
broken_outcome: "Schema-related release blockers say provisioning is required but do not distinguish missing SQL definitions from missing runtime migration orchestration."
expected_behavior: "Evidence names the existing SQL definitions, warns about duplicate/hand-maintained sources, and preserves the approval gate for applying them to the compose runtime."
system_path:
  - "Verifier inspects migration and schema files."
  - "Verifier maps runtime missing-relation symptoms to concrete SQL source files."
  - "Verifier records duplicate-source and ordering risks."
  - "Future implementation can decide the migration orchestration artifact with operator approval."
change_layers:
  - evidence
  - harness
verification_layers:
  - control-plane
  - metadata
blocked_by: []
blocks: []
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-041-runtime-schema-source-inventory.md"
    - "agent/state/TMP-041.work-order.json"
    - "agent/state/TMP-041.handoff.json"
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-034-acquisition-runtime-schema-provisioning/value-gate-report.md"
    - "slices/TMP-035-notification-message-outbox-schema/value-gate-report.md"
    - "slices/TMP-036-postback-outbox-schema/value-gate-report.md"
    - "slices/TMP-041-runtime-schema-source-inventory/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**"
    - "common/**"
    - "frontend/**"
    - "ops/**"
    - "docker-compose*.yml"
    - "Makefile"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
    - "*.sql"
    - ".git/**"
---

## Operator Story

As a platform operator, I can see whether runtime schema blockers are missing SQL definitions or missing approved migration orchestration, so the next release decision is about the correct artifact.

## Acceptance Criteria

- TMP-034, TMP-035, and TMP-036 evidence maps each missing relation to existing SQL source files.
- Duplicate `postback_outbox` definitions and hand-maintained `pg_schema.sql` risks are recorded.
- The remaining blocker is named as approved migration provisioning/orchestration for the compose runtime, not unknown schema design.
- No service, migration, SQL, compose, dependency, package, credential, runtime, or branch-integration files change.
