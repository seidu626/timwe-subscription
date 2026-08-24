# TMP-076 campaign-app-presence-console

## Problem

The Dayline app catalog reads six nullable campaigns.app_* columns
(app_name, app_tagline, app_description, app_category, app_artwork_url,
app_sample_content) but no console surface exposes them. CPs cannot manage
how their product appears in the app; the columns can only be set by SQL.

## Decision

Extend the existing campaign admin CRUD (acquisition-api /v1/admin/campaigns)
and the webspa-admin campaign form with an "App presence" section.

- domain.Campaign + adminCampaignUpsertRequest gain the six app_* fields.
- Repository Create/Update and admin SELECTs include the columns.
- Artwork upload reuses the shipped MinIO presign pattern: new
  POST /v1/admin/campaign-assets/artwork/presign with key prefix
  campaign-artwork/tenants/{tenant}/{slug}/..., same image-only rules.
- webspa-admin campaign form gains a scroll-spy "App presence" section
  (fields + artwork upload panel mirroring the background-image panel).

No migration needed; columns exist. Empty strings persist as NULL so the
catalog lp_copy fallback keeps working.

## Verification

- cd services/acquisition-api && go build ./... && go test ./...
  (handler tests: upsert request carries app_* fields through to domain;
  repository tests cover INSERT/UPDATE column lists if present)
- cd frontend/webspa-admin && npm run build (Angular type-check gate)
