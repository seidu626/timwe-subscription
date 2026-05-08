# TMP-004 Value Gate Report

Verdict: PASS

## Slice

Tenant admin binds secure credential references to a tenant channel without exposing secret material.

## Review Gates Applied

- Claude critique applied: purpose-scoped active credential uniqueness, HMAC tenant-scoped fingerprinting, partial active unique index, no metadata JSON, inactive-channel rejection, 404 cross-tenant behavior, and redacted audit payloads.
- Parallel schema/repository review applied: composite tenant/channel FK, rotation transaction, active versioning, schema registry update, and reference-only persistence.
- Parallel handler/service review applied: existing tenant resolver, strict JSON decode, 503 dependency unavailable for raw secrets without backend, and redacted responses.

## Acceptance Coverage

- Bind credential reference: implemented by `POST /v1/admin/channels/{id}/credentials`; returns credential id, version, status, purpose, and redacted display only.
- Rotate credential: repository transaction deactivates old active credential and inserts the next active version for the same tenant/channel/purpose.
- Secret backend unavailable: raw `secret_value` with no configured adapter returns 503 and creates no credential row.
- Cross-tenant access: channel lookup and credential queries include tenant_id and channel_id.
- Credential value read attempt: domain response uses `json:"-"` for `secret_ref` and fingerprint; handler test asserts raw submitted secret is not echoed.
- Inactive channel: service rejects before calling the secret adapter.
- Admin audit: rotation writes `tenant_channel_credential` activity log using redacted metadata only.

## Verification

Commands run:

```bash
cd services/acquisition-api && go test ./internal/repository ./internal/service ./internal/handler ./internal/transport
cd services/acquisition-api && go test ./...
git diff --check
```

Results:

- Focused acquisition-api packages passed.
- Full acquisition-api suite passed.
- Diff whitespace check passed.

## Test Quality

The repository does not contain `scripts/scan-test-quality.sh`; manual value-gate checks were applied against TMP-004 tests. The new tests include negative paths for inactive channel, missing backend, migration plaintext-column denial, transactional rotation, and response redaction.

## Gaps Deferred

- Production vault adapter and delete/cleanup semantics are deferred to TMP-015.
- Explicit credential deactivation endpoint is not included because TMP-004 acceptance only requires bind/list/rotate.
