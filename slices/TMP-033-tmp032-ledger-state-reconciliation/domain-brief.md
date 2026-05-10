# TMP-033 Domain Brief

- Actor: platform-operator
- Business outcome: Supervisor and harness state agree that TMP-032 is done, so the full-system verifier does not see a false in-progress task.
- Domain invariant: control-plane task state must match accepted handoff, manifest, and value-gate evidence before release-readiness status can be trusted.
- Entrypoint: `.harness/task-ledger.sqlite` and `agent-supervisor list-tasks`
- Trigger: Verifier finds supervisor ledger state reporting `T-TMP-032` as running after TMP-032 evidence has been accepted.
- Risk: Repairing ledger state must not alter source, runtime, schema, dependency, compose, or package files.

## Story Craft

The story is concrete and testable: supervisor, agent-harness, manifest, and handoff checks all report TMP-032 as done, and `slice-harness sync --dry-run` reports no drift.

## Roadmap To Slices

TMP-033 is an operational control-plane reconciliation slice under TMP-021. It does not implement product behavior; it aligns task state with accepted evidence.
