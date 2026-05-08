-- Tenant campaign ownership and channel binding.
-- The filename intentionally sorts after add_tenant_channels.sql.

ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

CREATE UNIQUE INDEX IF NOT EXISTS idx_campaigns_tenant_id_id
    ON campaigns (tenant_id, id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'campaigns_tenant_channel_fk'
    ) THEN
        ALTER TABLE campaigns
            ADD CONSTRAINT campaigns_tenant_channel_fk
            FOREIGN KEY (tenant_id, channel_id)
            REFERENCES tenant_channels (tenant_id, id)
            ON DELETE RESTRICT;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_campaigns_tenant_slug
    ON campaigns (tenant_id, slug)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_campaigns_legacy_slug
    ON campaigns (slug)
    WHERE tenant_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_campaigns_tenant_enabled
    ON campaigns (tenant_id, enabled, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_campaigns_channel
    ON campaigns (tenant_id, channel_id);
