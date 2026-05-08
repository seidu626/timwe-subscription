-- slice-harness: allow-new-canonical-path: TMP-008 tenant/channel notification and cadence routing.
-- Nullable columns preserve legacy global rows until TMP-011 backfills default tenant data.

ALTER TABLE product_message_series
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

ALTER TABLE message_content_items
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

ALTER TABLE subscription_message_state
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

ALTER TABLE message_outbox
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

ALTER TABLE product_message_series
    DROP CONSTRAINT IF EXISTS product_message_series_partner_role_id_product_id_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_message_series_tenant_key
    ON product_message_series (tenant_id, partner_role_id, product_id, name)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_message_series_legacy_key
    ON product_message_series (partner_role_id, product_id, name)
    WHERE tenant_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_product_message_series_tenant_channel
    ON product_message_series (tenant_id, channel_id, is_active, created_at DESC)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_message_content_items_tenant_series
    ON message_content_items (tenant_id, channel_id, series_id, content_version)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subscription_message_state_tenant_due
    ON subscription_message_state (tenant_id, channel_id, status, next_send_at)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_message_outbox_tenant_status
    ON message_outbox (tenant_id, channel_id, status, planned_send_at)
    WHERE tenant_id IS NOT NULL;
