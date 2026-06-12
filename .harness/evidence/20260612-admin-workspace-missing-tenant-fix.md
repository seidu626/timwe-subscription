# Admin Workspace Missing-Tenant Fix Evidence

Date: 2026-06-12
WorkOrder: `WO-FIX-ADMIN-WEBSPA-TENANT-WORKSPACE-AUTHORIZATION`
Surface: `frontend/webspa-admin`
Route: `/#/403?reason=missing-tenant`
API: `GET /v1/admin/tenants/workspaces`

## Debug Ledger

Symptom:

- The admin SPA rendered `Workspace authorization error` at `/#/403?reason=missing-tenant`.
- The browser network panel showed `GET /v1/admin/tenants/workspaces` returning HTTP 200.

Reproduction:

- Added a focused frontend regression matching the live authorization shape: `platform_scoped: false` with two authorized backend tenant workspaces: `nrg` and `careerify`.
- Red test: `.harness/logs/20260612-admin-workspace-service-test-red-headless.log`.
- The failing state stayed `missing-tenant`, `canSwitchTenant` stayed false, and selecting `careerify` failed.

Operator correction count: 0

Current evidence:

- Live DB membership query, redacted: `.harness/logs/20260612-admin-workspace-db-subject.tsv`.
- The account has active `TENANT_ADMIN` memberships for `nrg` and `careerify`.
- Backend service contract returns authorized tenant workspaces from `tenant_admin_memberships` for non-platform users.

Hypotheses:

1. Backend did not assign the user to a tenant. Refuted by live DB membership query.
2. Workspace endpoint returned an unparsable shape. Not supported by local backend/domain contract and existing parser coverage for `{platform_scoped, tenants}`.
3. Frontend state machine treated multi-tenant selection as platform-only. Confirmed by the red regression.

Result:

- Confirmed frontend logic bug.

Root cause:

- `TenantWorkspaceService` only allowed tenant selection/switching when `platformScoped` was true.
- A non-platform user with multiple explicit tenant memberships therefore had `availableTenants` populated, but no `currentTenant`, no switch capability, and a `missing-tenant` status.
- The 403 page reconciled that contradictory state into `Workspace authorization error`.

Patch plan:

- Let selection operate over the authorized `availableTenants` list for all users.
- Keep `platformScoped` as platform-scope metadata only; do not use it as the permission gate for selecting an explicitly assigned tenant.
- Require explicit selection whenever more than one authorized tenant workspace exists.

Verification:

- `npm test -- --watch=false --browsers=ChromeHeadlessNoSandbox --include src/app/core/services/tenant-workspace.service.spec.ts`
- `npm test -- --watch=false --browsers=ChromeHeadlessNoSandbox --include src/app/views/pages/page403/page403.component.spec.ts --include src/app/core/guards/tenant-workspace.guard.spec.ts --include src/app/core/http-interceptors/tenant-workspace.interceptor.spec.ts`
- `npm run build:prod`

Residual risk:

- The production SPA must be redeployed before the live admin URL changes behavior.
- No live bearer token was written to repo artifacts.

## Verification Logs

- `.harness/logs/20260612-admin-workspace-service-test-red-headless.log`
- `.harness/logs/20260612-admin-workspace-service-test-green.log`
- `.harness/logs/20260612-admin-workspace-route-contract-tests.log`
- `.harness/logs/20260612-admin-workspace-build-prod.log`
- `.harness/logs/20260612-admin-workspace-db-subject.tsv`
