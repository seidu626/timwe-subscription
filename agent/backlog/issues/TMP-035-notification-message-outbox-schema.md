---
id: TMP-035
title: "Notification message outbox schema provisioning"
class: vertical_defect_slice
status: blocked
scope_limit: "Classify and track the approval-gated notification message_outbox schema blocker discovered after TMP-031. Do not change schema, migrations, runtime code, compose files, dependencies, or credentials in this slice."
merge_policy: "Merge this registry slice only after HVC, slice-harness, supervisor preflight, value-gate evidence, and file-scope checks pass. The underlying implementation remains blocked until the named approval or operator decision is recorded."
evidence_required:
  - "targeted notification-worker compose smoke"
  - "docs/agent/full-system-verification-2026-05-09.md"
  - "slices/TMP-031-notification-worker-compose-db-env/value-gate-report.md"
  - "slices/TMP-035-notification-message-outbox-schema/value-gate-report.md"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "test -f slices/TMP-035-notification-message-outbox-schema/value-gate-report.md"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Notification worker dispatch loop runs in compose without missing message_outbox relation errors."
entrypoint: "docker compose notification-worker runtime startup"
trigger: "Verifier runs targeted notification-worker smoke after TMP-031 DB env fix."
broken_outcome: "Notification worker starts and exposes metrics, then dispatcher logs pq: relation message_outbox does not exist against the empty compose DB."
expected_behavior: "The compose DB provisioning path applies the message cadence/outbox schema before notification-worker dispatch polling."
reproduction:
  command: "targeted notification-worker compose smoke after TMP-031 DB env fix"
  observed: "Worker starts and metrics endpoint starts, then dispatcher logs pq: relation message_outbox does not exist."
  expected: "Notification worker dispatch loop runs without missing message_outbox relation errors."
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
  - "agent/backlog/issues/TMP-035-notification-message-outbox-schema.md"
  - "agent/state/TMP-035.work-order.json"
  - "agent/state/TMP-035.handoff.json"
  - "slices/manifest.json"
  - "slices/TMP-035-notification-message-outbox-schema/**"
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
---

## Operator Story

As a platform-operator, I can see TMP-035 as a distinct blocked slice so the full-system verification backlog does not hide this blocker inside prose.

## Acceptance Criteria

- Schema/migration change approval is recorded before implementation.
- A future implementation reruns targeted notification-worker compose smoke and confirms no message_outbox missing-relation errors.
- No schema, migration, compose, source, dependency, or credential file changes are made by this registry slice.
