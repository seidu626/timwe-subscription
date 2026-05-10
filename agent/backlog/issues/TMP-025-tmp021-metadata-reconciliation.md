---
id: TMP-025
title: "TMP-021 release matrix metadata reconciliation"
class: operational_slice
status: done
scope_limit: "Reconcile TMP-021 manifest and value-gate metadata with its accepted blocked handoff."
merge_policy: "Merge only after manifest JSON, HVC, and slice-harness status/sync checks pass."
evidence_required:
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "jq '.slices[] | select(.id==\"TMP-021\")' slices/manifest.json"
  - "test -f slices/TMP-021-full-system-verification/value-gate-report.md"
  - "jq empty slices/manifest.json"
  - "slice-harness status"
actor: platform-operator
outcome: "Release verification status shows TMP-021 blocked with its own evidence instead of another slice's landing-web evidence."
entrypoint: "slices/manifest.json and TMP-021 value-gate report"
trigger: "Operator inspects release-readiness status after full-system verification."
broken_outcome: "TMP-021 is marked done and points to TMP-022 evidence while its accepted handoff is blocked."
expected_behavior: "TMP-021 is blocked and points to its own full-system verification value-gate report."
system_path:
  - "Slice manifest is opened."
  - "TMP-021 status and DoD path are inspected."
  - "Operator sees blocked release-verification status with the exact blockers."
change_layers:
  - slice-registry
  - evidence
verification_layers:
  - harness
  - metadata
blocked_by: []
blocks: []
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-025-tmp021-metadata-reconciliation/**"
    - "agent/backlog/issues/TMP-025-tmp021-metadata-reconciliation.md"
    - "agent/state/TMP-025.work-order.json"
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

As a platform operator, I can trust TMP-021 release verification status because it points to the full-system audit evidence and preserves the blocked release gates.

## Acceptance Criteria

- TMP-021 manifest state is `blocked`.
- TMP-021 manifest automated commands are release-verification evidence commands, not landing-web build.
- TMP-021 manifest DoD path is `slices/TMP-021-full-system-verification/value-gate-report.md`.
- TMP-021 value-gate report verdict is `BLOCKED` with `outcome:blocked`.
- No source, dependency, vendor, or package files change.
