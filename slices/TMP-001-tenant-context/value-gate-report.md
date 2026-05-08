# TMP-001 Value Gate Report

Verdict: PASS

Timestamp: 2026-05-08T04:33:00Z
Agent: codex

## Audit 1: Acceptance Criteria Coverage

- Criterion: Platform operator creates tenant
  - Test files: `services/acquisition-api/internal/handler/admin_management_tenant_test.go`, `services/acquisition-api/internal/repository/admin_management_repository_test.go`
  - Test names: `TestCreateTenantReturnsCreatedTenantAndAuditReference`, `TestCreateTenantWithActivityLogCommitsTenantAndAudit`
  - Assertion type: 201 response body, normalized tenant key, audit log id, tenant row, and audit insert in the same transaction
  - Verdict: COVERED
- Criterion: Current tenant resolves from accepted admin-auth identity
  - Test files: `services/acquisition-api/internal/service/admin_management_service_test.go`, `services/acquisition-api/internal/handler/admin_management_tenant_test.go`
  - Test names: `TestResolveCurrentTenantHidesInactiveTenant`, `TestGetCurrentTenantHidesInactiveAndUnknownTenants`
  - Assertion type: tenant identity lookup by accepted `tenantctx.Identity` and fail-closed unavailable behavior
  - Verdict: COVERED
- Criterion: Missing tenant context performs no fallback
  - Test file: `services/acquisition-api/internal/handler/admin_management_tenant_test.go`
  - Test name: `TestGetCurrentTenantDoesNotTrustRawTenantHeader`
  - Assertion type: raw `X-Tenant-Id` is ignored without middleware identity and returns 403
  - Verdict: COVERED
- Criterion: Duplicate tenant key returns conflict without SQL leakage
  - Test file: `services/acquisition-api/internal/repository/admin_management_repository_test.go`
  - Test name: `TestCreateTenantWithActivityLogMapsDuplicateKeyConflict`
  - Assertion type: PostgreSQL unique violation maps to domain conflict error
  - Verdict: COVERED
- Criterion: Tenant admin cannot create tenants
  - Test files: `services/acquisition-api/internal/service/admin_management_service_test.go`, `services/acquisition-api/internal/handler/admin_management_tenant_test.go`
  - Test names: `TestCreateTenantRequiresPlatformScope`, `TestCreateTenantRejectsTenantScopedAdmin`
  - Assertion type: 403/forbidden with no repository mutation
  - Verdict: COVERED

## Audit 2: Failure Mode Coverage

- invalid_input: tenant create validation covers key normalization, required name, status, country, JSON-object metadata, and metadata size.
- missing_required: missing accepted tenant context and tenant-scoped create authorization are covered.
- duplicate_conflict: duplicate tenant key maps to `ErrAdminConflict` and handler maps conflict to 409.
- dependency_failure: audit insert failure rolls back tenant create through a single repository transaction.
- authorization: platform scope is required for tenant creation; raw tenant headers are not trusted on direct admin service routes.

Verdict: PASS

## Audit 3: Domain Invariant Preservation

- Tenant creation is platform-only
  - Positive: platform identity create path returns 201.
  - Negative: tenant-scoped admin create returns 403.
  - Verdict: PRESERVED
- Tenant key is normalized and unique
  - Positive: handler test returns normalized `tenant-a`.
  - Negative: duplicate key test maps unique violation to conflict.
  - Verdict: PRESERVED
- Tenant create is auditable atomically
  - Positive: tenant insert and audit insert commit together.
  - Negative: audit failure triggers rollback.
  - Verdict: PRESERVED
- No global fallback tenant behavior
  - Positive: current tenant lookup uses accepted `tenantctx.Identity`.
  - Negative: raw tenant header without identity returns 403 and performs no lookup.
  - Verdict: PRESERVED
- Inactive and unknown tenant-scoped lookups do not disclose existence differences
  - Positive: active lookup path is implemented by service.
  - Negative: inactive and unknown handler responses share status/body shape.
  - Verdict: PRESERVED

## Audit 4: User Journey Completeness

- Platform operator posts tenant create payload: implemented by `POST /v1/admin/tenants` route, handler, service, repository, migration, and tests.
- Tenant row and audit row are committed together: implemented in `CreateTenantWithActivityLog`; tested with commit and rollback paths.
- Tenant admin resolves current tenant from JWT-derived identity: implemented by `GET /v1/admin/tenants/current`; tested for accepted identity, missing context, inactive, and unknown paths.
- Raw/gateway tenant header ambiguity: direct service does not trust raw tenant headers; tested.

Verdict: PASS

## Audit 5: Test Quality

Scanner unavailable: `scripts/scan-test-quality.sh` is not present in this repository. Manual anti-pattern check performed for new tests:

- Assertion-free tests: 0 observed.
- Status-only assertions: 0 status-only tests; status checks include body, identity, repository, or transaction assertions.
- No negative tests: negative tests cover tenant-scoped create, raw-header fallback, duplicate key, inactive/unknown parity, missing context, invalid metadata, and audit rollback.
- Mock everything: repository tests exercise SQL transaction contracts with sqlmock; handler tests exercise real handler/service/repository code against sqlmock.

Verdict: PASS

## Verification Commands

- `cd services/acquisition-api && go test ./internal/handler ./internal/service ./internal/repository ./internal/transport`
- `cd services/acquisition-api && go test ./...`
- `git diff --check`
- `jq -e '.slices[] | select(.id=="TMP-001")' slices/manifest.json`

## Follow-Up Risks

- TMP-001 intentionally does not tenant-scope products, campaigns, reports, subscriptions, or callbacks; those remain owned by later slices.
- Auth0 Organizations and tenant membership modeling remain open design work for TMP-002 and admin portal slices.
- Gateway-injected trusted headers are not accepted on direct acquisition admin routes in this slice; TMP-012 owns public/gateway tenant routing.
