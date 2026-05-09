---
id: TMP-038
title: "Local main integration strategy"
class: operational_slice
status: blocked
scope_limit: "Classify and track the local-main integration decision blocker. Do not merge, reset, delete branches, or resolve conflicts in this slice."
merge_policy: "Merge this registry slice only after HVC, slice-harness, supervisor preflight, value-gate evidence, and file-scope checks pass. The underlying implementation remains blocked until the named approval or operator decision is recorded."
evidence_required:
  - "git status --short --branch in primary checkout"
  - "failed git merge --no-edit origin/main probe"
  - "docs/agent/full-system-verification-2026-05-09.md"
  - "slices/TMP-038-local-main-integration-strategy/value-gate-report.md"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "test -f slices/TMP-038-local-main-integration-strategy/value-gate-report.md"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: repo-maintainer
outcome: "The divergent primary local main branch is reconciled with origin/main through an explicit integration strategy instead of accidental merge conflict resolution."
entrypoint: "/home/xper626/workspace/apps/timwe-subscription main branch"
trigger: "Verifier compares primary checkout main against origin/main during full-system verification."
broken_outcome: "Primary local main is clean but diverged from origin/main; the latest dated snapshot lives in the TMP-038 value-gate report. An isolated merge probe produced broad add/add conflicts."
expected_behavior: "A maintainer chooses whether to preserve local-only history, reset to remote, or manually integrate the divergent histories before treating primary main as verified."
system_path:
  - "Full-system verifier reads the release matrix blocker."
  - "Blocker is classified into a concrete slice."
  - "Operator sees the approval or decision gate before implementation."
  - "Future implementation can run the listed acceptance proof after the gate is cleared."
change_layers:
  - harness
  - evidence
verification_layers:
  - control-plane
  - metadata
blocked_by:
  - "operator-approval"
blocks:
  - "TMP-021"
parallel_group: release-verification-blockers
file_scope:
  allowed:
  - "agent/backlog/issues/TMP-038-local-main-integration-strategy.md"
  - "agent/state/TMP-038.work-order.json"
  - "agent/state/TMP-038.handoff.json"
  - "slices/manifest.json"
  - "docs/agent/full-system-verification-2026-05-09.md"
  - "slices/TMP-038-local-main-integration-strategy/**"
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
  - ".git/**"
---

## Operator Story

As a repo-maintainer, I can see TMP-038 as a distinct blocked slice so the full-system verification backlog does not hide this blocker inside prose.

## Acceptance Criteria

- A human integration strategy is recorded before destructive or conflict-heavy branch operations.
- A future implementation verifies primary main and origin/main after the chosen strategy.
- No branch resets, merges, conflict resolutions, source files, or runtime files are changed by this registry slice.
