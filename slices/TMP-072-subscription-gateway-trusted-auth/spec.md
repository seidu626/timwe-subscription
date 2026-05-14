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

## Decision (locked 2026-05-14)

**Option B — partner handlers without trusted-service auth.**

Rationale:
- These endpoints are externally-facing partner routes. Sharing the internal trusted-service signing secret with KrakenD widens the blast radius of that secret (KrakenD already terminates partner TLS — adding secret-signing duty to it concentrates trust in one box).
- The existing partner endpoints (`/api/external/v1/subscription/*`) already pre-date the trusted-service gate and would benefit from a partner-auth pattern that resolves tenant context from headers KrakenD injects (path captures → `X-Tenant-Key` / `X-Channel-Key`).
- Path-based tenant capture (`/{tenant_key}/{channel_key}/...`) IS itself a form of tenant context: the URL is signed by virtue of having been routed through nginx → KrakenD → subscription-external. Re-validating it via `ResolveKeyPair` + DB lookup is enough.
- Operator confirmed Option B is the architectural fit.

Threat model:
- **Direct-to-service bypass**: subscription-external listens behind nginx and is not directly exposed. The new partner routes accept tenant context via headers/path; without gateway routing, an attacker would have to supply both header and path themselves and would still pass through `ResolveKeyPair` validation + DB lookup. There is no signed-identity escalation since the trusted-service path is preserved on the existing `/admin/*` routes for internal callers.
- **Replay**: external endpoints already accept idempotency keys at the body level; this slice does not introduce a new replay surface.
- **Cross-tenant injection**: `ResolveKeyPair` with `GatewayTrusted=true` (because KrakenD path captures are gateway-derived) detects header/query disagreement → 409. Mirrors TMP-071's notification handler behavior.
- **Audit logging**: handler must log resolved `tenant_id` UUID + `channel_id` UUID on every accept; this is the audit trail.

## Scope (Option B)
- `services/subscription-external/internal/handler/partner_handler.go` — add 4 new handler methods (`PartnerSubscriptionOptin`, `PartnerSubscriptionConfirm`, `PartnerSubscriptionOptout`, `PartnerSubscriptionStatus`) that resolve tenant via a new helper `tenantRouteFromGatewayHeaders(ctx, cfg, h.tenantRepo)`. The helper:
  - Calls `tenantctx.ResolveKeyPair` with `GatewayTrusted: true` (KrakenD's martian header injection makes the path-captured values gateway-trusted).
  - Maps `ErrTenantKeyConflict` → 409 `TENANT_KEY_CONFLICT`.
  - Requires both `tenant_key` and `channel_key`; missing → 400 `TENANT_CONTEXT_REQUIRED`.
  - Looks up `tenant_id` UUID + `channel_id` UUID via repo. Unknown tenant → 400 `UNKNOWN_TENANT`. Unknown channel for tenant → 400 `UNKNOWN_CHANNEL`.
  - Returns a populated `domain.TenantRouteContext` (same shape as `tenantRouteFromRequest`).
- `services/subscription-external/internal/repository/` — if `TenantIDByKey` / `ChannelIDByKeys` (or equivalent) don't already exist, add them. Confirm during build by grepping the repository package.
- `services/subscription-external/internal/transport/router.go` — register 4 new POST routes at `/api/v1/subscription-external/partners/{op}` mapped to the new handlers.
- `krakend/krakend.json` — change the 4 subscription endpoints' upstream `url_pattern` from `/api/v1/subscription-external/admin/{op}` to `/api/v1/subscription-external/partners/{op}`. Inbound endpoint paths (`/api/external/v1/{tenant_key}/{channel_key}/subscriptions/{op}`) and the `header.Modifier` injection block remain unchanged.
- `services/subscription-external/internal/handler/partner_handler_tenant_test.go` (or a new file) — table tests covering: happy path (200 + UUIDs), unknown tenant → 400, unknown channel → 400, missing channel → 400, header/query conflict → 409, no tenant context → 400.
- Out of scope: KrakenD FC template parity (TMP-073 — refresh template after this lands).
- `scripts/smoke/careerify-tenant-e2e.sh` — subscription cases must reach `200` and the cross-tenant refusal Case A must return `409` (not `400`).

## Files NOT to touch
- Existing `tenantRouteFromRequest` helper and the existing `PartnerStatusHandler`/`PartnerOptoutHandler`/`PartnerOptinConfirmHandler` — keep their trusted-service auth path intact for backward compatibility with internal callers.
- KrakenD inbound endpoint definitions (paths + martian header injection).

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
