---
id: TMP-045
title: "Compose runtime schema bootstrap"
class: vertical_defect_slice
status: done
scope_limit: "Provision clean-compose database prerequisites for acquisition-api, notification-worker, cadence-engine, and postback-dispatcher using one compose-owned bootstrap module. Do not introduce duplicate postback/message-outbox schemas or service-local self-migration paths."
merge_policy: "Merge only after clean PostgreSQL bootstrap proof, worker empty-poll query proof, compose config render, targeted service tests, HVC, supervisor preflight, JSON validity, and value-gate evidence pass."
evidence_required:
  - "ops/db/bootstrap/001_runtime_base.sql"
  - "scripts/compose-db-bootstrap.sh"
  - "docker-compose.yml"
  - "slices/TMP-045-compose-runtime-schema-bootstrap/value-gate-report.md"
acceptance_tests:
  - "bash -n scripts/compose-db-bootstrap.sh"
  - "docker compose --env-file .env.example -f docker-compose.yml config --quiet"
  - "disposable PostgreSQL bootstrap applies ops/db/bootstrap/001_runtime_base.sql plus canonical service migrations"
  - "notification-worker, cadence-engine, and postback-dispatcher empty-poll query shapes run against the bootstrapped database"
  - "cd services/acquisition-api && go test ./internal/repository"
  - "cd services/notification && go test ./..."
  - "cd services/postback-dispatcher && go test ./..."
actor: platform-operator
outcome: "Clean compose database startup creates the base products/userbase, message_outbox, and postback_outbox prerequisites before runtime services begin polling or bootstrapping admin schema."
entrypoint: "docker compose up db-bootstrap acquisition-api notification-worker cadence-engine postback-dispatcher"
trigger: "Verifier starts the compose stack from an empty database after TMP-034/TMP-035/TMP-036 approvals are auto-granted."
broken_outcome: "Runtime services connect to an empty compose database and fail with missing products, userbase, message_outbox, or postback_outbox relations."
expected_behavior: "The db-bootstrap service applies the minimal base schema and canonical service-owned migrations, exits successfully, and dependent services start only after it completes."
reproduction:
  command: "targeted compose runtime probes captured by TMP-034, TMP-035, and TMP-036"
  observed: "acquisition-api failed while altering products/userbase; notification-worker logged relation message_outbox does not exist; postback-dispatcher logged relation postback_outbox does not exist."
  expected: "clean compose database provisioning applies required schema before runtime services start polling or bootstrapping admin schema."
system_path:
  - "PostgreSQL reaches healthy state."
  - "db-bootstrap applies cross-service prerequisites from ops/db/bootstrap/001_runtime_base.sql."
  - "db-bootstrap applies acquisition-api owned admin, tenant, and postback migrations."
  - "db-bootstrap applies subscription-external owned cadence/message_outbox migrations."
  - "Runtime services start after db-bootstrap completion and can poll empty work queues without schema errors."
change_layers:
  - compose
  - schema-bootstrap
  - evidence
verification_layers:
  - compose-config
  - postgres-smoke
  - service-tests
blocked_by: []
blocks:
  - "TMP-021"
  - "TMP-034"
  - "TMP-035"
  - "TMP-036"
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-045-compose-runtime-schema-bootstrap.md"
    - "agent/state/TMP-045.work-order.json"
    - "agent/state/TMP-045.handoff.json"
    - "docker-compose.yml"
    - "ops/db/bootstrap/**"
    - "scripts/compose-db-bootstrap.sh"
    - "slices/manifest.json"
    - "slices/TMP-045-compose-runtime-schema-bootstrap/**"
    - "slices/decisions/TMP-034-acquisition-runtime-schema-provisioning.md"
    - "slices/decisions/TMP-035-notification-message-outbox-schema.md"
    - "slices/decisions/TMP-036-postback-outbox-schema.md"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/internal/**"
    - "services/**/cmd/**"
    - "frontend/**"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
    - ".git/**"
---

## Operator Story

As a platform operator, I can start the local compose runtime from an empty database and have the schema prerequisites applied before workers poll, so release verification is blocked by real runtime failures rather than missing provisioning order.

## Acceptance Criteria

- Compose config renders and `db-bootstrap` is the completion prerequisite for acquisition-api, notification-worker, cadence-engine, postback-dispatcher, and subscription DB consumers.
- A disposable PostgreSQL proof applies the bootstrap SQL and ordered canonical migrations with `ON_ERROR_STOP=1`.
- Notification-worker, cadence-engine, and postback-dispatcher empty-poll query shapes return zero rows rather than schema errors.
- Acquisition, notification, and postback package tests for the touched runtime surfaces pass.
- Domain brief, story/spec, roadmap entry, and value-gate evidence are recorded.
