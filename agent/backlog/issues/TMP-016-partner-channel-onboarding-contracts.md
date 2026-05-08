---
id: TMP-016
title: "Partner channel onboarding contracts"
class: bounded_enabler
status: ready
parent_vertical_slice_id: TMP-016
consumed_by:
  - TMP-003
  - TMP-004
  - TMP-007
  - TMP-012
  - TMP-013
scope_limit: "Create versioned tenant/channel onboarding docs and sandbox fixtures. Do not implement runtime subscription, callback, migration, or UI behavior."
merge_policy: "Merge only after HVC, supervisor preflight, contract fixture review, and value-gate report pass."
evidence_required:
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "agent-supervisor --config .harness/config.json preflight"
  - "contract fixture evidence"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "test -f docs/tenant-channel-onboarding.md"
non_goals:
  - "No live partner credential provisioning."
  - "No backend route rewrites."
actor: api-integrated-partner
outcome: "New tenant channels can be onboarded without bespoke engineering discovery."
entrypoint: "docs/tenant-channel-onboarding.md and sandbox fixtures"
trigger: "Partner begins tenant channel onboarding"
system_path:
  - "Onboarding document names API, callback, credential, sandbox, and postback contracts."
  - "Fixtures cover supported and unsupported capability examples."
  - "Legacy mapping ambiguity is documented with safe fallback behavior."
change_layers:
  - docs
  - examples
  - harness
verification_layers:
  - docs
blocked_by:
  - TMP-003
  - TMP-004
  - TMP-007
  - TMP-012
  - TMP-013
blocks: []
parallel_group: tenant-platform-contracts
file_scope:
  allowed:
    - "docs/**"
    - "examples/**"
    - "slices/TMP-016-partner-channel-onboarding-contracts/**"
    - "services/**/docs/**"
    - "services/**/README.md"
    - "agent/backlog/issues/TMP-016-partner-channel-onboarding-contracts.md"
    - "agent/state/TMP-016.work-order.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/webspa-admin/**"
    - "services/**/internal/**"
    - "Makefile"
---

## Operator story

As an API-integrated partner, I can follow a versioned tenant/channel contract pack to integrate without bespoke discovery.

## Acceptance criteria

- Onboarding document names tenant/channel API, callback, credential, sandbox, and postback contracts.
- Sandbox fixtures include supported and unsupported capability examples plus legacy mapping ambiguity.
- Callback signature and credential redaction guidance are explicit.
- Value-gate report maps contract criteria to concrete files and examples.
