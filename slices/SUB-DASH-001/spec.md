# Slice SUB-DASH-001: subscription operations dashboard

## User story
As a tenant operations admin, I can see current subscription counts beside TIMWE's latest daily billing totals so that I can detect gaps without treating acquisition estimates as provider revenue.

## Demo script
1. Configure one TIMWE report source for a tenant, shortcode, and product, then run the importer against a known CSV response.
2. Open `/operations` in that tenant workspace and select a date range.
3. Observe current local subscriptions, TIMWE daily billings, a provider import timestamp, and a source-specific stale state.

## Acceptance criteria
- [ ] `GET /v1/admin/reports/subscription-health` requires authenticated tenant context and accepts inclusive `startDate`, `endDate`, optional positive `productId`, and optional `shortcode`.
- [ ] The response contains `as_of`, `subscriptions: {active,inactive,other,total}`, and `provider: {state,last_imported_at,success_billings,revenue,currency,daily}`. `revenue` is a decimal string; `currency` is null unless configured from a confirmed source.
- [ ] Provider `state` is `fresh`, `stale`, or `unavailable`. Unavailable totals are null; a successful empty import returns numeric zero.
- [ ] A channel-filtered request never returns a tenant-wide provider amount as a channel amount.
- [ ] The importer uses server-side Basic Auth over HTTPS, accepts only configured report URLs, parses the observed Power BI Report Server CSV shape, validates every row against source shortcode/product, and replaces the fetched date window atomically.
- [ ] A failed fetch or parse preserves earlier rows and records a safe error state; repeated imports do not add duplicate billings.
- [ ] The Operations UI separates local and provider cards, displays import time and provisional current-day status, and refreshes local data without displaying stale provider data as live.

## Layers touched
- **Schema / migrations**: acquisition API migration for daily provider rows and sync status, tenant-scoped indexes.
- **Storage / RLS**: direct PostgreSQL queries with tenant predicates; no new subscriber PII.
- **API / handlers**: acquisition admin reports route and tenant-scoped handler.
- **Business logic**: TIMWE CSV parser, HTTP client, periodic import worker, local subscription projection.
- **UI components**: existing Operations page and its API service.
- **Types / contracts**: typed JSON response described above.
- **Tests (unit + e2e)**: CSV parser and import failure tests, scope tests, Angular build, browser journey when a running local stack is available.
- **Config / env vars**: report URL, Basic Auth secret, source mapping, poll interval; disabled when unset.
- **Observability**: import success/failure and last successful import time, without credentials or full URL.
- **Security**: fixed HTTPS report host, no redirects off host, bounded response size, authenticated tenant scope.
- **Reliability**: bounded timeout, atomic replacement, stale state, no claim of live provider billing.
- **Performance**: bounded date range and tenant/product indexes.
- **Docs**: configuration, rollout, rollback and known data limits.

## Out of scope
- Provider-confirmed per-transaction streaming until TIMWE supplies an event feed.
- Charge attempts, charge failures, unique billed subscribers, and currency inference from the screenshot.
- Publishing credentials or performing a production migration/deployment in this slice.

## Risks and mitigations
- **Wrong provider attribution**: validate shortcode/product and tenant source configuration before committing an import.
- **Partial-day revisions**: replace recent report windows on every successful import.
- **Report outage**: retain last good rows and expose stale state.
- **Cross-tenant reads**: derive tenant from auth context and enforce it in every query.

## Feature flag?
Yes: importer is disabled until `TIMWE_REPORT_URL`, credentials, and source mapping are configured.

## Definition of Done (slice-specific)
- [ ] Import the captured CSV shape with the expected count and decimal amount.
- [ ] Prove repeated import replacement, malformed-report rollback, tenant refusal, and unavailable provider semantics.
- [ ] Browser-check the Operations view or record the exact runtime blocker.

## Estimated layers of work
One migration, one admin endpoint, one importer, existing Operations UI, focused tests and a runbook. No new dependencies.
