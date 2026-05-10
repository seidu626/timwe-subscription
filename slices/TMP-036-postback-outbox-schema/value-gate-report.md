# TMP-036 Value Gate Report

- Timestamp: 2026-05-09T03:58:00Z
- Agent: Codex
- Verdict: BLOCKED
- Outcome code: outcome:blocked

## Audit 1: Blocker Classification

- Actor identified: COVERED by domain brief.
- Business outcome identified: COVERED by domain brief.
- Entrypoint identified: COVERED by issue and spec.
- Risk/approval gate identified: COVERED by issue, spec, and this report.

Audit 1 result: PASS for classification, BLOCKED for implementation.

## Audit 2: Scope Control

- No source/runtime/schema/dependency/compose/destructive git change in this slice: COVERED by final git diff review.
- Blocker remains visible as a blocked slice: COVERED by manifest and handoff once validated.

Audit 2 result: PASS for registry scope, BLOCKED for implementation.

## Blocking Gate

- Schema/migration provisioning is approval-gated by repo risk boundaries.
- `services/acquisition-api/migrations/create_postback_tables.sql` and `services/subscription-external/migrations/006_web_acquisition_campaigns.sql` both contain `postback_outbox` definitions.
- The duplicate definitions make canonical schema ownership and migration ordering explicit release decisions before automation.
- The postback_outbox ownership/provisioning path must be selected before implementation.

## Commands

```bash
jq empty slices/manifest.json agent/state/TMP-036.work-order.json agent/state/TMP-036.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
git diff --name-only
rg -n "CREATE TABLE.*postback_outbox" services/acquisition-api/migrations/create_postback_tables.sql services/subscription-external/migrations/006_web_acquisition_campaigns.sql
```

Result: BLOCKED by the gate above.
