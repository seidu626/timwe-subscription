# Admin Route And Tenant Context Failure Evidence

Date: 2026-06-12
WorkOrder: `WO-FIX-PRODUCTION-ADMIN-API-ROUTE-FAILURES-REPORTED`
Services:

- `krakend`
- `cadence-engine`

## Debug Ledger

Symptom:

- `GET https://api.nouveauricheglobalgroup.com/api/v1/renewal/worker/status` returned HTTP 404 from KrakenD.
- `GET https://api.nouveauricheglobalgroup.com/v1/admin/cadence/series?limit=500` returned HTTP 403 with `tenant context required` for an authenticated admin UI request.

Reproduction:

- Unauthenticated renewal status reproduced the KrakenD 404.
- Unauthenticated cadence returned HTTP 401, proving the user-observed 403 occurs after Auth0 validation.
- Remote `/etc/krakend` contained cadence admin routes but no renewal worker route.

Operator correction count: first report after the previous deploy.

Current evidence:

- Local static `krakend/krakend.json` contained `/api/v1/renewal/worker/status`.
- Deploy source `krakend/config/templates/Endpoint.tmpl` did not contain the renewal worker routes, so `krakend-sync` could never render them on the droplet.
- `cadence-engine` only accepted selected tenant headers for platform-scoped identities.
- `acquisition-api` already supports membership-backed tenant admins by resolving `tenant_admin_memberships` and accepting a selected `X-Tenant-Key` only when it matches an active membership.

Hypotheses:

- H1: renewal status 404 is stale or incomplete gateway render source.
- H2: cadence 403 is a backend tenant-auth contract mismatch for non-platform tenant admins whose tenant access comes from membership rows rather than JWT tenant claims.

Experiments:

- Search local and remote KrakenD config for `renewal/worker/status`.
- Read cadence access and tenant scope code.
- Compare acquisition-api admin access tenant resolution.
- Add focused tests for membership-backed cadence access.

Result:

- H1 confirmed.
- H2 confirmed.

Root cause:

- Renewal worker routes existed only in static checked JSON, not the KrakenD flexible-config template used for production deploy.
- `cadence-engine` had not been updated to the same tenant membership contract as `acquisition-api`, so authenticated membership-backed tenant admins could reach the service but had no stamped tenant context.

Patch plan:

- Add JWT-protected renewal worker start, stop, and status routes to `krakend/config/templates/Endpoint.tmpl`.
- Add membership-backed tenant context stamping to `cadence-engine`.
- Resolve memberships through `tenant_admin_memberships`; do not trust arbitrary selected tenant headers.
- Wire cadence-engine main to the repository-backed lookup.

Verification:

- `go test ./internal/adminhttp -run 'TestRequire.*Tenant|TestHandleSeriesReturnsErrWhenTenantMissing|TestTenantScopeResolvesPlatformTenantKey'`
- `go test ./internal/repository -run 'TestListActiveTenantsForMember|TestTenantIDByKey'`
- `go test ./internal/...`
- `go test ./...`
- `go build -o cadence-engine ./cmd/cadence-engine`
- `just krakend-check-do`
- rendered KrakenD debug output includes:
  - `POST /api/v1/renewal/worker/start`
  - `POST /api/v1/renewal/worker/stop`
  - `GET /api/v1/renewal/worker/status`
- `npm test -- --watch=false --browsers=ChromeHeadlessNoSandbox --include src/app/core/http-interceptors/tenant-workspace.interceptor.spec.ts --include src/app/core/guards/tenant-workspace.guard.spec.ts --include src/app/core/services/tenant-workspace.service.spec.ts`
- `npm run build:prod`
- `docker compose -f docker-compose.prod-do.yml config --quiet`
- `agent-hub schema validate-all --root . --json`
- `agent-hub reconcile --root . --check --json`

Residual risk:

- The authenticated browser request was not replayed with the user's live Auth0 token in local verification.
- Production was deployed after source verification. The authenticated browser request still requires the user's live Auth0 session for final UI confirmation.

## Production Deploy

Mode:

- service-level deploy for `cadence-engine`
- gateway config sync for `krakend`

Commit deployed: `84f1d94`

Rollback baseline:

- previous `cadence-engine` image ID: `55c929afc56e`
- previous remote KrakenD template hash: `fc1e4d71ae218e3c6a18a6a5e00a14f56a7853bd2a5307a9d3613f8b07c15eba`
- previous remote rendered KrakenD hash: `15fa4975c3b8ffa003cc7591cd47525e2d01fda7d1f09d71e9dbd223dc8da8e3`

Deploy commands:

- `just deploy-cadence-engine`
- `just krakend-sync`

Production verification:

- remote `cadence-engine` image ID: `4d2552495650`
- remote `cadence-engine`: `Up`, `healthy`, port `8091`
- `krakend`: active
- `nginx`: active
- remote `/etc/krakend/config/templates/Endpoint.tmpl` includes `/api/v1/renewal/worker/status`
- `GET https://api.nouveauricheglobalgroup.com/api/v1/renewal/worker/status` now returns HTTP 401 instead of HTTP 404, proving the route exists and is JWT-protected
- unauthenticated `GET https://api.nouveauricheglobalgroup.com/v1/admin/cadence/series?limit=1` returns HTTP 401
- `GET https://api.nouveauricheglobalgroup.com/health` returns `healthy`
- `HEAD https://admin.nouveauricheglobalgroup.com/` returns HTTP 200
- cadence-engine post-deploy logs show startup and admin listener on `:8091`

Deployment caveat:

- `sudo -n nginx -t` could not run for this user because sudo requires a password. nginx configuration was not changed.

## Secrets Handling

No bearer token, Auth0 subject, provider API key, provider PSK, database password, or raw subscriber number is included in this evidence.
