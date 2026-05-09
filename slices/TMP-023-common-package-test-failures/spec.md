# TMP-023 Spec

## Objective

Make `cd common && go test ./...` pass without changing dependencies, vendor contents, or downstream service behavior.

## Broken Behavior

- `common/openApiGenerator.go` fails to compile against the current gnostic generator API.
- `common/postgres/database_test.go` calls `NewPGXPool` without the current `*DatabaseConfig` argument.
- `TestMiddlewareRejectsReplayNonce` accepts a replayed nonce because the in-memory nonce store uses wall-clock time while the verifier uses a fixed test clock.

## Expected Behavior

- Normal common package tests do not compile tool-only generator helpers.
- Postgres tests call the current helper interface.
- Replay nonce test deterministically rejects the second request.

## Acceptance Proof

```bash
cd common && go test ./...
```
