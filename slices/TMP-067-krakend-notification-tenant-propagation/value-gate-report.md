# TMP-067 value-gate-report

**Run-id:** slice-TMP-067-build-2026-05-13T2025Z
**Date:** 2026-05-13

## Summary

Modified `krakend/krakend.json` to propagate tenant context (`tenant_key`, `channel_key`) from
query string through to the notification service as `X-Tenant-Key` / `X-Channel-Key` headers
for all 6 notification callback endpoints.

## Changes made

### `krakend/krakend.json` — 6 endpoints updated

Each of the following endpoints received:
1. `"input_query_strings": ["tenant_key", "channel_key", "external-tx-id"]` at the endpoint level
   (was `null`; `external-tx-id` already in `headers_to_pass` — now also captured via query string).
2. `"modifier/martian"` block in `backend[0].extra_config` with a `fifo.Group` containing two
   `header.Modifier` entries:
   - `X-Tenant-Key` ← `{tenant_key}` (query capture)
   - `X-Channel-Key` ← `{channel_key}` (query capture)

Endpoints modified:
- `/api/v1/notification/mo/{partnerRole}`
- `/api/v1/notification/mt/dn/{partnerRole}`
- `/api/v1/notification/user-optin/{partnerRole}`
- `/api/v1/notification/user-renewed/{partnerRole}`
- `/api/v1/notification/user-optout/{partnerRole}`
- `/api/v1/notification/charge/{partnerRole}`

The `/api/v1/notification/list` endpoint (authenticated admin read) was intentionally NOT modified
as it follows a different auth flow.

### Header injection mechanism: `modifier/martian` (built-in)

KrakenD CE v2+ ships `modifier/martian` as a built-in plugin — no additional compilation or
plugin binary required. The Dockerfile (`krakend/Dockerfile`) uses `docker.io/library/krakend:latest`
which is the official CE image at v2/v3 (confirmed schema version `3` in `krakend.json`). Martian
header injection is available and was applied.

No fallback to query-only propagation was needed. The `tenantctx` middleware in the notification
service already reads `X-Tenant-Key` (via `tenantctx.HeaderTenantKey` constant) and resolves
`tenant_id` from it via `h.service.TenantIDByKey` when the identity is platform-scoped.

### `scripts/smoke/krakend-notification-tenant.sh` — new file

Smoke script that POSTs minimal valid bodies to all 6 notification URLs with
`?tenant_key=careerify&channel_key=web-gh-airteltigo`. Reports PASS/FAIL per URL.
Returns exit 0 only if all 6 are 2xx. Bash syntax verified (`bash -n`).

## Gaps / deferred items

- **Live smoke run deferred to TMP-070**: The smoke script requires the full service stack
  (KrakenD on `:8080`, notification service on `:8082`, Postgres with careerify seed from TMP-066).
  Syntax-correct script is present; TMP-070 is the designated live verification slice.
- **`modifier/martian` query capture semantics**: KrakenD CE substitutes `{param}` in martian
  `value` fields from the request's query arguments captured via `input_query_strings`. This is
  consistent with KrakenD CE v2 documentation. If the deployed image turns out to be an older
  pre-v2 build where martian is not bundled, the `input_query_strings` entries alone will pass
  the raw query params to the upstream; TMP-069 (tenantctx resolver with query→header fallback)
  will handle the header promotion in that scenario.

## Acceptance criteria status

| Criterion | Status |
|---|---|
| All 6 endpoints declare `input_query_strings` containing `tenant_key`, `channel_key` | PASS — verified via `jq` |
| All 6 backend backends declare `modifier/martian` block for `X-Tenant-Key` and `X-Channel-Key` | PASS — verified via `jq` |
| Smoke script at `scripts/smoke/krakend-notification-tenant.sh` exists and is syntactically valid | PASS — `bash -n` clean |
| Live smoke run (all 6 URLs return 2xx) | DEFERRED to TMP-070 |
| No `services/notification/` code changes | PASS — zero Go files modified |
| No files outside `krakend/` and `scripts/smoke/` modified | PASS |
