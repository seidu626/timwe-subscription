# TMP-038 Spec

## Objective

Classify and track the local-main integration decision blocker. Do not merge, reset, delete branches, or resolve conflicts in this slice.

## Broken Behavior

Primary local main is currently ahead 51 and behind origin/main by 38 as of the 2026-05-09T08:44:16Z evidence refresh, and an isolated merge probe produced broad add/add conflicts.

## Expected Behavior

A maintainer chooses whether to preserve local-only history, reset to remote, or manually integrate the divergent histories before treating primary main as verified.

## Acceptance Proof

```bash
jq empty slices/manifest.json agent/state/TMP-038.work-order.json agent/state/TMP-038.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
git diff --name-only
```

## Approval Gate

- Destructive or broad conflict-resolution branch operations require explicit maintainer direction.
- Primary main contains local-only history that must not be discarded by an agent.
