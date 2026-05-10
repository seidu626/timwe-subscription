---
id: TMP-024
title: "Slice registry evidence reconciliation"
class: operational_slice
status: done
scope_limit: "Reconcile shipped slice registry metadata with already-accepted TMP-022 and TMP-023 evidence."
merge_policy: "Merge only after manifest JSON, HVC, and slice-harness status/sync checks pass."
evidence_required:
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "jq '.slices[] | select(.id==\"TMP-022\" or .id==\"TMP-023\")' slices/manifest.json"
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Release-readiness reports and control-plane dashboards show truthful slice states and evidence paths."
entrypoint: "slices/manifest.json"
trigger: "Operator inspects shipped full-system verification state."
broken_outcome: "TMP-022 points at TMP-023 evidence and TMP-023 remains planned despite accepted PASS evidence."
expected_behavior: "TMP-022 references landing-web build evidence and TMP-023 is done with common package test evidence."
system_path:
  - "Slice manifest is opened."
  - "TMP-022 and TMP-023 metadata are read by status tooling."
  - "Operator sees state and verification evidence matching accepted handoffs."
change_layers:
  - slice-registry
verification_layers:
  - harness
  - metadata
blocked_by: []
blocks: []
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "slices/manifest.json"
    - "slices/TMP-024-slice-registry-evidence-reconciliation/**"
    - "agent/backlog/issues/TMP-024-slice-registry-evidence-reconciliation.md"
    - "agent/state/TMP-024.work-order.json"
    - ".agent/**"
  forbidden:
    - "common/**"
    - "services/**"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
---

## Operator Story

As a platform operator, I can trust the slice registry after full-system verification so release-readiness status reflects the evidence that was actually accepted.

## Acceptance Criteria

- TMP-022 automated verification points to `cd services/landing-web && npm run build`.
- TMP-022 DoD path points to `slices/TMP-022-landing-web-dynamic-route-build/value-gate-report.md`.
- TMP-023 state is `done`.
- TMP-023 automated verification points to `cd common && go test ./...`.
- TMP-023 DoD path points to `slices/TMP-023-common-package-test-failures/value-gate-report.md`.
- No source, dependency, vendor, or package files change.
