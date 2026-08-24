# TMP-086: Workspace branding self-service + list imagery

## Problem

Tenant branding (logo/banner/brand color) was only editable by platform-scoped
admins: the console's branding editor lives inside the tenant catalog form,
which renders a read-only summary in workspace mode, and the backend tenant
PATCH hard-rejects tenant-scoped identities (`ErrAdminForbidden`). Workspace
operators therefore could not manage their own branding at all. Separately,
neither the tenant catalog nor the campaign list showed any imagery, so
branding and app artwork were invisible outside the edit dialogs.

## Change

Backend (acquisition-api):
- `PUT /v1/admin/tenants/current/branding` - available to any identity with
  tenant context (workspace operators included). Validates http(s) URLs
  (max 2048 chars) and `#RRGGBB` brand color.
- Service `UpdateCurrentTenantBranding` resolves the caller's own tenant via
  `ResolveCurrentTenant` and writes an `update_branding` audit log entry.
- Repo `UpdateTenantBrandingWithActivityLog` uses
  `jsonb_set(COALESCE(metadata_json,'{}'), '{branding}', ...)` so service
  config in metadata_json is never clobbered; all-empty payload removes the
  branding key (`- 'branding'`).

Frontend (webspa-admin):
- Branding editor extracted to a shared `ng-template` and now rendered in BOTH
  platform edit mode and workspace mode; workspace mode gets a dedicated
  "Save Branding" button wired to the new endpoint (uploads already worked
  for tenant scope via the presign route).
- Tenant catalog table: logo thumbnail (fallback business icon) + brand color
  swatch in the Name column.
- Campaign list: new leading "Product" column with app artwork thumbnail
  (fallback image icon) + app_name.

## Verification

- `go build ./...` clean; `go test ./internal/handler/ ./internal/service/
  ./internal/repository/ ./internal/transport/` -> 446 passed.
- New tests: tenant-scoped save preserves other metadata keys; all-empty
  payload removes branding; invalid color/scheme -> 400; no tenant context
  -> 403.
- `npm run build` (webspa-admin) clean (pre-existing budget warnings only).
