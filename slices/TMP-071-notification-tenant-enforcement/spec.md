# TMP-071 — notification-tenant-enforcement

## User story
As a security reviewer, an MNO callback to `/api/v1/notification/{type}/{partnerRole}` is rejected with `HTTP 4xx` when `tenant_key` resolves to an unknown row or when `channel_key` is missing, so unknown tenants cannot quietly persist notifications under a `NULL` tenant context.

## Background
TMP-070 smoke against the live shared DB showed two scoping gaps that are NOT covered by TMP-067/TMP-069:

- **Case B (unknown tenant)** — `POST /api/v1/notification/mo/2117?tenant_key=evil-tenant&channel_key=web-gh-airteltigo` returns `200`. The handler's `tenantIDForAdminRead` path falls through to `headerOrQueryTenantID(ctx)` and then `TenantIDByKey` lookup; when lookup fails it logs and returns the empty string, which lets `handleNotification` proceed with `notification.TenantID = nil`. The handler does NOT distinguish "no tenant context supplied" from "tenant context supplied but unknown".
- **Case C (missing channel)** — `POST /api/v1/notification/mo/2117?tenant_key=careerify` (no `channel_key`) returns `200`. The handler does not require a channel; `channelIDFromRequest` returns `""` and the body persists with `channel_id = NULL`.

Both behaviors are silent acceptance of a partial tenant context, which means a partner can submit notifications under any string and have them stored without scoping.

Evidence: `slices/TMP-070-careerify-tenant-e2e-smoke/value-gate-report.md` "Cross-tenant refusal smoke" section.

## Scope
- `services/notification/internal/handler/http.go` — split the tenant-resolution function into "no context supplied" (return empty) vs "context supplied but invalid" (return error). `handleNotification` rejects the latter with `HTTP 4xx`.
- `services/notification/internal/handler/http_test.go` — add table tests for: (a) tenant_key resolves → 200, (b) unknown tenant_key → 4xx, (c) missing channel_key when tenant_key present → 4xx, (d) no tenant context at all (legacy path) → 200 (unchanged).
- `scripts/smoke/careerify-tenant-cross-tenant-refusal.sh` — assert Case B and Case C now return the configured 4xx.

## Out of scope
- Subscription endpoints (covered by TMP-072).
- KrakenD FC template (TMP-073).
- Reconciling tenantless rows in production (separate, TMP-055 territory).

## Acceptance criteria
- Notification handler returns `4xx` with a structured error (matching the existing error envelope) when `tenant_key` is supplied via header or query and does not resolve to a row in `tenants`.
- Notification handler returns `4xx` when `tenant_key` is supplied without `channel_key`.
- Legacy callers that send NO tenant context at all (no header, no query) keep working (returns `200` with `TenantID = nil`).
- `careerify-tenant-cross-tenant-refusal.sh` Cases B and C return their declared expected status (`4xx`) and the suite's overall exit code is `0`.
- Unit tests in `services/notification/internal/handler/http_test.go` cover the four cases above.

## Dependencies
- TMP-066 (seed) — to verify the resolve-success path with a known tenant.
- TMP-069 (resolver) — already present; do NOT change resolver semantics, only the handler's rejection logic.

## Risk
Low. Handler-only change, no schema, no gateway, no shared infra. New rejection behavior could break any partner that has been relying on the silent-accept path — document the change in `docs/tenant-channel-onboarding.md`.

## Verification
```
go test ./services/notification/internal/handler/... -run TenantEnforcement
bash scripts/smoke/careerify-tenant-cross-tenant-refusal.sh   # against local stack
```
