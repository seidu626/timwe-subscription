# Tenant Nullability Enforcement Plan

TMP-052 audited remaining `tenant_id IS NULL` paths after the canonical `nrg` migration. The enforcement plan is forward-only:

1. Keep TMP-050 migration script and runbook predicates as the operator proof surface.
2. Prove acquisition/admin table groups have zero tenantless rows.
3. Prove subscription/cadence table groups have zero tenantless rows.
4. Collapse runtime nullable joins and lookups into tenant-aware canonical paths.
5. Add forward migrations for NOT NULL constraints and legacy partial-index cleanup after proof.

Existing migrations must not be rewritten to pretend historical nullable columns never existed. Cleanup belongs in new migrations that can run against the live schema safely.
