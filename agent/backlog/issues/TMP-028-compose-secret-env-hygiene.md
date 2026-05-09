---
id: TMP-028
title: "Compose secret and env hygiene"
class: operational_slice
status: ready
scope_limit: "Remove checked-in subscription DB credential material from docker-compose.yml and provide safe local env scaffolding without starting runtime services."
merge_policy: "Merge only after compose config renders with the example env file, HVC, slice-harness, supervisor preflight, and source-scope checks pass."
evidence_required:
  - "docker compose --env-file .env.example -f docker-compose.yml config"
  - "rg -n 'APP_DATABASE_POSTGRESQL_HOST=139|APP_DATABASE_POSTGRESQL_PASSWORD=[^$]' docker-compose.yml || true"
  - "slices/TMP-028-compose-secret-env-hygiene/value-gate-report.md"
acceptance_tests:
  - "docker compose --env-file .env.example -f docker-compose.yml config"
  - "jq empty slices/manifest.json agent/state/TMP-028.work-order.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Local compose configuration no longer depends on checked-in subscription database credential material."
entrypoint: "docker-compose.yml"
trigger: "Operator renders or starts the local compose stack."
broken_outcome: "The subscription service in docker-compose.yml carries hardcoded database host/user/name and secret-shaped password material while other services use env inputs."
expected_behavior: "The subscription service uses environment inputs for credential-bearing fields, defaults host routing to the Docker database service, preserves literal SSL mode, and .env.example documents safe placeholders for local rendering."
system_path:
  - "Operator copies .env.example or supplies env variables."
  - "Docker Compose resolves subscription service DB settings from env."
  - "Compose config renders without checking credential values into source."
change_layers:
  - runtime-config
  - verification-evidence
verification_layers:
  - compose-config
  - harness
blocked_by: []
blocks: ["TMP-021"]
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - ".env.example"
    - "docker-compose.yml"
    - "docs/environment-variables.md"
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-028-compose-secret-env-hygiene/**"
    - "agent/backlog/issues/TMP-028-compose-secret-env-hygiene.md"
    - "agent/state/TMP-028.work-order.json"
    - "agent/state/TMP-028.handoff.json"
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

As a platform operator, I can render the local compose stack from environment inputs instead of repository-embedded subscription database credential material, so runtime configuration is safer to share and review.

## Acceptance Criteria

- `docker-compose.yml` subscription service uses env inputs for database credentials, optional service-native host/port overrides, Docker `database` routing by default, and literal SSL mode.
- A root `.env.example` exists with safe placeholder values for compose-required variables.
- `docker compose --env-file .env.example -f docker-compose.yml config` renders successfully.
- TMP-021 evidence no longer says a checked-in subscription DB credential remains, but still says runtime start requires real environment/provider values.
- No service source, frontend source, dependency, vendor, package manifest, or lockfile files are changed.
