# Claude Async Review Synthesis

Claude's independent read-only review agreed with the overall direction, but found that the first amended plan still left several load-bearing platform decisions too implicit.

## Integrated Findings

1. Tenant claim and service-to-service auth cannot remain a detail inside TMP-001. Added `TMP-018-tenant-claim-and-service-auth-contract`.
2. Public routing was correctly added as TMP-012, but it depends on the tenant claim/trust-boundary contract. Updated dependencies and roadmap sequencing.
3. Campaign assets and landing media need tenant-scoped object-storage namespacing before campaign work is complete. Added `TMP-019-tenant-asset-namespacing` and made TMP-005 depend on it.
4. Observability must start earlier than production hardening. Added `TMP-020-tenant-observability-baseline`; TMP-015 remains the later ops/secret hardening slice.
5. Value-gate evidence must be named and automatable. Updated `value-gate-plan.md` to require file/function evidence and positive/negative invariant tests.

## Findings Recorded For Implementation Sizing

These findings should influence backlog splitting during implementation:

- TMP-007 may need to split inbound partner auth from outbound provider credential resolution.
- TMP-008 may need to split notification tenant-scope from cadence tenant-scope.
- TMP-011 should be treated as a migration program: nullable tenant columns and dual-read flags early, table-by-table backfill with each feature slice, then final `NOT NULL` enforcement and fallback retirement.
- HE bootstrap tenant scoping must be explicitly tested inside TMP-006 or split if it grows.
- Billing ownership must be decided before charge-capable channel reporting claims production readiness.

## Deferred Decisions To Capture As ADRs

- Tenant claim model: Auth0 Organizations, custom claim, or hybrid.
- Tenant isolation model: shared DB plus repository enforcement, PostgreSQL RLS, schema-per-tenant, or database-per-tenant.
- Public tenant routing: host, path, signed token, or gateway host map.
- Secret backend: external vault/secret manager, encrypted DB reference, or staged adapter.
- Service-to-service auth: gateway-signed header, HMAC, mTLS, or service account JWT.
