-- Per-product subscription counts in the admin console aggregate over
-- (tenant_id, product_id, status). The existing tenant index leads with
-- channel_id, which forces a full per-tenant scan for these counts.
-- Apply with CONCURRENTLY outside a transaction on live databases:
--   CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscriptions_tenant_product_status
--     ON subscriptions (tenant_id, product_id, status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_product_status
    ON subscriptions (tenant_id, product_id, status);
