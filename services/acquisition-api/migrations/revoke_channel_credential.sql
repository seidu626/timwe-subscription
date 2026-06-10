-- Add REVOKED status and purged_at audit marker to tenant_channel_credentials.
-- crypto-erasure: when a credential is revoked the ciphertext row is deleted
-- from tenant_channel_secrets; purged_at records when that happened.

ALTER TABLE tenant_channel_credentials
    DROP CONSTRAINT IF EXISTS chk_tenant_channel_credentials_status;

ALTER TABLE tenant_channel_credentials
    ADD CONSTRAINT chk_tenant_channel_credentials_status
        CHECK (status IN ('ACTIVE', 'INACTIVE', 'REVOKED'));

ALTER TABLE tenant_channel_credentials
    ADD COLUMN IF NOT EXISTS purged_at TIMESTAMPTZ;
