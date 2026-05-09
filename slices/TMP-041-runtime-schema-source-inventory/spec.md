# TMP-041 Slice Spec

## Story

As a platform operator, I can inspect TMP-034, TMP-035, and TMP-036 and see the exact SQL sources for the missing runtime relations, duplicate-source hazards, and the remaining migration orchestration approval gate.

## Scope

Allowed:
- Add an evidence-only operational slice.
- Update TMP-034, TMP-035, TMP-036, and full-system verification evidence.
- Keep the blocked status of schema provisioning slices.

Forbidden:
- Editing `services/**`, `*.sql`, `docker-compose*.yml`, `Makefile`, dependencies, runtime code, credentials, or branch integration state.

## Acceptance Proof

- `services/pg_schema.sql` contains `userbase` and `products` definitions.
- `services/subscription-external/migrations/011_message_cadence_engine.sql` contains `message_outbox`.
- `services/acquisition-api/migrations/create_postback_tables.sql` and `services/subscription-external/migrations/006_web_acquisition_campaigns.sql` contain `postback_outbox`.
- Evidence records that duplicate/hand-maintained SQL sources require an approved canonical orchestration path.
- HVC, slice-harness, supervisor preflight, and JSON gates pass.

## Pass/Fail Criteria

Pass when the evidence narrows the blocker to approved migration provisioning/orchestration without changing runtime, schema, SQL, compose, or service files.

Fail if the slice claims the runtime schema is provisioned, changes SQL/compose/source files, or hides duplicate-source risk.
