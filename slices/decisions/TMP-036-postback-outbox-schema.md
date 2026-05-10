# TMP-036 Decision Template: Postback Outbox Schema

Status: accepted

Approval recorded: yes - auto-approved by operator directive on 2026-05-10.

## Context

The postback dispatcher now starts and connects to the compose database, but polling logs `postback_outbox` missing.

Two SQL sources define `postback_outbox`: `services/acquisition-api/migrations/create_postback_tables.sql` and `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`. This duplicate ownership must be resolved before automation.

## Decision Required

Choose canonical schema ownership and migration order before implementation:

- Acquisition API owns `postback_outbox`.
- Subscription external owns `postback_outbox`.
- A shared migration/provisioning path owns the table and both services consume it.

## Decision

Acquisition API owns `postback_outbox` and `postback_attempts` for this runtime path. The compose bootstrap applies `services/acquisition-api/migrations/create_postback_tables.sql` followed by `services/acquisition-api/migrations/add_tenant_postback_routing.sql`; it does not apply the duplicate postback definitions from `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`.

## Consequences To Review

- Duplicate table definition cleanup or compatibility.
- Which service owns outbox lifecycle, retries, and failure semantics.
- Migration ordering across acquisition and subscription services.
- Runtime behavior for existing local/CI databases.

Reviewed outcome: `TMP-045` implements and verifies this for local compose/runtime verification only. The subscription-external `006` duplicate remains legacy/compat material to prune or split later.

## Post-Decision Proof

```bash
docker compose --env-file .env.example -f docker-compose.yml config
# targeted postback-dispatcher compose smoke with approved provisioning path
# verify no postback_outbox missing-relation logs
```

Implemented proof: `slices/TMP-045-compose-runtime-schema-bootstrap/value-gate-report.md`.

## Slice Impact

- Blocks: `TMP-021`, `TMP-036`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-036.handoff.json`, `slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md`
