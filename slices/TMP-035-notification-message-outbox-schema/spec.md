# TMP-035 Spec

## Objective

Classify and track the approval-gated notification message_outbox schema blocker discovered after TMP-031. Do not change schema, migrations, runtime code, compose files, dependencies, or credentials in this slice.

## Broken Behavior

Notification worker starts and exposes metrics, then dispatcher logs pq: relation message_outbox does not exist against the empty compose DB.

## Expected Behavior

The compose DB provisioning path applies the message cadence/outbox schema before notification-worker dispatch polling.

## Acceptance Proof

```bash
jq empty slices/manifest.json agent/state/TMP-035.work-order.json agent/state/TMP-035.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
git diff --name-only
```

## Approval Gate

- Schema/migration provisioning is approval-gated by repo risk boundaries.
- The message_outbox ownership/provisioning path must be selected before implementation.
