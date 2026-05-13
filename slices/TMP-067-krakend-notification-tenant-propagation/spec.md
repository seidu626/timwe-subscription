# TMP-067 — krakend-notification-tenant-propagation

Skeleton spec. Authored by `/slice-plan` 2026-05-13. To be expanded by `/slice-spec`.

## User story

As an MNO partner, my callback to `/api/v1/notification/{type}/{partnerRole}?tenant_key=careerify&channel_key=web-gh-airteltigo` reaches the notification service with `X-Tenant-Key` and `X-Channel-Key` injected by KrakenD so tenant context is resolved before any work.

## Demo

`curl` through gateway against the 6 notification URLs (`mo`, `mt/dn`, `user-optin`, `user-renewed`, `user-optout`, `charge`) for `careerify`; notification service log shows `tenant_id` resolved for careerify and request body is persisted scoped to that tenant.

## Scope (files in)

- `krakend/krakend.json` — modify the 6 notification endpoint entries.
- `krakend/config/templates/*.tmpl` if endpoints are templated.
- `scripts/smoke/krakend-notification-tenant.sh` — new smoke script (in PATH for verification).
- `slices/TMP-067-krakend-notification-tenant-propagation/value-gate-report.md`.

## Scope (files out)

- No `services/notification/` code changes expected (existing tenantctx middleware reads `X-Tenant-Key`/`X-Channel-Key` headers). Verify; if a small fallback is needed, lift it to TMP-069 instead of conflating slices.

## Acceptance

- All 6 `/api/v1/notification/...` endpoints in `krakend/krakend.json` declare `input_query_strings: ["tenant_key", "channel_key", ...]`.
- KrakenD injects `X-Tenant-Key` and `X-Channel-Key` headers to upstream from those query params (KrakenD modifier or static plugin).
- Smoke script `scripts/smoke/krakend-notification-tenant.sh` returns 0; each URL gets a 2xx and notification service logs the resolved `tenant_id`.

## Verification

See manifest `verification.automated` and `manual_smoke`.

## Notes

Depends on TMP-066. Defence-in-depth contract (header vs query precedence) lives in TMP-069, not here.
