---
id: TMP-032
title: "Postback dispatcher compose DB env"
class: vertical_defect_slice
status: done
scope_limit: "Fix postback-dispatcher compose runtime DB env names so the common config loader reads the local compose database settings without source, dependency, vendor, package, frontend, credential, or schema changes."
merge_policy: "Merge only after compose config render, targeted postback-dispatcher smoke, HVC, JSON validation, slice-harness, supervisor preflight, and source-scope checks pass."
evidence_required:
  - "docker compose --env-file .env.example -f docker-compose.yml config"
  - "targeted postback-dispatcher compose smoke"
  - "slices/TMP-032-postback-dispatcher-compose-db-env/value-gate-report.md"
acceptance_tests:
  - "docker compose --env-file .env.example -f docker-compose.yml config"
  - "targeted postback-dispatcher compose smoke"
  - "jq empty slices/manifest.json agent/state/TMP-032.work-order.json agent/state/TMP-032.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status && slice-harness sync --dry-run"
actor: verification-agent
outcome: "Postback dispatcher connects to local compose Postgres using env names read by common/config."
entrypoint: "docker-compose.yml postback-dispatcher service"
trigger: "Verifier runs bounded compose runtime smoke."
broken_outcome: "Postback dispatcher uses localhost/default DB settings in compose because the service env uses DATABASE_POSTGRESQL_* names that common/config does not bind."
expected_behavior: "Postback dispatcher compose env supplies DB_POSTGRESQL_* aliases, including sslmode=disable, so startup connects to the compose database service."
reproduction:
  command: "docker compose --env-file .env.example -f docker-compose.yml up -d --build database postback-dispatcher"
  observed: "postback-dispatcher retries against localhost DB instead of the compose database service."
  expected: "postback-dispatcher connects to database:5432 and starts its worker loop."
system_path:
  - "Verifier starts compose database."
  - "Verifier starts postback-dispatcher."
  - "Dispatcher loads common config."
  - "Dispatcher opens and pings Postgres."
  - "Dispatcher starts polling postback_outbox."
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
    - "slices/TMP-032-postback-dispatcher-compose-db-env/**"
    - "agent/backlog/issues/TMP-032-postback-dispatcher-compose-db-env.md"
    - "agent/state/TMP-032.work-order.json"
    - "agent/state/TMP-032.handoff.json"
    - "agent/state/TMP-021.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/postback-dispatcher/**"
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

As a verification agent, I can start postback-dispatcher against the local compose database, so the full compose smoke can verify dispatcher startup separately from postback schema provisioning and external delivery.

## Acceptance Criteria

- `postback-dispatcher` compose env includes `DB_POSTGRESQL_*` names consumed by `common/config`.
- Compose config renders with `.env.example`.
- Targeted postback-dispatcher smoke starts database and dispatcher; dispatcher connects to DB.
- No postback source, common source, dependency metadata, vendor, frontend, package manifest, lockfile, credential, or schema files are changed.
