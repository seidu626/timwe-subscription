---
id: TMP-054
title: "Subscription cadence tenant nullable proof"
class: bounded_enabler
status: queued
parent_vertical_slice_id: TMP-050
consumed_by:
  - TMP-055
scope_limit: "Produce read-only proof that subscription-external and cadence-engine tenant-owned tables have no remaining tenant_id NULL rows after the canonical nrg migration. Do not mutate a remote database."
merge_policy: "Merge only after HVC, read-only SQL proof or explicit credential blocker evidence, supervisor preflight, and handoff evidence pass."
evidence_required:
  - "read-only tenantless row-count SQL for subscriptions, notifications, admin_subscription_action_logs, product_message_series, message_content_items, subscription_message_state, and message_outbox"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Subscription/cadence tenant-owned tables have row-count proof for tenant_id IS NULL."
  - "Cadence runtime nullable join candidates are mapped to the tables they depend on."
  - "No remote database mutation is performed."
actor: platform-operator
outcome: "subscription/cadence tenant-owned tables are proven ready or blocked for canonical tenant enforcement."
entrypoint: "read-only subscription cadence tenantless row-count audit"
trigger: "TMP-052 found cadence nullable joins and subscription/cadence nullable schema paths."
system_path:
  - "Resolve database connection from documented env only."
  - "Run read-only row-count checks for subscription/cadence table group."
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
  - "Do not change subscription or cadence runtime code in this proof slice."
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-054-subscription-cadence-tenant-null-proof.md"
    - "agent/state/TMP-054.work-order.json"
    - "agent/state/TMP-054.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-054-subscription-cadence-tenant-null-proof/**"
    - "docs/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "docker-compose*.yml"
---

## Operator Story

As a platform operator, I can prove the subscription/cadence table group has no remaining tenantless rows before runtime joins and schema constraints stop accepting nullable tenant ownership.
