# TMP-066 — careerify-tenant-onboarding-seed

Skeleton spec. Authored by `/slice-plan` 2026-05-13. To be expanded by `/slice-spec`.

## User story

As a platform operator, I can onboard the `careerify` tenant with its `web-gh-airteltigo` channel and provider credentials so downstream gateway and notification slices route to real tenant rows.

## Demo

Run the new migration locally; `SELECT tenant_id FROM tenants WHERE tenant_key='careerify'` returns a UUID; `tenant_channels` has `web-gh-airteltigo` bound to that tenant; `services/subscription-external/internal/service/tenant_routing.go:217` query for that pair returns provider credentials.

## Scope (files in)

- `services/*/migrations/` — new migration(s) creating tenant + channel + credential rows for careerify.
- `slices/TMP-066-careerify-tenant-onboarding-seed/value-gate-report.md`.

## Scope (files out)

- Anything in `krakend/`, `ops/nginx/`, or service handlers. That is TMP-067+/-068/-069.

## Acceptance

- New tenant row exists with `tenant_key='careerify'`.
- New channel row exists with `channel_key='web-gh-airteltigo'` bound to that tenant.
- Provider credential rows referenced by `tenant_routing.go:217` query exist for that pair (purpose = `tenantCredentialPurposeProviderAPI`).
- Migration is forward-only and idempotent.
- Tests verify the tenant_routing lookup succeeds.

## Verification

See manifest `verification.automated`.

## Notes

Reuse the `nrg` seed pattern from recent commits (`1d0d0a4`, `6428050`). HVC is not required in this repo.
