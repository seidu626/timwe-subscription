-- slice-harness: allow-new-canonical-path: TMP-007 stores tenant/channel ownership for subscription routing.
-- Tenant/channel ownership for subscription-external routing. Columns are nullable so legacy
-- global rows continue to operate until TMP-011 migrates default-tenant data.

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

ALTER TABLE admin_subscription_action_logs
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS channel_id UUID;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_tenant_identity
    ON subscriptions (tenant_id, partner_role_id, user_identifier, product_id)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_channel_status
    ON subscriptions (tenant_id, channel_id, status, created_at DESC)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_tenant_channel_created
    ON notifications (tenant_id, channel_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_tenant_created
    ON admin_subscription_action_logs (tenant_id, channel_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;
