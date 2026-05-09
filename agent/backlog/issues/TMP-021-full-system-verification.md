---
id: TMP-021
title: "Release verification matrix"
class: operational_slice
status: blocked
scope_limit: "Create an evidence matrix for discovered runnable components, implemented tenant-platform features, harness state, and runtime/build health. Do not implement product features inside this slice; create focused defect slices for concrete failures that require code changes."
merge_policy: "Merge only after the full-system verification matrix, supervisor preflight, HVC, representative build/test commands, and value-gate evidence are recorded."
evidence_required:
  - "agent-supervisor --config .harness/config.json preflight"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "docs/agent/full-system-verification-2026-05-09.md"
  - "slices/TMP-021-full-system-verification/value-gate-report.md"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "agent-supervisor --config .harness/config.json preflight"
  - "test -f docs/agent/full-system-verification-2026-05-09.md"
  - "test -f slices/TMP-021-full-system-verification/value-gate-report.md"
non_goals:
  - "No production deploy."
  - "No dependency additions or upgrades."
  - "No broad refactor."
actor: platform-operator
outcome: "Operator has evidence-backed release-readiness status for every discovered runnable component and implemented feature, including passed, fixed, blocked, failed, not applicable, or not implemented rows."
entrypoint: "docs/agent/full-system-verification-2026-05-09.md"
trigger: "Operator requests end-to-end release verification"
system_path:
  - "Discover runnable-component and feature inventory from source, manifests, docs, and tests."
  - "Run control-plane, build, test, and smoke checks where the local environment supports them."
  - "Record blocked checks with exact unblocking requirements instead of claiming proxy success."
change_layers:
  - docs
  - harness
verification_layers:
  - control-plane
  - build
  - tests
  - runtime-smoke
blocked_by: []
blocks: []
parallel_group: tenant-platform-verification
file_scope:
  allowed:
    - "docs/agent/**"
    - "slices/TMP-021-full-system-verification/**"
    - "slices/manifest.json"
    - "agent/backlog/issues/TMP-021-full-system-verification.md"
    - "agent/state/TMP-021.work-order.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**"
    - "common/**"
    - "frontend/**"
    - "ops/**"
    - "docker-compose*.yml"
    - "Makefile"
---

## Operator story

As a platform operator, I can inspect a release verification matrix so that readiness is based on real build, test, runtime, feature, and blocked-check evidence.

## Acceptance criteria

- Service inventory lists every discovered runnable component with canonical or derived build, test, start, and smoke commands.
- Feature inventory maps implemented tenant-platform features to source evidence, invariants, interfaces, and verification method.
- Verification matrix records command results with one of: passed, fixed, failed, blocked, not applicable, or not implemented.
- Control-plane drift, git divergence, runtime blockers, and environment limitations are documented explicitly.
- Value-gate report maps the audit criteria to concrete commands and artifacts.
