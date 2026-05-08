# TMP-008 Spec: Tenant-Scoped Notification and Cadence Routing

## Story

As a tenant admin, I want to inspect notifications and manage cadence series/content for my tenant channels so that subscriber messaging operations are isolated by tenant and channel.

## Scope

- Require tenant scope on notification list and cadence admin routes in this slice.
- Add tenant/channel persistence to cadence series, content, subscription message state, and message outbox.
- Read tenant/channel context for notifications and worker outbox jobs.
- Preserve legacy global rows until TMP-011 performs default tenant backfill.
- Keep provider credential and broader observability hardening out of this slice.

## Acceptance Criteria

1. `GET /api/v1/notification/list` validates admin bearer identity, rejects missing verified tenant context, and returns only rows for the requested tenant, with optional channel filtering.
2. Notification list cache keys include tenant and channel so a prior tenant's response cannot be reused for another tenant.
3. Inbound notification records can persist tenant/channel context supplied by trusted upstream/gateway headers.
4. `GET /v1/admin/cadence/series` requires tenant context and lists only that tenant's series, optionally filtered by channel.
5. `POST /v1/admin/cadence/content/import/csv` creates or updates tenant-owned series/content rows and cannot reuse another tenant's series key.
6. Nested cadence series routes load series by tenant and return 404 for cross-tenant or channel-mismatched access.
7. Cadence planner writes message outbox jobs with tenant/channel fields and tenant/channel-aware idempotency key parts.
8. Duplicate cadence outbox jobs remain blocked by the existing global `message_outbox.idempotency_key` uniqueness; tenant/channel are encoded into generated keys to avoid cross-tenant collision.
9. Paused subscription message state is not claimed for planning.
10. Notification worker claims tenant/channel context and propagates it to MT dispatch headers/payload; failures remain on the tenant-owned job.

## Evidence Plan

- Handler/service/repository unit tests for notification tenant/channel filtering, missing tenant rejection, and spoofed raw tenant header rejection.
- Repository/admin HTTP tests for cadence tenant filtering, duplicate outbox idempotency, active-only state claims, and missing tenant rejection.
- Migration review for tenant-scoped series uniqueness plus legacy global uniqueness.
- `go test` on `services/notification` and `services/cadence-engine`.
- Value-gate scanner on touched test files.

## Out of Scope

- Frontend admin portal tenant workspace behavior.
- Reporting/operations dashboards.
- Production secret storage and provider credential rotation.
- Legacy default tenant backfill and enforcement of non-null tenant columns.
