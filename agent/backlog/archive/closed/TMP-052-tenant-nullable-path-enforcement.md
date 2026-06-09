---
id: TMP-052
title: "Tenant nullable path enforcement"
class: bounded_enabler
status: queued
parent_vertical_slice_id: TMP-050
consumed_by:
  - TMP-051
scope_limit: "Audit tenant-aware tables and codepaths for remaining tenant_id nullable behavior after nrg migration. Produce or implement the next safe enforcement slice for NOT NULL constraints only where repository/runtime proof exists."
merge_policy: "Merge only after HVC, static nullable-path audit, affected tests, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "rg -n \"tenant_id IS NULL|tenant_id = NULL|WHERE .*tenant_id IS NULL|tenant_id UUID\" services scripts docs frontend -g '!**/node_modules/**'"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Remaining tenant nullable codepaths are inventoried and classified as delete_now, collapse_into_canonical, keep_as_permanent_capability, or needs_human_decision."
  - "No enforcement migration is added unless the audit proves it is safe for the touched table set."
  - "Follow-up implementation slices are emitted for any table group that still needs runtime proof."
actor: platform-operator
outcome: "tenant nullable ownership paths are classified for canonical nrg enforcement without speculative schema rewrites."
entrypoint: "tenant_id nullable-path audit"
trigger: "operator requested no legacy/compatibility codepaths after canonical nrg migration."
system_path:
  - "Audit tenant-aware migrations, repositories, scripts, and frontend tenant workspace assumptions."
  - "Classify remaining nullable paths with prune criteria."
  - "Implement safe enforcement only where proof exists, or emit focused follow-up slices."
change_layers:
  - architecture-audit
  - migration-readiness
  - evidence
verification_layers:
  - static-audit
  - hvc
parallel_group: tenant-platform-canonicalization
non_goals:
  - "Do not force NOT NULL constraints across every table without live data proof."
  - "Do not mutate a remote database in this agent session."
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-052-tenant-nullable-path-enforcement.md"
    - "agent/state/TMP-052.work-order.json"
    - "agent/state/TMP-052.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-052-tenant-nullable-path-enforcement/**"
    - "docs/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "docker-compose*.yml"
---

## Operator Story

As a platform operator, I can see which tenant nullable paths remain after the nrg migration so enforcement work proceeds from evidence instead of broad schema rewrites.
