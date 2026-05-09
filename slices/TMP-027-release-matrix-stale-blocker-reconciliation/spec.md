# TMP-027 Spec

## Outcome

Operator-inspectable release matrix accurately reflects current Go test and build pass evidence while keeping remaining real blockers visible.

## Acceptance

- `cd services/subscription-partner && go test ./...` passes.
- `cd services/notification && go test ./...` passes.
- `make build-all-local` passes.
- Generated binaries from the build are not committed.
- TMP-021 value gate and full-system matrix retire the stale notification/subscription-partner dependency/vendor blocker.
- Current blockers remain visible.

## Non-Goals

- No service source changes.
- No dependency, vendor, lockfile, or package manifest changes.
- No compose runtime start without env/secret decisions.
- No webspa-admin submodule pointer or source changes.
- No local primary branch merge/reset.
