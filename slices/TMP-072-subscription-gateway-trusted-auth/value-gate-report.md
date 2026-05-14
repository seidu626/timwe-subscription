# TMP-072 — value gate report

## Verdict

**PASS** — Option B partner handlers ship: external subscription endpoints no longer require trusted-service auth; tenant context resolved via `tenantctx.ResolveKeyPair` + DB lookup; KrakenD now rewrites `/api/external/v1/{tenant_key}/{channel_key}/subscriptions/{op}` to `/api/v1/subscription-external/partners/{op}`.

## What shipped

| Layer | File | Change |
|---|---|---|
| Handler | `services/subscription-external/internal/handler/partner_handler.go` | 4 new handler methods (`PartnerSubscriptionOptin`, `PartnerSubscriptionConfirm`, `PartnerSubscriptionOptout`, `PartnerSubscriptionStatus`); new `tenantRouteFromGatewayHeaders` helper that goes through `ResolveKeyPair(GatewayTrusted: true)` then `repo.TenantIDByKey` + `repo.ChannelIDByKeys`; new `gatewayTenantLookup` interface; `WithTenantRepo` setter. Existing trusted-secret path (`tenantRouteFromRequest`) untouched. |
| Repository | `services/subscription-external/internal/repository/postgres.go` | Added `ErrTenantChannelNotFound` sentinel and `ChannelIDByKeys(tenantID, channelKey)`; SQL filters `status = 'ACTIVE'`. `TenantIDByKey` already existed (line 61). |
| Transport | `services/subscription-external/internal/transport/router.go` | Registered 4 new POST routes at `/api/v1/subscription-external/partners/{optin,confirm,optout,status}`. Existing admin routes (`/admin/{op}`) unchanged. |
| Gateway | `krakend/krakend.json` | Changed `url_pattern` for the 4 inbound subscription endpoints (lines 3985/4072/4159/4246) from `/admin/{op}` → `/partners/{op}`. Inbound paths, `input_query_strings`, and martian header injection unchanged. Other admin routes (lines 743–1056) untouched. |
| Cmd wiring | `services/subscription-external/cmd/main.go` | `partnerHandler := handler.NewPartnerHandler(...).WithTenantRepo(repo)` — wires the gateway lookup helper. |
| Handler tests | `services/subscription-external/internal/handler/partner_subscription_handler_test.go` | New file. 30 tests across the 4 endpoints covering 6 scenarios per endpoint (happy + 5 rejection cases). |
| Smoke comment | `scripts/smoke/careerify-tenant-cross-tenant-refusal.sh` | Case A code corrected from `TENANT_CONTEXT_REQUIRED` to `TENANT_KEY_CONFLICT`. |

## Acceptance criteria check

| Criterion | Status | Evidence |
|---|---|---|
| `POST /api/external/v1/careerify/web-gh-airteltigo/subscriptions/{op}` returns 2xx through the gateway | DEFERRED (unit-verified, e2e pending TMP-073 + live stack) | Unit tests pass for the four new handler methods. End-to-end gateway run requires TMP-073's KrakenD FC template parity, since the runtime image renders templates not `krakend.json`. |
| Cross-tenant refusal Case A returns 409 + `TENANT_KEY_CONFLICT` | PASS (unit) | `TestGatewayRouteStatus_ConflictMapsTo409` + adversarial-review trace confirms the conflict branch in `ResolveKeyPair` is exercised. |
| Audit logs show tenant_id + channel_id UUIDs | PASS | All 4 handlers log resolved UUIDs at Info level before processing (partner_handler.go:355/390/425/459). |
| Existing internal admin callers keep working | PASS | `tenantRouteFromRequest` (trusted-secret helper) and the existing admin routes are unchanged. `go test ./internal/handler/...` reports 37/37 pass including the legacy partner-handler tests. |

## Adversarial review trajectory

Reviewer flagged 2 defects on initial commit `2085fb0`:

1. **CRITICAL — DI wiring miss.** `cmd/main.go` instantiated `NewPartnerHandler(logger, svc, cfg)` but never called `.WithTenantRepo(repo)`, so `h.tenantRepo` was nil at runtime and every new `/partners/{op}` endpoint would have returned `500 INTERNAL_ERROR` in production despite unit tests passing (tests inject the repo through a stub setter). Fixed in `0b7cb57` (cherry-picked as `7e28426` on main): chain `.WithTenantRepo(repo)` onto the constructor call.
2. **HIGH — Design clarification.** `GatewayTrusted=true` allows query-only requests to pass `ResolveKeyPair` even when KrakenD is bypassed. This is the locked Option B decision (nginx network isolation is the operational gate; downstream DB lookup is the secondary defence) but the rationale was undocumented in code. Fixed in `0b7cb57` by adding a multi-line comment in `tenantRouteFromGatewayHeaders` explaining the policy and what would have to change to flip it (e.g., wire an HMAC trust marker through KrakenD).

Plus 1 LOW: smoke-script comment for Case A said `TENANT_CONTEXT_REQUIRED` but the correct code is `TENANT_KEY_CONFLICT` — fixed in the same repair commit.

Re-verified on the repair: build OK, 37/37 handler tests pass on main after cherry-pick (commits `14720cd` + `7e28426`).

## Risk notes

- **Direct-to-service bypass with GatewayTrusted=true** is an accepted residual risk per the locked threat model. If the deployment surface ever changes (e.g., subscription-external is exposed to a different network boundary, or nginx is replaced), revisit the trust gate. The in-code comment is the breadcrumb.
- **DI wiring is the failure mode this slice can re-introduce** if future PRs add another route. Consider asserting on startup that `h.tenantRepo != nil` for the new endpoints — deferred as a future hardening.
- Repair commit adds one indexed DB read per request (`ChannelIDByKeys`). Mirror of TMP-071 cost; not expected to be a bottleneck.

## Deferred follow-ups

- End-to-end smoke against a local docker-compose stack — depends on TMP-073 FC template parity to make KrakenD runtime serve the new `url_pattern`.
- Optional hardening: assert non-nil `tenantRepo` at handler-startup so a future DI miss fails fast (not at first request).
- TMP-073 must refresh `krakend/config/templates/Endpoint.tmpl` to reflect `/partners/{op}` upstream pattern (called out in TMP-073 spec).
