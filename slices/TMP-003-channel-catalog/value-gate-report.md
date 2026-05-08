# TMP-003 Value Gate Report

Verdict: PASS

## Slice

Tenant admin defines channel capabilities for future tenant/channel routing.

## Claude And Parallel Review Applied

Claude and the read-only explorers aligned on these amendments:

- Implement inside the existing acquisition-api admin-management module.
- Add a tenant-owned channel catalog with no credential or secret-shaped fields.
- Use tenant-scoped unique channel keys and tenant-scoped update predicates.
- Store capabilities as `TEXT[]` with closed validation rather than JSONB.
- Return 404 for cross-tenant mutation attempts.

## Acceptance Evidence

- `tenant_channels` migration adds tenant FK, status, capability checks, scoped uniqueness, and provider/scope uniqueness.
- Admin schema bootstrap now applies the admin-management migration set and verifies `public.tenant_channels`.
- Domain model exposes `channel_id`, `tenant_id`, `channel_key`, provider scope, capabilities, status, and enabled state with no credential fields.
- Service validation normalizes provider/country/operator, derives channel_key, deduplicates/sorts capabilities, rejects unsupported capabilities, and enforces `charge` requires `mt`.
- Handler routes support:
  - `GET /v1/admin/channels`
  - `POST /v1/admin/channels`
  - `PATCH /v1/admin/channels/{id}/enabled`
- Patch enabled uses a strict JSON body and tenant-scoped single-statement update.

## Verification

Commands run:

```bash
cd services/acquisition-api && go test ./internal/repository ./internal/service ./internal/handler ./internal/transport
cd services/acquisition-api && go test ./...
```

Results:

- Focused acquisition-api packages passed.
- Full acquisition-api suite passed.

## Risk Notes

- The channel key is server-derived as `provider-country-operator`; later channel-type additions may need an explicit type field if non-SMS providers share the same provider/scope.
- Credential binding remains out of scope and intentionally absent from the schema.
