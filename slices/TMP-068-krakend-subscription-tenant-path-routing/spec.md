# TMP-068 — krakend-subscription-tenant-path-routing

Skeleton spec. Authored by `/slice-plan` 2026-05-13. To be expanded by `/slice-spec`.

## User story

As an external subscription partner, `POST /api/external/v1/{tenant_key}/{channel_key}/subscriptions/{optin|confirm|optout|status}` reaches the existing subscription-external admin handlers with tenant and channel resolved by KrakenD header injection (Option A), with zero subscription-external handler changes.

## Demo

`curl POST` through gateway for `careerify/web-gh-airteltigo/subscriptions/optin` returns 2xx; `subscription-external` log shows `tenant_id` and `channel_id` resolved; `admin_actions` row written with correct tenant scoping.

## Scope (files in)

- `krakend/krakend.json` — add 4 new endpoints (`optin`, `confirm`, `optout`, `status`).
- `krakend/config/templates/*.tmpl` if templated.
- `scripts/smoke/krakend-subscription-tenant.sh` — new smoke script.
- `slices/TMP-068-krakend-subscription-tenant-path-routing/value-gate-report.md`.

## Scope (files out)

- No `services/subscription-external/internal/handler/` code change. The existing `partner_handler.go:262-293` resolves `X-Tenant-Key`/`X-Channel-Key` already; the gateway is the new contract.
- If integration tests must be added to subscription-external, that is allowed but limited to `*_test.go` files asserting the header path.

## Acceptance

- KrakenD has 4 new `POST` endpoints at `/api/external/v1/{tenant_key}/{channel_key}/subscriptions/{op}` for op in (optin, confirm, optout, status).
- Each endpoint rewrites upstream path to `/api/v1/subscription-external/admin/{op}` (Option A).
- Each endpoint injects headers `X-Tenant-Key` from `{tenant_key}` capture and `X-Channel-Key` from `{channel_key}` capture.
- Smoke script returns 0 with all 4 URLs producing 2xx + tenant-scoped DB rows.

## Verification

See manifest `verification.automated`.

## Notes

Depends on TMP-066. Option A locked in operator decision from mythos-agent-orchestrator turn. Option B (path-aware backend handlers) explicitly rejected.
