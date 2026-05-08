# TMP-010 Domain Brief

## Actors
- Operations analyst: views tenant and channel health through reporting and monitoring endpoints (`slices/TMP-010-tenant-reporting-and-ops/slice.yaml`).
- Tenant admin: can view only their tenant's operational reports through JWT tenant context (`services/acquisition-api/internal/transport/admin.go`, `common/auth/tenantctx/identity.go`).
- Platform operator: may request platform-wide aggregation only when identity is platform-scoped (`common/auth/tenantctx/identity.go`).
- Gateway or trusted service: forwards tenant and channel context with signed trusted headers to subscription-external (`common/auth/tenantctx/trusted_service.go`).

## Ubiquitous Language
- Tenant context: `tenantctx.Identity` with `TenantID`, `TenantKey`, `PlatformScoped`, and trust source.
- Channel context: tenant channel IDs/keys used to bind campaigns and provider routing.
- Report filters: date range, tenant, channel, campaign, country, and `all_tenants` for reports.
- KPI report: landing views/clicks, acquisition transactions, subscriptions, charges, revenue, and conversion rates.
- Monitoring dashboard: charging failure metrics, alerts, status, and scoped operational health.
- Degraded status: an explicit signal that scoped operational data is unavailable and must not be hidden as healthy zero data.

## Domain Invariants
- Tenant actors must not aggregate across tenants; tenant reports require `TenantID`.
- `all_tenants=true` is platform-only; tenant admins receive 403.
- Channel filters must belong to the current tenant and be active before report execution.
- Channel-scoped dashboard responses must expose scope and degraded status when scoped metrics are unavailable.
- Revenue/report datasource errors must surface as failures instead of silently converting to zero.

## Failure Modes
- Missing tenant identity: report parsing returns forbidden before repository queries run.
- Unauthorized aggregation: tenant admin requests `all_tenants=true`; handler returns 403 `tenant_aggregation_forbidden`.
- Unsupported channel: malformed or non-tenant channel returns 400 `invalid_channel`.
- Partial datasource failure: KPI revenue source failure returns an error; monitoring missing scoped metrics returns `status=degraded`.
- Empty tenant data: report repositories return zero metrics with filters echoed.

## User Journey
1. Tenant admin calls `GET /v1/admin/reports/kpis` with JWT tenant context.
2. Reports handler builds tenant/channel filters, validates channel ownership, and queries tenant-scoped aggregates.
3. Operations analyst calls `GET /api/v1/subscription-external/monitoring/dashboard` through trusted gateway headers.
4. Monitoring handler returns tenant/channel dashboard metrics or an explicit degraded response if scoped metrics are unavailable.

## Open Questions
- `landing_events` still lacks direct `tenant_id` and `channel_id`; reports infer scope through campaign slug. A later migration should denormalize tenant/channel onto landing events to eliminate duplicate-slug ambiguity.
- Subscription-external charging failure SQL remains global outside the dashboard scope model; deeper list/stats mutation scoping should be a follow-on slice if operations endpoints beyond the declared dashboard entrypoint are expanded.
