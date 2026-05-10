# TMP-041 Value Gate Report

- Timestamp: 2026-05-09T06:05:49Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:done

## Domain Grounding

- Actor: platform-operator.
- Business outcome: runtime schema blockers are narrowed to exact source files and an approval-gated provisioning decision.
- Domain invariant: having SQL definitions in the repo does not mean the compose runtime database is provisioned.
- Entrypoint: TMP-034, TMP-035, TMP-036, and the full-system release matrix.
- Risk: applying SQL or compose changes without approval can hide ordering defects or mutate runtime state.

## Story Craft

As a platform operator, I can see the difference between missing schema definitions and missing approved migration orchestration, so the next implementation slice has a concrete decision point.

## Source Inventory

| Blocker | Runtime symptom | Existing SQL source | Notes |
|---|---|---|---|
| TMP-034 | acquisition-api exits because relation `products` is missing before `add_admin_management_tables.sql` runs | `services/pg_schema.sql` defines `userbase` and `products` | `pg_schema.sql` is hand-maintained DDL, not a numbered migration; it also contains a duplicate `listResponse` declaration. |
| TMP-035 | notification-worker starts, then dispatcher logs missing `message_outbox` | `services/subscription-external/migrations/011_message_cadence_engine.sql` defines `message_outbox` | Migration 011 also defines `update_updated_at_column()`, so function ordering should be reviewed if bundled with other migrations. |
| TMP-036 | postback-dispatcher starts, then polling logs missing `postback_outbox` | `services/acquisition-api/migrations/create_postback_tables.sql` and `services/subscription-external/migrations/006_web_acquisition_campaigns.sql` both define `postback_outbox` | Duplicate table definitions mean the canonical runtime provisioning path still needs an explicit decision. |

## Acceptance Results

| Criterion | Result | Evidence |
|---|---|---|
| Products/userbase source identified | PASS | `services/pg_schema.sql` contains `CREATE TABLE userbase` and `CREATE TABLE IF NOT EXISTS products`. |
| Message outbox source identified | PASS | `services/subscription-external/migrations/011_message_cadence_engine.sql` contains `CREATE TABLE IF NOT EXISTS message_outbox`. |
| Postback outbox source identified | PASS with risk | `services/acquisition-api/migrations/create_postback_tables.sql` and `services/subscription-external/migrations/006_web_acquisition_campaigns.sql` both contain `CREATE TABLE IF NOT EXISTS postback_outbox`. |
| Remaining blocker preserved | PASS | TMP-034, TMP-035, and TMP-036 remain blocked until approved migration provisioning/orchestration exists for the compose runtime. |
| No runtime/schema/source changes | PASS | This slice changes metadata and evidence only; `services/**`, `*.sql`, `docker-compose*.yml`, `Makefile`, dependency, package, and runtime files are forbidden. |

## Remaining Gate

The next executable implementation requires an approved canonical migration provisioning artifact for the compose runtime, such as a reviewed migration runner, compose init path, or operator runbook. This slice does not apply SQL and does not choose between duplicate postback schema sources.

## Commands

```bash
rg -n "CREATE TABLE.*products|CREATE TABLE.*userbase|CREATE TABLE.*message_outbox|CREATE TABLE.*postback_outbox" services/pg_schema.sql services/subscription-external/migrations/006_web_acquisition_campaigns.sql services/subscription-external/migrations/011_message_cadence_engine.sql services/acquisition-api/migrations/create_postback_tables.sql
jq empty slices/manifest.json agent/state/TMP-041.work-order.json agent/state/TMP-041.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
agent-supervisor preflight with worktree-local temp config
agent-supervisor auto-loop --max-rounds 1 with worktree-local temp config
git diff --check
git diff --name-only file-scope review
```

Result: PASS for source inventory and blocker narrowing; release readiness remains BLOCKED until migration provisioning/orchestration is approved and verified.
