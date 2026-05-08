-- Tenant acquisition transaction ownership.
-- Runs after tenant campaign binding so tenant-owned duplicate slugs are safe.

ALTER TABLE acquisition_transactions
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE RESTRICT;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'acquisition_transactions_campaign_slug_fkey'
    ) THEN
        ALTER TABLE acquisition_transactions
            DROP CONSTRAINT acquisition_transactions_campaign_slug_fkey;
    END IF;
END $$;

ALTER TABLE campaigns
    DROP CONSTRAINT IF EXISTS campaigns_slug_key;

DROP INDEX IF EXISTS idx_campaigns_slug;

CREATE INDEX IF NOT EXISTS idx_acq_trans_tenant_campaign_msisdn
    ON acquisition_transactions (tenant_id, campaign_slug, msisdn, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_acq_trans_tenant_status
    ON acquisition_transactions (tenant_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_acq_trans_tenant_click
    ON acquisition_transactions (tenant_id, ad_provider, click_id, created_at DESC);
