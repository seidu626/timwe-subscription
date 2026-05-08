# Campaign admin CRUD

## Status
- Owner: AI assistant
- State: completed
- Started: 2026-01-16
- Completed: 2026-01-16

## Goal
Enable creation and management of campaigns in `frontend/webspa-admin`, persisted in `services/acquisition-api` so landing pages can use them.

## Dependencies
- Backend: `services/acquisition-api`
- Admin UI: `frontend/webspa-admin`
- Landing usage: `services/landing-web` (`GET /v1/campaigns/{slug}` via `ACQUISITION_API_URL`)

## ExitCriteria
- Admin can list/create/update/enable-disable campaigns via token-protected endpoints.
- CORS preflight works for admin UI in browser.
- Landing page continues to work unchanged and can fetch campaign by slug.
- Basic tests added for auth/validation and at least one CRUD happy path.

## Notes
- Auth choice: `X-Admin-Token` header (token from env on API side).
- Public endpoints remain read-only and return `PublicCampaign` subset.
- `/tasks` structure created (TaskIndex + active task + monthly log).

### Backend (acquisition-api) - Completed
- Implemented admin routing + auth/CORS helper (`internal/transport/admin.go`).
- Added admin CRUD handlers (`internal/handler/campaign_handler.go`).
- Added admin service methods with CampaignRepo interface (`internal/service/campaign_service.go`).
- Added repo methods: Create, Update, ListAll, SetEnabled, GetAdminBySlug.
- Added unit tests for admin CRUD happy path and validation.
- Updated README.md with env vars documentation.

### Frontend (webspa-admin) - Completed
- Added `acquisitionApiEndpoint` and `landingWebBaseUrl` to environments.
- Created `AdminTokenInterceptor` for X-Admin-Token header.
- Created Campaign model and CampaignService.
- Created campaigns feature module with list/create/edit pages.
- Created settings module for admin token storage.
- Updated sidebar navigation with Campaigns and Settings links.
- Cleaned up hardcoded Bearer token in data.service.ts.
- Fixed pre-existing bug in notification component (`filters.request` → `filters.msisdn`).

