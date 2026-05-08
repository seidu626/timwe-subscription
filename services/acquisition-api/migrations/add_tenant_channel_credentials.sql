-- Tenant channel credential references. This table stores references only, never secret values.

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channels_tenant_id_id
    ON tenant_channels (tenant_id, id);

CREATE TABLE IF NOT EXISTS tenant_channel_credentials (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    channel_id UUID NOT NULL,
    purpose VARCHAR(80) NOT NULL DEFAULT 'provider_api',
    version INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    secret_ref TEXT NOT NULL,
    secret_ref_display TEXT NOT NULL,
    secret_fingerprint TEXT NOT NULL,
    created_by VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_at TIMESTAMPTZ,
    deactivated_at TIMESTAMPTZ,
    CONSTRAINT fk_tenant_channel_credentials_channel
        FOREIGN KEY (tenant_id, channel_id)
        REFERENCES tenant_channels (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_tenant_channel_credentials_status CHECK (status IN ('ACTIVE', 'INACTIVE')),
    CONSTRAINT chk_tenant_channel_credentials_version CHECK (version > 0),
    CONSTRAINT chk_tenant_channel_credentials_purpose CHECK (purpose ~ '^[a-z0-9][a-z0-9_-]{1,78}[a-z0-9]$'),
    CONSTRAINT chk_tenant_channel_credentials_secret_ref CHECK (
        secret_ref LIKE 'vault://%' OR
        secret_ref LIKE 'aws-sm://%' OR
        secret_ref LIKE 'gcp-sm://%' OR
        secret_ref LIKE 'azure-kv://%' OR
        secret_ref LIKE 'secret://%' OR
        secret_ref LIKE 'env://%'
    ),
    CONSTRAINT chk_tenant_channel_credentials_display CHECK (length(trim(secret_ref_display)) > 0),
    CONSTRAINT chk_tenant_channel_credentials_fingerprint CHECK (length(secret_fingerprint) = 64)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channel_credentials_version
    ON tenant_channel_credentials (tenant_id, channel_id, purpose, version);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channel_credentials_active
    ON tenant_channel_credentials (tenant_id, channel_id, purpose)
    WHERE status = 'ACTIVE';

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channel_credentials_fingerprint
    ON tenant_channel_credentials (tenant_id, channel_id, purpose, secret_fingerprint);

CREATE INDEX IF NOT EXISTS idx_tenant_channel_credentials_channel_created
    ON tenant_channel_credentials (tenant_id, channel_id, created_at DESC);
