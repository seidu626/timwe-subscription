---
id: TMP-031
title: "Notification worker compose DB env"
class: vertical_defect_slice
status: done
scope_limit: "Fix notification-worker compose runtime DB connection configuration so the worker can ping the local compose Postgres without source, dependency, vendor, package, frontend, credential, or schema changes."
merge_policy: "Merge only after compose config render, targeted notification-worker smoke, HVC, JSON validation, slice-harness, supervisor preflight, and source-scope checks pass."
evidence_required:
  - "docker compose --env-file .env.example -f docker-compose.yml config"
  - "targeted notification-worker compose smoke"
  - "slices/TMP-031-notification-worker-compose-db-env/value-gate-report.md"
acceptance_tests:
  - "docker compose --env-file .env.example -f docker-compose.yml config"
  - "targeted notification-worker compose smoke"
  - "jq empty slices/manifest.json agent/state/TMP-031.work-order.json agent/state/TMP-031.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status && slice-harness sync --dry-run"
actor: verification-agent
outcome: "Notification worker connects to the local compose Postgres with explicit user, password, database, and sslmode settings."
entrypoint: "docker-compose.yml notification-worker service"
trigger: "Verifier runs bounded compose runtime smoke."
broken_outcome: "Notification worker exits during compose smoke while pinging Postgres because its compose env only overrides DB host and leaves the worker to use incomplete/default DB settings."
expected_behavior: "Notification worker compose env supplies the same local Postgres connection fields used by the rest of the stack, including sslmode=disable."
reproduction:
  command: "docker compose --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml up -d --build database notification-worker"
  observed: "notification-worker exits on DB ping before it can run dispatch loop/metrics."
  expected: "notification-worker starts and remains running long enough to expose its metrics listener."
system_path:
  - "Verifier starts compose database."
  - "Verifier starts notification-worker."
  - "Worker loads notification config."
  - "Worker opens and pings Postgres."
  - "Worker starts dispatcher loop and metrics server."
change_layers:
  - compose-runtime-config
verification_layers:
  - compose-config
  - runtime-smoke
  - harness
blocked_by: []
blocks: ["TMP-021"]
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - "docker-compose.yml"
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-031-notification-worker-compose-db-env/**"
    - "agent/backlog/issues/TMP-031-notification-worker-compose-db-env.md"
    - "agent/state/TMP-031.work-order.json"
    - "agent/state/TMP-031.handoff.json"
    - "agent/state/TMP-021.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/notification/**"
    - "common/**"
    - "services/*/go.mod"
    - "services/*/go.sum"
    - "services/*/vendor/**"
    - "frontend/**"
    - "package.json"
    - "package-lock.json"
    - "vendor/**"
---

## Operator Story

As a verification agent, I can start notification-worker against the local compose database, so the full compose smoke can verify worker startup separately from provider/live-flow dependencies.

## Acceptance Criteria

- `notification-worker` compose env includes local Postgres host, port, user, password, database, and `sslmode=disable`.
- Compose config renders with `.env.example`.
- Targeted notification-worker smoke starts database and notification-worker; worker remains running long enough to expose metrics.
- No notification source, dependency metadata, vendor, frontend, package manifest, lockfile, credential, or schema files are changed.
