---
id: TMP-027
title: "Release matrix stale blocker reconciliation"
class: operational_slice
status: done
scope_limit: "Reconcile TMP-021 release-verification evidence after current origin/main proves subscription-partner and notification default tests plus canonical local build pass."
merge_policy: "Merge only after HVC, slice-harness, supervisor preflight, JSON validation, and evidence artifact checks pass."
evidence_required:
  - "cd services/subscription-partner && go test ./..."
  - "cd services/notification && go test ./..."
  - "make build-all-local"
  - "slices/TMP-027-release-matrix-stale-blocker-reconciliation/value-gate-report.md"
  - "agent/state/TMP-021.handoff.json"
acceptance_tests:
  - "cd services/subscription-partner && go test ./..."
  - "cd services/notification && go test ./..."
  - "make build-all-local"
  - "jq empty slices/manifest.json agent/state/TMP-027.work-order.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Operator-inspectable release matrix accurately reflects current Go test and build pass evidence while keeping remaining real blockers visible."
entrypoint: "docs/agent/full-system-verification-2026-05-09.md"
trigger: "Operator reviews current full-system verification state after new origin/main evidence."
broken_outcome: "TMP-021 still says notification and subscription-partner dependency/vendor repairs are blocked even though current default tests and canonical local build pass."
expected_behavior: "TMP-021 and the full-system matrix keep only current blockers and record the passing commands that retired stale blockers."
system_path:
  - "Operator checks current origin/main."
  - "Go service tests and canonical local build run."
  - "Full-system verification artifact is reconciled."
  - "Harness state records TMP-027 as the evidence correction slice."
change_layers:
  - verification-evidence
  - harness-state
verification_layers:
  - harness
  - service-tests
  - build
blocked_by: []
blocks: ["TMP-021"]
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-027-release-matrix-stale-blocker-reconciliation/**"
    - "agent/backlog/issues/TMP-027-release-matrix-stale-blocker-reconciliation.md"
    - "agent/state/TMP-027.work-order.json"
    - "agent/state/TMP-027.handoff.json"
    - "agent/state/TMP-021.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "common/**"
    - "services/**"
    - "frontend/**"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
    - "vendor/**"
---

## Operator Story

As a platform operator, I can trust the release matrix to distinguish current blockers from already-resolved failures, so release-readiness decisions are based on current verification evidence.

## Acceptance Criteria

- Current `services/subscription-partner` default tests pass and are recorded.
- Current `services/notification` default tests pass and are recorded.
- Current `make build-all-local` passes and is recorded.
- TMP-021 no longer lists notification/subscription-partner dependency/vendor metadata repair as a current blocker.
- Runtime/env, webspa-admin submodule, dependency vulnerability, and local-main divergence blockers remain visible.
- No source, dependency, vendor, package manifest, lockfile, or frontend files are changed.
