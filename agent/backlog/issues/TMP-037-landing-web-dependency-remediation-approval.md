---
id: TMP-037
title: "Landing web dependency remediation approval"
class: operational_slice
status: done
scope_limit: "Classify and track the dependency-change approval blocker. Do not change package manifests, lockfiles, frontend code, dependencies, or runtime behavior in this slice."
merge_policy: "Merge this registry slice only after HVC, slice-harness, supervisor preflight, value-gate evidence, and file-scope checks pass. The underlying implementation remains blocked until the named approval or operator decision is recorded."
evidence_required:
  - "npm audit --audit-level=moderate"
  - "docs/agent/full-system-verification-2026-05-09.md"
  - "slices/TMP-021-full-system-verification/value-gate-report.md"
  - "slices/TMP-037-landing-web-dependency-remediation-approval/value-gate-report.md"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "test -f slices/TMP-037-landing-web-dependency-remediation-approval/value-gate-report.md"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Landing-web dependency vulnerability remediation has an explicit approval gate before breaking Next/PostCSS upgrades are attempted."
entrypoint: "services/landing-web/package.json and package-lock.json"
trigger: "Verifier runs npm audit after landing-web build passes."
broken_outcome: "npm audit reports Next/PostCSS advisories and npm audit fix proposes a breaking Next upgrade to next@16.2.6."
expected_behavior: "Dependency upgrade scope, risk, and UI regression proof are approved before package manifests or lockfiles change."
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
blocked_by: []
blocks:
  - "TMP-021"
parallel_group: release-verification-blockers
file_scope:
  allowed:
  - "agent/backlog/issues/TMP-037-landing-web-dependency-remediation-approval.md"
  - "agent/state/TMP-037.work-order.json"
  - "agent/state/TMP-037.handoff.json"
  - "slices/manifest.json"
  - "slices/TMP-037-landing-web-dependency-remediation-approval/**"
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
---

## Operator Story

As a platform-operator, I can see TMP-037 as a distinct blocked slice so the full-system verification backlog does not hide this blocker inside prose.

## Acceptance Criteria

- Explicit dependency-change approval is recorded before implementation.
- A future implementation reruns npm audit and landing-web build/UI regression checks.
- No package, lockfile, frontend, dependency, or runtime files are changed by this registry slice.

## Approval Record

- Approved by: operator auto-proceed directive in this Codex session
- Approved at: 2026-05-10T05:23:39Z
- Scope approved: create a bounded implementation slice for the breaking Next/PostCSS remediation, including package/lockfile updates and build/runtime regression proof.
