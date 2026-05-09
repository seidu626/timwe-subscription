# TMP-027 Notes

## Domain Grounding

- Actor: platform-operator.
- Business outcome: release-readiness status is based on current verification evidence, not stale failure rows.
- Domain invariant: removing resolved blockers must not hide still-current runtime, submodule, security, or integration blockers.
- Entrypoint: `docs/agent/full-system-verification-2026-05-09.md`.
- Risk: `make build-all-local` writes local binaries; generated binary changes must not be committed.

## Story Craft

The story is concrete and testable: current Go service tests and canonical local build pass, so the operator should see those checks as passed while unresolved gates remain blocked.

## Value Gate

Pass criteria:
- `services/subscription-partner` default tests pass.
- `services/notification` default tests pass.
- `make build-all-local` passes.
- TMP-021 removes the stale dependency/vendor blocker and retains current blockers.
- No source, dependency, vendor, package manifest, lockfile, or frontend source files are changed.

## Claude Critique

Claude read the proposed operational slice before edits and agreed the class was appropriate. Guardrails applied here:

- Reframe the outcome as an operator-visible release matrix signal, not just blocker deletion.
- Rerun canonical commands in-slice on the branch being merged.
- Update the machine-readable TMP-021 handoff as well as the human-readable matrix and value gate.
- Keep TMP-021 status blocked while webspa-admin, compose runtime, dependency vulnerability, and local-main divergence gates remain unresolved.

Claude also warned not to touch source, dependencies, vendor trees, package manifests, lockfiles, or frontend source. The manifest is touched only because the harness requires TMP-027 state registration.
