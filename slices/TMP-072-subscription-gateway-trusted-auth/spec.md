# TMP-072 — subscription-gateway-trusted-auth

## User story
As an external subscription partner, my `POST` to `/api/external/v1/{tenant_key}/{channel_key}/subscriptions/{op}` reaches the subscription-external service and is processed under the correct tenant context, instead of being rejected at the gateway boundary with `trusted service secret is not configured`.

## Background
TMP-068 routed the 4 subscription endpoints through KrakenD to the `subscription-external` **admin** handler `partner_handler.go::tenantRouteFromRequest` (line 261). That handler requires:

1. `cfg.Auth.JwtToken.Secret` to be configured (otherwise: HTTP 400 `trusted service secret is not configured`), and
2. an HMAC-signed `Identity` header validated by `tenantctx.IdentityFromTrustedRequest`.

KrakenD only injects raw `X-Tenant-Key` / `X-Channel-Key` from path captures (via the martian `header.Modifier` block). It does NOT mint a trusted-service identity token, so the gateway-routed request fails the trusted-auth gate before any tenant-resolution logic runs.

TMP-070 smoke confirmed: 4/4 subscription endpoints return `HTTP 400 trusted service secret is not configured`, and the cross-tenant conflict refusal (Case A → expect 409) never reaches the resolver.

Evidence: `slices/TMP-070-careerify-tenant-e2e-smoke/value-gate-report.md` "Verdict and ownership of gaps" row 4.

## Two acceptable approaches (pick one in spec phase)

### Option A — KrakenD mints the trusted-service header
KrakenD per-backend `extra_config` runs a Lua/martian script (or a custom KrakenD plugin) that, on each subscription request, signs an `X-Trusted-Identity` HMAC token using a shared secret with `subscription-external`. The token carries `tenant_key`, `channel_key`, issued-at, nonce. `partner_handler.go::tenantRouteFromRequest` validates exactly as it does today; no Go code change.

- Pros: zero Go change; matches the existing trust model used for internal trusted-service calls.
- Cons: Requires deploying secret material into KrakenD and rotating it alongside the backend; martian/Lua signing logic has to handle skew + nonce.

### Option B — Route subscription endpoints to non-trusted external handlers
Replace the `url_pattern` rewrite from `/api/v1/subscription-external/admin/{op}` to `/api/v1/subscription-external/partners/{op}` (or equivalent external-facing partner route that does NOT require a signed trusted-service identity). The external handler enforces tenant context strictly via `tenantctx.ResolveKeyPair` + DB lookup, mirroring how the `partner` external endpoints already work.

- Pros: No secret distribution; aligns with how legacy partner endpoints are exposed today.
- Cons: Requires either adding new external partner handlers (if they don't exist for `optin/confirm/optout/status`) or refactoring; broader change surface in subscription-external Go code.

The spec phase MUST pick one and document why.

## Scope (final once an option is picked)
- `krakend/krakend.json` (and `krakend/config/templates/Endpoint.tmpl` if TMP-073 lands first or merges).
- One of: shared secret plumbing into KrakenD config + nonce store (Option A) OR new/refactored partner handlers in `services/subscription-external/internal/handler/` (Option B).
- `scripts/smoke/careerify-tenant-e2e.sh` — subscription cases must reach `200` and the cross-tenant refusal Case A must return `409` (not `400`).

## Out of scope
- Notification endpoints (TMP-071).
- FC template parity (TMP-073).

## Acceptance criteria
- `POST /api/external/v1/careerify/web-gh-airteltigo/subscriptions/{optin|confirm|optout|status}` returns `2xx` through the gateway against the careerify seed.
- Cross-tenant refusal Case A (`X-Tenant-Key: careerify` + `?tenant_key=other-tenant`) returns `409` with the structured `TENANT_KEY_CONFLICT` envelope from `common/auth/tenantctx/resolver.go`.
- `subscription-external` audit logs show `tenant_id = <careerify uuid>` and `channel_id = <web-gh-airteltigo uuid>` for each accepted request.
- Existing internal admin callers (notification → subscription-external admin) keep working — verify by re-running `services/subscription-external` test suite end-to-end.

## Dependencies
- TMP-066, TMP-068 (shipped).
- TMP-069 (shipped) — resolver returns the 409 once reached.

## Risk
Medium. Touches authentication for an externally-facing route. Spec phase must document threat model: token replay resistance, secret rotation path, audit logging.

## Verification
```
bash scripts/smoke/careerify-tenant-e2e.sh                       # 10/10 PASS expected
bash scripts/smoke/careerify-tenant-cross-tenant-refusal.sh      # 3/3 PASS expected
go test ./services/subscription-external/...
```
