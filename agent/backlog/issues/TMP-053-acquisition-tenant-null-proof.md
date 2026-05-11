---
id: TMP-053
title: "Acquisition tenant nullable proof"
class: bounded_enabler
status: queued
parent_vertical_slice_id: TMP-050
consumed_by:
  - TMP-055
scope_limit: "Produce read-only proof that acquisition-api tenant-owned tables have no remaining tenant_id NULL rows after the canonical nrg migration. Do not mutate a remote database."
merge_policy: "Merge only after HVC, read-only SQL proof or explicit credential blocker evidence, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "read-only tenantless row-count SQL for campaigns, acquisition_transactions, postback_outbox, products, userbase, userbase_import_jobs, userbase_import_errors, and admin_activity_logs"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Acquisition/admin tenant-owned tables have row-count proof for tenant_id IS NULL."
  - "If credentials are unavailable, blocker evidence names the exact missing env/tool."
  - "No remote database mutation is performed."
actor: platform-operator
outcome: "acquisition/admin tenant-owned tables are proven ready or blocked for canonical tenant enforcement."
entrypoint: "read-only acquisition tenantless row-count audit"
trigger: "TMP-052 found acquisition runtime and migration nullable tenant paths."
system_path:
  - "Resolve database connection from documented env only."
  - "Run read-only row-count checks for acquisition/admin table group."
  - "Record proof for TMP-055 enforcement."
change_layers:
  - evidence
  - migration-readiness
verification_layers:
  - static-audit
  - read-only-db-proof
parallel_group: tenant-platform-canonicalization
non_goals:
  - "Do not add NOT NULL constraints in this proof slice."
  - "Do not change acquisition runtime code in this proof slice."
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-053-acquisition-tenant-null-proof.md"
    - "agent/state/TMP-053.work-order.json"
    - "agent/state/TMP-053.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-053-acquisition-tenant-null-proof/**"
    - "docs/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "docker-compose*.yml"
---

## Operator Story

As a platform operator, I can prove the acquisition/admin table group has no remaining tenantless rows before code or schema enforcement removes nullable tenant paths.
