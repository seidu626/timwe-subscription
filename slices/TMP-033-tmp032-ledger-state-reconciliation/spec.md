# TMP-033 Spec

## Objective

Reconcile the supervisor ledger state for T-TMP-032 with the already-accepted TMP-032 handoff, manifest, and agent task evidence.

## Broken Behavior

- The committed supervisor ledger reports `T-TMP-032|running`.
- `.agent/tasks.json`, `slices/manifest.json`, and `agent/state/TMP-032.handoff.json` report the TMP-032 postback dispatcher compose DB env slice as done.
- Full-system verification can therefore see a false in-progress task after the merged TMP-032 work has closed.

## Expected Behavior

- `.harness/task-ledger.sqlite` reports `T-TMP-032|done`.
- `agent-supervisor list-tasks` reports `T-TMP-032` as `done`.
- `agent-harness list` reports `T-TMP-032` as `done`.
- `slice-harness sync --dry-run` reports no drift.
- No source, runtime, dependency, schema, package, or compose files change.

## Acceptance Proof

```bash
sqlite3 .harness/task-ledger.sqlite "select id,status from tasks where id='T-TMP-032';"
tmp=$(mktemp); jq --arg repo "$PWD" '.repo_path=$repo' .harness/config.json > "$tmp"; agent-supervisor --config "$tmp" preflight; agent-supervisor --config "$tmp" list-tasks; rm -f "$tmp"
agent-harness list
slice-harness sync --dry-run
hvc check agent/backlog/issues/*.md --fail-on block
git diff --name-only
```
