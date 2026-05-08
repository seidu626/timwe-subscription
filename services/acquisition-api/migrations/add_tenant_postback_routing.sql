-- TMP-009: Tenant and channel ownership for postback routing.

ALTER TABLE postback_outbox
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID,
    ADD COLUMN IF NOT EXISTS failure_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_postback_outbox_tenant_status_retry
    ON postback_outbox (tenant_id, status, next_retry_at, created_at)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_postback_outbox_tenant_transaction
    ON postback_outbox (tenant_id, transaction_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_postback_outbox_tenant_channel
    ON postback_outbox (tenant_id, channel_id, created_at DESC)
    WHERE tenant_id IS NOT NULL AND channel_id IS NOT NULL;
