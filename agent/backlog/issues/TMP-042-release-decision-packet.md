---
id: TMP-042
title: "Release blocker decision packet"
class: operational_slice
status: done
scope_limit: "Consolidate the exact operator decisions required to unblock TMP-021, TMP-026, TMP-034, TMP-035, TMP-036, TMP-037, and TMP-038. Do not change schema, migrations, runtime code, compose files, dependencies, package manifests, credentials, submodule contents, or branch state."
merge_policy: "Merge only after HVC, slice-harness, supervisor preflight, JSON validity, value-gate evidence, and file-scope checks pass."
evidence_required:
  - "docs/agent/release-decision-packet-2026-05-09.md"
  - "slices/TMP-042-release-decision-packet/value-gate-report.md"
  - "agent/state/TMP-042.handoff.json"
acceptance_tests:
  - "test -f docs/agent/release-decision-packet-2026-05-09.md"
  - "test -f slices/TMP-042-release-decision-packet/value-gate-report.md"
  - "jq empty slices/manifest.json agent/state/TMP-042.work-order.json agent/state/TMP-042.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "The remaining full-system verification blockers are reduced to explicit decision records with choices, affected slices, and verification proof required after approval."
entrypoint: "Full-system verification blocked queue"
trigger: "Supervisor reports no ready tasks while release verification remains blocked."
broken_outcome: "Repeated control-plane checks say no ready tasks, but the operator lacks a single packet naming the decisions needed to make the blocked work executable."
expected_behavior: "The decision packet names each blocked slice, the concrete decision options, the minimum approval artifact required, and the verification commands to run after the decision."
system_path:
  - "Verifier reads blocked slice handoffs and value-gate reports."
  - "Verifier consolidates the required decisions without choosing on behalf of the operator."
  - "Future implementation can proceed only after a decision artifact is recorded for the relevant blocker."
change_layers:
  - evidence
  - harness
verification_layers:
  - control-plane
  - metadata
blocked_by: []
blocks:
  - "TMP-021"
  - "TMP-026"
  - "TMP-034"
  - "TMP-035"
  - "TMP-036"
  - "TMP-037"
  - "TMP-038"
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-042-release-decision-packet.md"
    - "agent/state/TMP-042.work-order.json"
    - "agent/state/TMP-042.handoff.json"
    - "docs/agent/release-decision-packet-2026-05-09.md"
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-042-release-decision-packet/**"
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
    - "*.sql"
    - ".git/**"
---

## Operator Story

As a platform operator, I can review one decision packet for all remaining release-verification blockers, so I can approve, reject, or defer the exact work needed to make the next implementation slices executable.

## Acceptance Criteria

- The packet lists the seven blocked slices and the decision each needs.
- Each decision row names the allowed implementation choices and the evidence that must be produced after approval.
- The packet does not record an approval and does not choose schema, dependency, branch, or gitlink strategy.
- No service, migration, SQL, compose, dependency, package, credential, runtime, submodule, or branch-integration files change.
