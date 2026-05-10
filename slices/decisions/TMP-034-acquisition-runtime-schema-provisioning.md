# TMP-034 Decision Template: Acquisition Runtime Schema Provisioning

Status: accepted

Approval recorded: yes - auto-approved by operator directive on 2026-05-10.

## Context

The acquisition API compose image builds, but the runtime exits during admin schema bootstrap because `add_admin_management_tables.sql` expects `products`/`userbase` in the empty compose database.

`services/pg_schema.sql` defines these base tables, but it is hand-maintained DDL, not a reviewed runtime provisioning path.

## Decision Required

Choose a canonical provisioning path before implementation:

- Reviewed compose/runtime migration runner.
- Documented operator runbook plus verification command.
- Migration refactor that moves base tables into the canonical migration flow.

## Decision

Use a reviewed compose/runtime bootstrap path for local full-system verification. The bootstrap path creates only the cross-service prerequisites from `ops/db/bootstrap/001_runtime_base.sql`, then lets the existing acquisition-api migrations alter `products` and `userbase` during admin/tenant schema provisioning.

## Consequences To Review

- Migration ordering and rollback behavior.
- Ownership of base `products` and `userbase` schema.
- Whether hand-maintained `pg_schema.sql` remains authoritative.
- Effect on local compose, CI, and production-like environments.

Reviewed outcome: `TMP-045` implements and verifies this for local compose/runtime verification only. Production migration ownership remains outside this decision.

## Post-Decision Proof

```bash
docker compose --env-file .env.example -f docker-compose.yml config
# targeted acquisition-api compose smoke with approved provisioning path
# verify acquisition-api reaches /health
```

Implemented proof: `slices/TMP-045-compose-runtime-schema-bootstrap/value-gate-report.md`.

## Slice Impact

- Blocks: `TMP-021`, `TMP-034`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-034.handoff.json`, `slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md`
