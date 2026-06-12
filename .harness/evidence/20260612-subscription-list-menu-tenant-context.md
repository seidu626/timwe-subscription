## Debug Ledger

Symptom:
- Production `GET /api/v1/subscription/list?page=1&pageSize=10&sort_by=startDate&sort_dir=desc` returned `403 Tenant context is required`.
- The admin SPA did not show the tenant management menu for a bootstrap operator token whose access token had a subject but no email claim.

Reproduction:
- Code path inspection: `subscription-partner` requires `X-Tenant-Id` or `X-Tenant-Key` for `/api/v1/subscription/list`.
- Deployed KrakenD flexible-config template routed `/api/v1/subscription/list` through `TenantApiEndpoint`, which only forwards tenant query params and drops the admin SPA's `X-Tenant-Key` / `X-Tenant-Id` headers.
- Production droplet `.env` had both `ADMIN_BOOTSTRAP_PLATFORM_EMAILS` and `ADMIN_BOOTSTRAP_PLATFORM_SUBJECTS` empty, so backend workspace bootstrap could not promote an email-less approved subject to platform scope.
- Frontend bootstrap config did not accept `platformAdminSubjects`, so runtime subject bootstrap could not mirror the backend subject contract.

Operator correction count:
- First correction after the earlier cadence/renewal route fix.

Current evidence:
- `services/subscription-partner/internal/handler/http.go` rejects tenantless subscription list requests.
- `krakend/config/templates/Endpoint.tmpl` previously used `TenantApiEndpoint` for `/api/v1/subscription/list`.
- `krakend/config/templates/AdminApiEndpoint.tmpl` forwards `Authorization`, `X-Tenant-Key`, and `X-Tenant-Id`.
- `frontend/webspa-admin/src/app/core/services/tenant-workspace.service.ts` previously only accepted bootstrap emails.
- Remote bootstrap env state before rollout: `ADMIN_BOOTSTRAP_PLATFORM_EMAILS=empty`, `ADMIN_BOOTSTRAP_PLATFORM_SUBJECTS=empty`.

Hypotheses:
- H1: Gateway drops tenant headers on subscription list. Prediction: switching the route to `AdminApiEndpoint` preserves wildcard query params, validates JWT, and forwards selected tenant headers.
- H2: Menu is hidden because platform scope cannot be established for an approved subject-only token. Prediction: adding frontend subject bootstrap support and setting backend subject bootstrap makes `/v1/admin/tenants/workspaces` return platform scope, causing the existing nav filter and guard to allow tenant management.

Experiments:
- Render KrakenD DO config and inspect `/api/v1/subscription/list`.
- Run focused subscription list handler tests.
- Run acquisition/notification subject bootstrap tests.
- Run admin SPA tenant workspace tests and production build.

Predicted observations:
- KrakenD render includes `auth/validator`, wildcard query forwarding, and the admin wrapper for subscription list.
- Existing tenantless backend rejection remains intact.
- Subject bootstrap works without an email claim.

Stale-runtime evidence:
- Remote `nginx` and `krakend` were active before rollout.
- Remote `acquisition-api` was healthy before rollout.
- Production env lacked bootstrap subject/email values before rollout.

Result:
- Local implementation verified.
- Production rollout completed on 2026-06-12.

Root cause:
- Contract mismatch: admin SPA sends tenant context in headers, but the deployed gateway template used the public tenant-query route wrapper for subscription list.
- Authorization config gap: subject-only bootstrap existed in backend code but was not configured in production, and frontend runtime bootstrap did not support subject keys.

Patch plan:
- Route `/api/v1/subscription/list` through `AdminApiEndpoint`.
- Add `platformAdminSubjects` to frontend tenant bootstrap config and tests.
- Document backend/frontend subject bootstrap contract.
- Remove real-looking Auth0 subject fixtures from tests.
- Deploy gateway config, admin SPA image, and production bootstrap-subject env update with an acquisition-api restart.

Verification:
- `go test ./internal/transport -run 'TestAdminRequireAppliesBootstrapPlatformSubject|TestAdminRequireStampsSingleMembershipTenant'` in `services/acquisition-api`: pass.
- `go test ./internal/transport -run 'TestAdminRequireAppliesBootstrapPlatformSubject'` in `services/notification`: pass.
- `go test ./internal/handler -run 'TestListSubscriptions_ReturnsPaginationHeaderAndBody|TestListSubscriptions_RequiresTenantContext'` in `services/subscription-partner`: pass.
- `go test ./internal/...` in `services/acquisition-api`: pass.
- `go test ./internal/...` in `services/notification`: pass.
- `go test ./internal/...` in `services/subscription-partner`: pass.
- `npm test -- --watch=false --browsers=ChromeHeadlessNoSandbox --include src/app/core/services/tenant-workspace.service.spec.ts --include src/app/core/guards/tenant-workspace.guard.spec.ts --include src/app/core/http-interceptors/tenant-workspace.interceptor.spec.ts`: 17 success.
- `npm run build:prod`: pass with existing SCSS budget and selector warnings.
- `just krakend-check-do`: pass.
- `docker compose -f docker-compose.prod-do.yml config --quiet`: pass.
- Production deploy:
  - Remote `.env` backup created: `.env.bak-20260612T075703Z-subscription-menu`.
  - Remote `ADMIN_BOOTSTRAP_PLATFORM_SUBJECTS` set without printing the value.
  - `acquisition-api` restarted and became healthy.
  - `just krakend-sync`: pass; remote KrakenD active.
  - `just deploy-webspa-admin`: pass; remote `webspa-admin` became healthy.
  - Side-effect `notification-service` recreate from compose dependency also returned healthy.
  - Public `GET /api/v1/subscription/list?...` without JWT returns `401`, proving the gateway route now stops at auth instead of forwarding a headerless request to backend tenant-context rejection.
  - Public `GET /v1/admin/tenants/workspaces` without JWT returns `401`.
  - Public `GET /health` returns `200`.
  - Public `HEAD https://admin.nouveauricheglobalgroup.com/` returns `200`.
  - Deployed `webspa-admin` image: `sha256:fa74c92fe304bb4d37f8b8aa8ef65a7f5e9610b3873420d5c507c7b599bb790e`.
  - Deployed `acquisition-api` image unchanged: `sha256:5bd0f91a77ba0af0405c7a5eb08a4b3928efee48cf2910c11c61d6cecc78c194`.
  - Remote KrakenD template hash after sync: `1a4abe46fb4c4b1aed5218e7ba92029844fdc1089e77cd4311ce89779a104636`.

Rollback:
- Previous `webspa-admin` image: `sha256:57ae640f32e00ecaf8443d840c5b4e6db8f7c000c5c5b8c2f61b1680dc5b25cc`.
- Previous `acquisition-api` image: `sha256:5bd0f91a77ba0af0405c7a5eb08a4b3928efee48cf2910c11c61d6cecc78c194`.
- Previous KrakenD template hash: `f184247a287e41bb34d54a7e1aabe62e18d0a19d4324d59acf3b0a4b14e15927`.
- Previous rendered KrakenD hash: `15fa4975c3b8ffa003cc7591cd47525e2d01fda7d1f09d71e9dbd223dc8da8e3`.
- Restore remote env from `.env.bak-20260612T075703Z-subscription-menu`, restart `acquisition-api`, deploy previous webspa image or previous source commit, and rerun `krakend-sync` from the prior commit to roll back.

Peer review or blocked reason:
- No peer runtime was available in this turn; the fix was constrained to existing contracts and verified with regression tests.

Residual risk:
- The user must refresh/sign back in so the SPA re-reads workspace state and uses a fresh access token/session cache.
