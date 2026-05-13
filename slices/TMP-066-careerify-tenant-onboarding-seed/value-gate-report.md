# TMP-066 value-gate-report

**Run-id:** slice-TMP-066-build-2026-05-13T2012Z
**Date:** 2026-05-13

## Evidence

### Migration authored
- File: `services/acquisition-api/migrations/seed_careerify_tenant_channel.sql`
- Seeds three rows in order: `tenants`, `tenant_channels`, `tenant_channel_credentials`.
- Forward-only, idempotent via `ON CONFLICT ... DO NOTHING` on each unique index:
  - `tenants(tenant_key)` — built-in UNIQUE constraint on the column.
  - `tenant_channels(tenant_id, channel_key)` — `idx_tenant_channels_tenant_key` unique index.
  - `tenant_channel_credentials(tenant_id, channel_id, purpose, version)` — `idx_tenant_channel_credentials_version` unique index.
- `secret_ref = 'env://CAREERIFY_TIMWE_API_SECRET'` satisfies `chk_tenant_channel_credentials_secret_ref`.
- `secret_fingerprint` is 64 hex chars; satisfies `length(secret_fingerprint) = 64`.

### tenant_routing.go lookup query contract verified
Query at lines 208–222 JOINs tenants → tenant_channels → tenant_channel_credentials
filtering `c.status='ACTIVE'`, `cred.purpose='provider_api'`, `cred.status='ACTIVE'`.
The seeded row satisfies all filters and returns `provider='timwe'` with a non-empty `secret_ref`.

### Test result
```
go test ./services/subscription-external/internal/service/... -run TestTenantRoutingCareerifyChannelLookup
PASS (1/1)
go test ./services/subscription-external/internal/service/... (full suite)
PASS (67/67)
```

`TestTenantRoutingCareerifyChannelLookup` drives `TenantProviderRouter.Resolve` via a
fake `database/sql` driver returning the seeded row values and asserts:
- `provider == "timwe"`
- `tenant_id`, `channel_id` non-empty
- `secret_ref_display == "careerify-timwe-api"`
- `APIKey` and `BaseURL` populated from `env://CAREERIFY_TIMWE_API_SECRET`

## Acceptance criteria status

| Criterion | Status |
|---|---|
| tenants row with tenant_key='careerify' | PASS — migration inserts it |
| tenant_channels row with channel_key='web-gh-airteltigo' bound to tenant | PASS — migration inserts it |
| tenant_channel_credentials row with purpose='provider_api', status='ACTIVE', non-empty secret_ref | PASS — migration inserts it |
| Migration forward-only and idempotent | PASS — no DROP, ON CONFLICT DO NOTHING on all three |
| Tests verify tenant_routing lookup succeeds | PASS — 67/67 unit tests pass |
