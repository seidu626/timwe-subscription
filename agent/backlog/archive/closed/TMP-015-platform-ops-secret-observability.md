---
id: TMP-015
title: "Platform ops secret observability"
class: operational_slice
status: done
parent_vertical_slice_id: TMP-015
scope_limit: "Harden platform ops docs/config for secret hygiene and tenant/channel observability labels. Do not implement admin UI or partner onboarding contracts."
merge_policy: "Merge only after HVC, supervisor preflight, docs/config checks, and value-gate report pass."
evidence_required:
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "agent-supervisor --config .harness/config.json preflight"
  - "tenant/channel observability evidence"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "test -f slices/TMP-015-platform-ops-secret-observability/value-gate-report.md"
non_goals:
  - "No production secret rotation execution."
  - "No unrelated dependency changes."
actor: platform-operator
outcome: "Tenant channels operate with safe credentials and visible tenant-specific health."
entrypoint: "docker-compose/config/docs/ops monitoring"
trigger: "Platform operator reviews or runs tenant channel operations"
system_path:
  - "Credential-shaped config is documented or guarded without exposing secret material."
  - "Tenant/channel labels avoid PII and high-cardinality values."
  - "Runbooks cover secret backend unavailable and unsafe local config failure modes."
change_layers:
  - ops
  - config
  - docs
  - tests
  - harness
verification_layers:
  - ops
  - docs
blocked_by:
  - TMP-004
  - TMP-007
  - TMP-010
  - TMP-017
  - TMP-020
blocks: []
parallel_group: tenant-platform-ops
file_scope:
  allowed:
    - "config/**"
    - "docker-compose*.yml"
    - "docs/**"
    - "ops/**"
    - "scripts/**"
    - "slices/TMP-015-platform-ops-secret-observability/**"
    - "services/**/internal/observability/**"
    - "services/**/internal/config/**"
    - "agent/backlog/issues/TMP-015-platform-ops-secret-observability.md"
    - "agent/state/TMP-015.work-order.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/webspa-admin/**"
    - "package.json"
    - "pnpm-lock.yaml"
---

## Operator story

As a platform operator, I can operate tenant channels with credential hygiene and tenant/channel health signals that do not leak secrets or PII.

## Acceptance criteria

- Credential-shaped config is documented or guarded without exposing secret material.
- Tenant/channel observability labels avoid PII and high-cardinality values.
- Secret backend unavailable and unsafe config failure modes have value-gate evidence.
- Production readiness checklist references default tenant, gateway trust, metrics, and health.
