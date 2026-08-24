# TMP-084: Tenant branding (logo / banner / brand color)

## Problem

The Dayline app renders every tenant identically (generic storefront icon,
no imagery), so tenants cannot present a branded storefront. There is no way
for a tenant or platform operator to attach a logo, banner, or brand color.

## Approach

- Store branding in `tenants.metadata_json -> 'branding'` as
  `{logo_url, banner_url, brand_color}`. No migration needed; metadata_json
  already exists and the admin PATCH already writes it.
- Upload path reuses the campaign asset storage (MinIO + public nginx proxy
  from TMP-083): new `POST /v1/admin/tenant-assets/presign` with
  `{kind: logo|banner, file_name, content_type, size_bytes}`; keys land under
  `tenant-branding/tenants/<tenant>/<kind>/...`.
- Catalog exposure: `ListAppCatalog` selects `t.metadata_json -> 'branding'`,
  parses it into `domain.TenantBranding`, and the marketplace grouping
  surfaces it as `tenants[].branding` (omitted when empty).
- Console: tenant edit form gains a Branding section (logo/banner upload via
  presign + PUT, brand color input). Branding is folded into the metadata
  object the PATCH sends, because the tenant PATCH replaces metadata_json
  wholesale.
- App: `MarketplaceTenant.branding` typed; Discover renders the tenant logo
  in section headers; the storefront screen renders banner + logo.
- Krakend: new AdminApiEndpoint route for `/v1/admin/tenant-assets/presign`
  (repo mirror + live droplet template).

## Non-goals

- Per-product branding overrides.
- Brand-color theming of the whole app screen (TMP-085 handles app themes).
- Image resizing/optimization server-side.

## Verification

- `go build ./... && go test ./internal/... -run 'Branding|AppCatalog|Presign' -count=1`
- webspa-admin: `ng build --configuration production`
- dayline-mobile: `npx tsc --noEmit` and `npx expo lint` (no new issues)
