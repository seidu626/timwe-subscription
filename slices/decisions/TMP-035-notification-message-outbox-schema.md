# TMP-035 Decision Template: Notification Message Outbox Schema

Status: proposed

Approval recorded: no

## Context

The notification worker now starts in compose, but its dispatcher logs `message_outbox` missing against the empty compose database.

`services/subscription-external/migrations/011_message_cadence_engine.sql` defines `message_outbox`, but the compose runtime does not currently prove that this migration is applied before worker polling.

## Decision Required

Choose how runtime provisioning applies the message outbox schema before notification worker startup:

- Reviewed compose/runtime migration runner.
- Documented operator runbook plus verification command.
- Service-local startup guard that fails clearly until the canonical migration path is run.

## Decision

Pending operator decision.

## Consequences To Review

- Whether subscription-external owns the message outbox schema.
- Ordering of migration 011 and its helper function.
- Worker startup behavior when schema is absent.
- Operational observability for migration failures.

## Post-Decision Proof

```bash
docker compose --env-file .env.example -f docker-compose.yml config
# targeted notification-worker compose smoke with approved provisioning path
# verify no message_outbox missing-relation logs
```

## Slice Impact

- Blocks: `TMP-021`, `TMP-035`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-035.handoff.json`, `slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md`
