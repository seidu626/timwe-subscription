# TMP-034 Decision Template: Acquisition Runtime Schema Provisioning

Status: proposed

Approval recorded: no

## Context

The acquisition API compose image builds, but the runtime exits during admin schema bootstrap because `add_admin_management_tables.sql` expects `products`/`userbase` in the empty compose database.

`services/pg_schema.sql` defines these base tables, but it is hand-maintained DDL, not a reviewed runtime provisioning path.

## Decision Required

Choose a canonical provisioning path before implementation:

- Reviewed compose/runtime migration runner.
- Documented operator runbook plus verification command.
- Migration refactor that moves base tables into the canonical migration flow.

## Decision

Pending operator decision.

## Consequences To Review

- Migration ordering and rollback behavior.
- Ownership of base `products` and `userbase` schema.
- Whether hand-maintained `pg_schema.sql` remains authoritative.
- Effect on local compose, CI, and production-like environments.

## Post-Decision Proof

```bash
docker compose --env-file .env.example -f docker-compose.yml config
# targeted acquisition-api compose smoke with approved provisioning path
# verify acquisition-api reaches /health
```

## Slice Impact

- Blocks: `TMP-021`, `TMP-034`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-034.handoff.json`, `slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md`
