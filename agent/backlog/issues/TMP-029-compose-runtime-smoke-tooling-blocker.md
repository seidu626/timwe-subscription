---
id: TMP-029
title: "Compose smoke Docker auth blocker evidence"
class: operational_slice
status: ready
scope_limit: "Record the bounded compose runtime smoke attempt that advanced past config rendering but failed before app startup on local Docker registry auth/tooling."
merge_policy: "Merge only after HVC, JSON validation, slice-harness, supervisor preflight, and evidence-only scope checks pass."
evidence_required:
  - "docker compose --project-name timwe_smoke_* --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-smoke-override.yml up -d --build ..."
  - "docker pull docker.io/library/golang:1.24-alpine"
  - "slices/TMP-029-compose-runtime-smoke-tooling-blocker/value-gate-report.md"
acceptance_tests:
  - "jq empty slices/manifest.json agent/state/TMP-029.work-order.json agent/state/TMP-029.handoff.json .agent/tasks.json agent/state/TMP-021.handoff.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status && slice-harness sync --dry-run"
actor: verification-agent
outcome: "Compose smoke evidence distinguishes config readiness from local Docker auth/tooling failure before app containers start."
entrypoint: "docs/agent/full-system-verification-2026-05-09.md"
trigger: "Verifier attempts a bounded local compose runtime smoke after TMP-028."
broken_outcome: "Release evidence says compose runtime is blocked only by env/network readiness even after a smoke attempt proves local Docker registry auth fails before service startup."
expected_behavior: "The release matrix records the temporary Redis port override, temporary external network creation, Docker Hub auth failure, cleanup result, and the direct image-pull reproduction."
system_path:
  - "Verification agent renders compose with .env.example."
  - "Verification agent avoids unrelated host port conflict with a temporary override."
  - "Verification agent creates the missing external network temporarily and cleans it up."
  - "Docker/Podman fails to pull the Go builder base image before app containers start."
  - "Release evidence records the blocker as local registry auth/tooling, not an app runtime failure."
change_layers:
  - verification-evidence
  - harness-metadata
verification_layers:
  - harness
  - evidence
blocked_by: []
blocks: ["TMP-021"]
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-029-compose-runtime-smoke-tooling-blocker/**"
    - "agent/backlog/issues/TMP-029-compose-runtime-smoke-tooling-blocker.md"
    - "agent/state/TMP-029.work-order.json"
    - "agent/state/TMP-029.handoff.json"
    - "agent/state/TMP-021.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "common/**"
    - "services/**"
    - "frontend/**"
    - "docker-compose.yml"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
    - "vendor/**"
---

## Operator Story

As a verification agent, I can record the bounded compose runtime smoke blocker precisely, so operators know local Docker registry auth/tooling must be fixed before app startup evidence can be collected.

## Acceptance Criteria

- Release evidence records that compose config rendered and the runtime smoke used a temporary Redis port override plus temporary `shared-network` creation.
- Evidence records that the smoke failed before app containers started because the Go builder image could not be pulled with current local Docker registry auth.
- Direct image pull reproduction is recorded.
- Cleanup evidence records that no smoke containers, temporary override file, or temporary `shared-network` remained.
- No source, compose, dependency, vendor, package manifest, lockfile, or frontend files are changed.
