-- Admin management audit and import history tables

CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY,
    tenant_key VARCHAR(100) NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    default_country VARCHAR(2) NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_tenants_status CHECK (status IN ('ACTIVE', 'INACTIVE')),
    CONSTRAINT chk_tenants_key_format CHECK (tenant_key ~ '^[a-z0-9][a-z0-9_-]{1,98}[a-z0-9]$'),
    CONSTRAINT chk_tenants_default_country CHECK (default_country ~ '^[A-Z]{2}$')
);

CREATE INDEX IF NOT EXISTS idx_tenants_status
    ON tenants (status, created_at DESC);

CREATE TABLE IF NOT EXISTS admin_activity_logs (
    id UUID PRIMARY KEY,
    entity_type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    actor VARCHAR(255),
    request_id VARCHAR(255),
    before_json JSONB,
    after_json JSONB,
    metadata_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_activity_logs_entity
    ON admin_activity_logs (entity_type, entity_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_activity_logs_actor
    ON admin_activity_logs (actor, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_activity_logs_action
    ON admin_activity_logs (action, created_at DESC);

CREATE TABLE IF NOT EXISTS userbase_import_jobs (
    id UUID PRIMARY KEY,
    filename TEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    total_rows INTEGER NOT NULL DEFAULT 0,
    success_rows INTEGER NOT NULL DEFAULT 0,
    failed_rows INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_by VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_userbase_import_jobs_started_at
    ON userbase_import_jobs (started_at DESC);

CREATE TABLE IF NOT EXISTS userbase_import_errors (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES userbase_import_jobs(id) ON DELETE CASCADE,
    row_number INTEGER NOT NULL,
    raw_row TEXT,
    error_message TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_userbase_import_errors_job_id
    ON userbase_import_errors (job_id, id);

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

ALTER TABLE userbase
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

ALTER TABLE userbase_import_jobs
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

ALTER TABLE userbase_import_errors
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

ALTER TABLE admin_activity_logs
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_product_id_key;

ALTER TABLE userbase
    DROP CONSTRAINT IF EXISTS userbase_msisdn_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_products_tenant_product_id
    ON products (tenant_id, product_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_userbase_tenant_msisdn
    ON userbase (tenant_id, msisdn);

CREATE INDEX IF NOT EXISTS idx_userbase_import_jobs_tenant_started
    ON userbase_import_jobs (tenant_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_activity_logs_tenant_entity
    ON admin_activity_logs (tenant_id, entity_type, entity_id, created_at DESC);
