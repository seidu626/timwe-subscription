-- Encrypted secret store for tenant channel credentials.
-- Stores AES-256-GCM ciphertext; the nonce (12 bytes) is prepended to the
-- ciphertext in the single `ciphertext` column: stored = nonce || ciphertext.
-- Secret references in tenant_channel_credentials use: secret://<id>

CREATE TABLE IF NOT EXISTS tenant_channel_secrets (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    channel_id   UUID NOT NULL,
    purpose      VARCHAR(80) NOT NULL DEFAULT 'provider_api',
    ciphertext   BYTEA NOT NULL,
    key_version  INTEGER NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_tenant_channel_secrets_purpose
        CHECK (purpose ~ '^[a-z0-9][a-z0-9_-]{1,78}[a-z0-9]$'),
    CONSTRAINT chk_tenant_channel_secrets_key_version
        CHECK (key_version > 0)
);

CREATE INDEX IF NOT EXISTS idx_tenant_channel_secrets_tenant_channel
    ON tenant_channel_secrets (tenant_id, channel_id, purpose, created_at DESC);
