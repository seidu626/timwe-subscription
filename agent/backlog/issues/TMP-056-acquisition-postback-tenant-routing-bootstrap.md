---
id: TMP-056
title: "Acquisition postback tenant routing bootstrap"
class: vertical_defect_slice
status: queued
parent_vertical_slice_id: TMP-045
scope_limit: "Make acquisition-api startup apply the canonical postback tenant-routing schema before the in-process postback dispatcher can query tenant-aware postback_outbox columns. Do not add a duplicate postback schema path."
merge_policy: "Merge only after HVC, supervisor preflight, targeted acquisition repository tests, and value-gate evidence pass."
evidence_required:
  - "tail -200 services/acquisition-api/acquisition-api.log shows pq: column \"tenant_id\" does not exist on postback_outbox"
  - "cd services/acquisition-api && go test ./internal/repository"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Acquisition startup bootstrap includes the canonical add_tenant_postback_routing.sql migration."
  - "The bootstrap order preserves the single canonical postback path documented by TMP-045 and does not run subscription-external duplicate postback DDL."
  - "Repository tests assert the migration is included and that the migration adds tenant_id, channel_id, and failure_reason columns required by PostbackRepository."
actor: platform-operator
outcome: "acquisition-api starts without postback dispatcher missing-column errors after admin schema bootstrap completes."
entrypoint: "acquisition-api startup schema bootstrap and postback dispatcher polling"
trigger: "Acquisition API log shows Admin management schema bootstrap completed, then postback dispatcher polling fails because postback_outbox.tenant_id is missing."
broken_outcome: "Bootstrap succeeds, but PostbackRepository.ClaimPendingPostbacks selects tenant_id/channel_id/failure_reason from postback_outbox before the tenant-routing migration has run."
expected_behavior: "The service-local bootstrap applies the canonical postback tenant-routing migration before the dispatcher starts polling."
reproduction:
  command: "tail -200 services/acquisition-api/acquisition-api.log"
  observed: "pq: column \"tenant_id\" does not exist from internal/worker.(*PostbackDispatcher).poll"
  expected: "postback dispatcher empty poll returns zero rows or dispatchable rows without schema errors."
system_path:
  - "Acquisition API connects to PostgreSQL."
  - "Admin management bootstrap applies tenant/admin schema migrations."
  - "Bootstrap applies the canonical acquisition-owned postback tenant-routing migration."
  - "Postback dispatcher starts and can query tenant-aware postback_outbox columns."
change_layers:
  - backend-runtime
  - schema-bootstrap
  - tests
  - evidence
verification_layers:
  - static-audit
  - repository-tests
  - harness
blocked_by: []
blocks:
  - "TMP-021"
parallel_group: acquisition-runtime-schema
non_goals:
  - "Do not mutate a remote database directly in the agent session."
  - "Do not add subscription-external postback DDL to acquisition startup."
  - "Do not introduce compatibility or fallback query paths for tenantless postbacks."
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-056-acquisition-postback-tenant-routing-bootstrap.md"
    - "agent/state/TMP-056.work-order.json"
    - "agent/state/TMP-056.handoff.json"
    - "services/acquisition-api/internal/repository/admin_management_schema.go"
    - "services/acquisition-api/internal/repository/admin_management_schema_test.go"
    - "slices/manifest.json"
    - "slices/TMP-056-acquisition-postback-tenant-routing-bootstrap/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "docker-compose*.yml"
    - "services/subscription-external/**"
---

## Operator Story

As a platform operator, I can start acquisition-api after tenant postback routing is enabled and have the in-process dispatcher poll without missing tenant-aware postback_outbox columns.

## Acceptance Criteria

- The acquisition-api startup schema bootstrap includes `services/acquisition-api/migrations/add_tenant_postback_routing.sql`.
- The bootstrap keeps acquisition-api postback migrations as the single canonical postback path and does not add subscription-external duplicate DDL.
- Targeted repository tests prove the startup migration list and tenant-routing migration cover the columns used by `PostbackRepository`.
