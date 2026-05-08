-- Tenant channel catalog for multi-channel routing.

CREATE TABLE IF NOT EXISTS tenant_channels (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    channel_key VARCHAR(120) NOT NULL,
    provider VARCHAR(80) NOT NULL,
    country VARCHAR(2) NOT NULL,
    operator VARCHAR(120),
    capabilities TEXT[] NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_tenant_channels_key_format CHECK (channel_key ~ '^[a-z0-9][a-z0-9_-]{1,118}[a-z0-9]$'),
    CONSTRAINT chk_tenant_channels_country CHECK (country ~ '^[A-Z]{2}$'),
    CONSTRAINT chk_tenant_channels_status CHECK (status IN ('ACTIVE', 'INACTIVE')),
    CONSTRAINT chk_tenant_channels_capabilities_nonempty CHECK (array_length(capabilities, 1) > 0),
    CONSTRAINT chk_tenant_channels_capabilities_allowed CHECK (capabilities <@ ARRAY['optin','confirm','mt','charge']::text[])
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channels_tenant_key
    ON tenant_channels (tenant_id, channel_key);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channels_tenant_provider_scope
    ON tenant_channels (tenant_id, provider, country, COALESCE(operator, ''));

CREATE INDEX IF NOT EXISTS idx_tenant_channels_tenant_status_created
    ON tenant_channels (tenant_id, status, created_at DESC);
