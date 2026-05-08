# TMP-010 Roadmap Slice Notes

TMP-010 remains the P3 operations/reporting slice in the tenant multi-channel roadmap.

## Implemented Boundary
- `GET /v1/admin/reports/kpis`
- `GET /v1/admin/reports/campaign-performance/export`
- `GET /api/v1/subscription-external/monitoring/dashboard`

## Deliberate Scope Control
- No frontend dashboard redesign.
- No BI warehouse or reporting service split.
- No broad renewal-worker rewrite.
- No migration for `landing_events` tenant/channel columns in this slice; the repository uses campaign ownership as the current scoping source.

## Follow-On Candidate
Create a dedicated slice for tenant/channel scoping of subscription-external charging failure list, stats, summary, and mutations before those endpoints are exposed as tenant-admin operations surfaces.
