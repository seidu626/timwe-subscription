# Reporting v1 (KPIs + Acquisition)

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-16
- Completed: 2026-01-16

## Dependencies
- Acquisition API (`services/acquisition-api`) must run against the same Postgres schema that includes `campaigns` and `acquisition_transactions`.
- Landing Web (`services/landing-web`) must be able to send analytics events to Acquisition API.
- Admin UI (`frontend/webspa-admin`) must have valid Acquisition admin token configured via Settings.

## ExitCriteria
- [x] Admin UI "Reports" menu routes to a working page and shows:
  - KPIs for selected date range
  - Acquisition funnel starting at landing events
  - Campaign performance table
  - Time series charts
- [x] Backend report endpoints are under `/v1/admin/reports/*` and protected by existing `X-Admin-Token`.
- [x] Landing events ingestion exists and does **not** store MSISDN.
- [x] Tests pass for new backend report + ingestion logic.

## Todos
1. task-scaffold — Create task entry + active record + logging updates [completed]
2. db-landing-events-migration — Add a migration creating `landing_events` + indexes in `services/subscription-external/migrations/`. [completed]
3. acq-api-landing-events-endpoint — Implement `POST /v1/analytics/landing/events` in acquisition-api with validation and safe logging (no sensitive values). [completed]
4. acq-api-admin-reports-endpoints — Implement `/v1/admin/reports/*` endpoints (kpis, timeseries, funnel, campaign-performance) using aggregated SQL queries and reusing existing admin token auth. [completed]
5. angular-reports-page — Add `reports` route + new reports module/page that calls acquisition-api report endpoints and renders KPI cards, charts, and campaign table. [completed]
6. tests — Add backend unit/integration tests for the new ingestion + report endpoints; add frontend minimal component/service tests where feasible. [completed]

## Notes
- Revenue definition (v1): estimate using `charge_payout` when present, else fallback to `products.price_point_value` for charged transactions.
- Funnel start (v1): include landing events (landing_view, optional landing_click).

## Implementation Summary
- Created `008_landing_events.sql` migration with indexes for funnel queries
- Added `POST /v1/analytics/landing/events` public endpoint for landing page event ingestion
- Added `GET /v1/admin/reports/kpis`, `/acquisition-funnel`, `/campaign-performance`, `/timeseries` admin endpoints
- Created Angular Reports module with filters, KPI cards, funnel visualization, and campaign performance table
- Added unit tests for landing event validation, handler utilities, and filter parsing

