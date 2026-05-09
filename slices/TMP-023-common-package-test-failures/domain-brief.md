# TMP-023 Domain Brief

## Actors

- Platform operator: runs shared library verification before release.
- Service implementer: depends on common tenant auth and database helpers from downstream services.

## Ubiquitous Language

- Trusted service headers: signed internal headers carrying tenant/service context.
- Nonce store: replay-protection store for trusted service requests.
- PGX pool: PostgreSQL connection pool helper used by services.

## Domain Invariants

- Trusted service requests must reject replayed nonces.
- Database pool tests must match the current constructor interface.
- Tooling helpers must not break normal package builds.

## Failure Modes

- Generator API drift prevents the root common package from compiling.
- Tests call an older helper signature and fail at compile time.
- Replay nonce tests use a stale clock and accept duplicate nonce use.

## User Journey

1. Platform operator runs `cd common && go test ./...`.
2. Common auth, config, postgres, and helper packages compile and test.
3. Downstream service verification can trust shared common behavior.

## Open Questions

- `openApiGenerator.go` appears to be a tool helper rather than runtime library code; this slice excludes it from normal package builds rather than upgrading generator APIs.
