-- Enforce globally-unique campaign slugs.
--
-- Public landing URLs are single-segment (/lp/{slug}, /api/campaigns/{slug}) and
-- created freely by users. The acquisition-api resolves the owning tenant
-- server-side from the slug (GET /v1/campaigns/{slug} -> GetEnabledBySlug), which
-- is only unambiguous if slugs are unique across tenants. The existing
-- idx_campaigns_tenant_slug only guarantees per-tenant uniqueness; this partial
-- unique index guarantees global uniqueness for tenant-owned campaigns.
--
-- Safe to apply: there are no cross-tenant slug collisions at creation time.
CREATE UNIQUE INDEX IF NOT EXISTS idx_campaigns_slug_global
  ON campaigns (slug)
  WHERE tenant_id IS NOT NULL;
