# Tenant Channel Onboarding Contract Pack

Contract version: `tenant-channel-v1.0.0`

This pack gives API-integrated partners the stable tenant/channel contract for opt-in, confirmation, mobile-terminated messaging, charging, callbacks, and conversion postbacks. It is documentation-only and does not provision live credentials.

## Identity Model

Every partner request must carry both tenant and channel identity.

| Field | Required | Example | Notes |
| --- | --- | --- | --- |
| `tenant_key` | Yes | `nrg` | Stable tenant slug assigned during onboarding. |
| `channel_key` | Yes | `web-gh-mobplus` | Stable channel slug under the tenant. |
| `partner_key` | Yes | `mobplus` | Partner identifier used for audit and routing. |
| `capability` | Yes | `optin`, `confirm`, `mt`, `charge`, `callback`, `postback` | Must be enabled for the tenant channel before use. |
| `idempotency_key` | Mutations only | `tenant-channel-v1.0.0:optin:partner-click-001` | Stable for retries of the same logical action. |

Tenant/channel identity must come from the trusted onboarding contract or gateway/auth context. A request that can succeed without tenant/channel context is invalid.

## API Shape

The public contract is versioned by document version and by the `/v1` URL segment. Breaking changes require a new document version and fixture set.

| Operation | Method and Path | Required Capability | Idempotency |
| --- | --- | --- | --- |
| Opt-in | `POST /api/external/v1/{tenant_key}/{channel_key}/subscriptions/optin` | `optin` | Required |
| Confirm | `POST /api/external/v1/{tenant_key}/{channel_key}/subscriptions/confirm` | `confirm` | Required |
| Mobile terminated message | `POST /api/external/v1/{tenant_key}/{channel_key}/mt` | `mt` | Required |
| Charge | `POST /api/external/v1/{tenant_key}/{channel_key}/charges` | `charge` | Required |
| Callback receive | `POST /api/external/v1/{tenant_key}/{channel_key}/callbacks/{event_type}` | `callback` | Event id required |
| Conversion postback delivery | Partner-defined HTTPS URL from channel config | `postback` | `postback_id` required |

Recommended request headers:

```http
Authorization: Bearer <partner-access-token>
Content-Type: application/json
Idempotency-Key: tenant-channel-v1.0.0:optin:partner-click-001
X-Timwe-Contract-Version: tenant-channel-v1.0.0
```

## Credential Exchange

Credentials are exchanged out of band through the operator-approved secret channel for the tenant. Do not send credentials in tickets, examples, or screenshots.

The onboarding record must include:

- `tenant_key`
- `channel_key`
- `partner_key`
- enabled capabilities
- sandbox base URL
- callback shared secret reference
- postback target URL template
- credential rotation owner

Credential values in documentation must be placeholders such as `<partner-access-token>` or `<callback-shared-secret-ref>`.

## Callback Signing

Callbacks use HMAC-SHA256 with a timestamped canonical payload.

Required headers:

```http
X-Timwe-Timestamp: 2026-05-08T10:00:00Z
X-Timwe-Event-Id: evt_01JXAMPLE000000000000000
X-Timwe-Signature: sha256=<hex-hmac>
X-Timwe-Contract-Version: tenant-channel-v1.0.0
```

Signature input:

```text
<timestamp>.<event_id>.<raw_request_body>
```

Validation rules:

- reject missing signature, timestamp, or event id with `SIGNATURE_REQUIRED`
- reject timestamps outside a five-minute clock-skew window with `SIGNATURE_EXPIRED`
- reject HMAC mismatch with `SIGNATURE_INVALID`
- treat duplicate `event_id` as idempotent replay and return the original outcome

## Retry and Idempotency

Partners may retry network failures with the same `Idempotency-Key`.

| Case | Expected Result |
| --- | --- |
| Same key and same body | Return original result. |
| Same key and different body | Reject with `IDEMPOTENCY_CONFLICT`. |
| Missing key on mutation | Reject with `IDEMPOTENCY_KEY_REQUIRED`. |
| Duplicate callback event id | Return original callback outcome. |

## Errors

| Code | HTTP Status | Meaning |
| --- | --- | --- |
| `TENANT_CHANNEL_REQUIRED` | 400 | Missing tenant or channel identity. |
| `TENANT_CHANNEL_NOT_FOUND` | 404 | Tenant/channel pair is unknown or disabled. |
| `CAPABILITY_NOT_ENABLED` | 409 | Requested capability is not enabled for the channel. |
| `SIGNATURE_REQUIRED` | 401 | Callback signature headers are missing. |
| `SIGNATURE_INVALID` | 401 | Callback HMAC does not match the canonical payload. |
| `IDEMPOTENCY_KEY_REQUIRED` | 400 | Mutation request omitted the idempotency key. |
| `IDEMPOTENCY_CONFLICT` | 409 | Idempotency key was reused with a different body. |

## Postback Contract

Postbacks are delivered from the platform to the partner URL configured on the tenant channel. The payload must include tenant/channel identity and a stable postback id.

```json
{
  "contract_version": "tenant-channel-v1.0.0",
  "tenant_key": "nrg",
  "channel_key": "web-gh-mobplus",
  "partner_key": "mobplus",
  "postback_id": "pb_01JXAMPLE000000000000000",
  "event_type": "conversion.charged",
  "click_id": "partner-click-001",
  "msisdn_hash": "sha256:example-redacted",
  "amount": "1.00",
  "currency": "GHS",
  "occurred_at": "2026-05-08T10:00:00Z"
}
```

Do not include raw MSISDN values in postback examples.

## TIMWE Field Mapping

| Provider Field | Tenant/Channel Contract Field | Posture |
| --- | --- | --- |
| `realm` | `tenant_key` | Must resolve to the configured tenant key. |
| `channel` or `entry_channel` | `channel_key` | Must resolve to the configured tenant channel key. |
| `partnerRole` | `partner_key` | Mapped during onboarding. |
| `txid`, `tracker`, or `click_id` | `click_id` | Preserve original partner value. |
| unsigned callback | signed callback headers | Rejected; sandbox must reject missing signatures. |

Ambiguous provider mappings must fail closed. Operators must choose a single tenant/channel mapping before enabling production traffic.

## Sandbox Fixtures

Fixtures live in `examples/tenant-channel-onboarding/contract-fixtures.json`.

Run the local fixture smoke:

```bash
examples/tenant-channel-onboarding/validate-fixtures.sh
```

The fixture bundle includes supported opt-in, charge, callback, and postback examples plus negative examples for missing callback signatures and unsupported charge capability.

## Tenant / Channel Key Resolver Precedence

The canonical resolver lives in `common/auth/tenantctx.ResolveKeyPair`. All service handlers must call it; direct inline parsing of `X-Tenant-Key` / `X-Channel-Key` is not permitted.

### Four-rule contract (evaluated in order)

| # | Header present | Query present | Gateway trust | Outcome |
|---|---|---|---|---|
| 1 | Yes | No | Any | Header value accepted. |
| 2 | Yes | Yes, same value | Any | Header value accepted (agreement). |
| 3 | Yes | Yes, different value | Any | **Refused** — `ErrTenantKeyConflict` (HTTP 409). Error message names the conflicting header and both values. |
| 4 | No | Yes | **Verified** | Query value accepted. |
| — | No | Yes | Not verified | **Refused** — query-only resolution without gateway trust is rejected. |

### Case sensitivity

Values are normalised to lowercase before comparison. `Careerify`, `CAREERIFY`, and `careerify` are treated as the same tenant key.

### Gateway trust boundary

"Gateway trusted" means the request carries a valid HMAC-signed service context as defined in `common/auth/tenantctx.IdentityFromTrustedRequest` (headers `X-Service-Id`, `X-Service-Timestamp`, `X-Service-Signature`). KrakenD signs every forwarded request with the shared `AUTH.JWT_TOKEN.SECRET`. Handlers set `GatewayTrusted: true` after `IdentityFromTrustedRequest` succeeds; they never set it unconditionally.

### Error envelope

Conflict errors are returned through the existing `writeError` envelope:

```json
{
  "code": "TENANT_CONTEXT_REQUIRED",
  "message": "tenant key conflict: header and query parameter disagree: X-Tenant-Key header=\"careerify\" query=\"other-tenant\"",
  "inError": true,
  "responseData": {}
}
```

HTTP status is **409 Conflict**.

## Notification Handler Tenant Enforcement (TMP-071)

The notification handler at `services/notification/internal/handler/http.go` enforces tenant resolution before persisting any inbound MNO callback. Resolution runs in `resolveNotificationTenant` and produces one of three outcomes:

1. **No tenant context supplied** (legacy callers, no header / query / middleware identity) — handler proceeds with `TenantID = nil` and persists the row tenantless. This preserves backwards compatibility with partner callbacks that pre-date tenant scoping.
2. **Tenant context resolves cleanly** — handler attaches the canonical `tenant_id` UUID and, when `channel_key` is also supplied, resolves it to a `channel_id` UUID via `repository.ChannelIDByKeys`. The persisted notification carries both.
3. **Tenant context supplied but invalid** — handler returns a structured error envelope and one of the following statuses:

| Code | HTTP | Trigger |
|---|---|---|
| `TENANT_KEY_CONFLICT` | 409 | Header and query disagree (delegated to `tenantctx.ResolveKeyPair`, `ErrTenantKeyConflict`). |
| `UNKNOWN_TENANT` | 400 | `tenant_key` supplied but no row in `tenants` matches. |
| `CHANNEL_REQUIRED` | 400 | `tenant_key` supplied without a `channel_key`. |
| `UNKNOWN_CHANNEL` | 400 | `(tenant_id, channel_key)` pair has no row in `tenant_channels` (status filter applied). |

The rejection envelope matches the existing `writeError` shape:

```json
{
  "message": "unknown tenant_key",
  "code": "UNKNOWN_TENANT",
  "inError": "true"
}
```

### Migration impact for existing partners

Partners that have been sending `tenant_key` with no channel context, or with an unknown tenant slug, were previously silently accepted with HTTP 200 and stored with `tenant_id = NULL` / `channel_id = NULL`. They now receive HTTP 400. Operators onboarding a new tenant must:

1. Seed the row in `tenants` (e.g., careerify — see `ops/db/migrations/`).
2. Seed the corresponding `tenant_channels` row with `status = 'ACTIVE'`.
3. Coordinate with the partner so their submissions include both `tenant_key` and `channel_key`, or send no tenant context at all (legacy path).

The cross-tenant refusal smoke (`scripts/smoke/careerify-tenant-cross-tenant-refusal.sh`) is the regression gate for this behavior: Cases B and C must return 4xx, Case A must return 409.

## KrakenD FC Template Layout (TMP-073)

The container built from `krakend/Dockerfile` runs `krakend run -dc /etc/krakend/config/krakend.tmpl`. The static `krakend/krakend.json` reference is **not** what the runtime serves — the runtime renders config at startup from:

| Path | Role |
| --- | --- |
| `krakend/config/krakend.tmpl` | Top-level shell. References `service.json` settings and includes the `Endpoint` template. |
| `krakend/config/settings/service.json` | Service-wide values: port, timeouts, upstream URLs, CORS, TLS, telemetry. |
| `krakend/config/templates/Endpoint.tmpl` | The endpoint list — invokes per-endpoint partials. |
| `krakend/config/templates/TenantApiEndpoint.tmpl` | **Tenant-aware partials**. `TenantApiEndpoint` forwards notification `tenant_key`/`channel_key` query params through `input_query_strings`. `TenantPathApiEndpoint` rewrites external `/api/external/v1/{tenant_key}/{channel_key}/subscriptions/{op}` path captures into backend `tenant_key`/`channel_key` query params. |
| `krakend/config/templates/TimweApiEndpoint.tmpl` | Legacy (non tenant-aware) partial. Still used for `/api/v1/notification/list`, internal `/admin/{op}` admin routes, and non-tenant subscription paths. |

When onboarding a new tenant, update **both** the static reference (`krakend/krakend.json`) and the FC template (`krakend/config/templates/Endpoint.tmpl`) — they must agree, or smoke tests against `docker-compose up krakend` will diverge from the static-config smoke. Verify with:

```bash
docker run --rm -v "$PWD/krakend:/etc/krakend" \
  -e FC_ENABLE=1 \
  -e FC_SETTINGS=/etc/krakend/config/settings \
  -e FC_PARTIALS=/etc/krakend/config/partials \
  -e FC_TEMPLATES=/etc/krakend/config/templates \
  -e FC_OUT=/tmp/rendered.json \
  docker.io/library/krakend:latest \
  check -dc /etc/krakend/config/krakend.tmpl
```

The check is `Syntax OK!` and the rendered JSON contains the expected endpoint set with tenant query forwarding and tenant-path backend query rewrites.

## Operator Smoke Matrix

Two scripts in `scripts/smoke/` cover the careerify/web-gh-airteltigo tenant end-to-end.
Full evidence and run instructions are in
`slices/TMP-070-careerify-tenant-e2e-smoke/value-gate-report.md`.

### Happy-path (10 URLs)

```bash
# Defaults: HOST=http://127.0.0.1:8080  TENANT_KEY=careerify  CHANNEL_KEY=web-gh-airteltigo
HOST=https://staging-gw.example.com ./scripts/smoke/careerify-tenant-e2e.sh
```

Exercises 6 notification endpoints (`/api/v1/notification/…`) and 4 subscription endpoints
(`/api/external/v1/careerify/web-gh-airteltigo/subscriptions/…`).  
Asserts HTTP 2xx on all 10. Prints a final 10-row PASS/FAIL matrix. Exit 0 = gate passes.

### Adversarial / cross-tenant refusal (3 cases)

```bash
HOST=https://staging-gw.example.com ./scripts/smoke/careerify-tenant-cross-tenant-refusal.sh
```

| Case | What it probes | Expected |
|------|---------------|----------|
| A — conflict | `X-Tenant-Key: careerify` header + `?tenant_key=other-tenant` query | HTTP 409 |
| B — foreign tenant | `tenant_key=evil-tenant` (unknown tenant) | HTTP 4xx |
| C — missing channel | `tenant_key=careerify` only, no `channel_key` | HTTP 4xx |

PASS means the server **rejected** the request. A 2xx is a FAIL (tenant-scoping gap).  
Targets commits: TMP-066=77f9359, TMP-067=7e10692, TMP-068=3027c86, TMP-069=3897e89.
