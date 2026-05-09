# TMP-036 Spec

## Objective

Classify and track the approval-gated postback_outbox schema blocker discovered after TMP-032. Do not change schema, migrations, runtime code, compose files, dependencies, or credentials in this slice.

## Broken Behavior

Postback dispatcher starts and connects to DB, then polling logs pq: relation postback_outbox does not exist against the empty compose DB.

## Expected Behavior

The compose DB provisioning path applies postback outbox schema before postback-dispatcher polling.

## Acceptance Proof

```bash
jq empty slices/manifest.json agent/state/TMP-036.work-order.json agent/state/TMP-036.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
git diff --name-only
```

## Approval Gate

- Schema/migration provisioning is approval-gated by repo risk boundaries.
- The postback_outbox ownership/provisioning path must be selected before implementation.
