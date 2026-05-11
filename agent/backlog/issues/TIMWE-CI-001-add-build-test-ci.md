---
id: TIMWE-CI-001
title: "Add build/test CI pipeline (Angular, Next.js, Go lanes)"
class: operational_slice
status: queued
scope_limit: "Add GitHub Actions CI for UI and service build/test lanes; do not change application runtime behavior."
merge_policy: "Merge only after all CI lanes run clean on a clean checkout and harness verification."
evidence_required:
  - "gh workflow run ci.yml"
  - "gh run watch --exit-status"
acceptance_tests:
  - "A pull request triggers ci.yml with Angular, Next.js, and Go service lanes."
  - "Angular, Next.js, and Go lanes all complete successfully on a clean checkout."
  - "CI workflow fails on missing lockfiles, tooling setup failures, or unresolved build/test regressions."
actor: platform-engineering
outcome: "PRs run full build/test checks for Angular, Next.js, and Go services before merge."
entrypoint: ".github/workflows/ci.yml"
trigger: "Changes are proposed to frontend, service, or shared common code."
broken_outcome: "CI remains manual or single-surface only; regressions in any lane merge unseen."
expected_behavior: "Every PR runs the full lint/build/test matrix and blocks merge on failures."
reproduction:
  command: "Open a PR containing a known failing service test and confirm ci.yml marks the PR as failed."
  observed: "CI passes only changed lanes or does not run at all."
  expected: "ci.yml runs Angular, Next.js, and Go lanes and reports the regression as failed."
system_path:
  - "ci.yml"
  - ".github/workflows/ci.yml"
verification_layers:
  - automation
  - harness
blocked_by: []
blocks:
  - "TIMWE-SENTRY-001"
parallel_group: ci-platform
non_goals:
  - "Do not modify runtime business logic."
  - "Do not add production deployment flows."
file_scope:
  allowed:
    - "agent/backlog/issues/TIMWE-CI-001-add-build-test-ci.md"
    - "agent/state/TIMWE-CI-001.work-order.json"
    - "agent/state/TIMWE-CI-001.handoff.json"
    - ".github/workflows/ci.yml"
    - "slices/TIMWE-CI-001/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "frontend/**/package-lock.json"
    - ".env"
---

## Operator Story

As a platform engineer, I can open a PR and have Angular, Next.js, and Go service lanes run in CI so regressions are caught before merge.
