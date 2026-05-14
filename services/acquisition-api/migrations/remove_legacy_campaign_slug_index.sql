-- TMP-055: tenantless campaign rows have been backfilled to the canonical nrg tenant.
-- Keep historical migrations intact, but remove the runtime-only partial index that
-- made tenant_id IS NULL campaign slugs a supported ownership lane.

DROP INDEX IF EXISTS idx_campaigns_legacy_slug;
