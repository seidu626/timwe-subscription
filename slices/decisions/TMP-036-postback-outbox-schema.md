# TMP-036 Decision Template: Postback Outbox Schema

Status: proposed

Approval recorded: no

## Context

The postback dispatcher now starts and connects to the compose database, but polling logs `postback_outbox` missing.

Two SQL sources define `postback_outbox`: `services/acquisition-api/migrations/create_postback_tables.sql` and `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`. This duplicate ownership must be resolved before automation.

## Decision Required

Choose canonical schema ownership and migration order before implementation:

- Acquisition API owns `postback_outbox`.
- Subscription external owns `postback_outbox`.
- A shared migration/provisioning path owns the table and both services consume it.

## Decision

Pending operator decision.

## Consequences To Review

- Duplicate table definition cleanup or compatibility.
- Which service owns outbox lifecycle, retries, and failure semantics.
- Migration ordering across acquisition and subscription services.
- Runtime behavior for existing local/CI databases.

## Post-Decision Proof

```bash
docker compose --env-file .env.example -f docker-compose.yml config
# targeted postback-dispatcher compose smoke with approved provisioning path
# verify no postback_outbox missing-relation logs
```

## Slice Impact

- Blocks: `TMP-021`, `TMP-036`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-036.handoff.json`, `slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md`
