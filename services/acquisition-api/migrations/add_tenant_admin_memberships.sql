CREATE TABLE IF NOT EXISTS tenant_admin_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    auth0_subject TEXT NOT NULL,
    email TEXT,
    role TEXT NOT NULL DEFAULT 'TENANT_ADMIN',
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_tenant_admin_memberships_subject_nonempty CHECK (length(trim(auth0_subject)) > 0),
    CONSTRAINT chk_tenant_admin_memberships_role CHECK (role IN ('TENANT_ADMIN', 'TENANT_VIEWER')),
    CONSTRAINT chk_tenant_admin_memberships_status CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_admin_memberships_tenant_subject
    ON tenant_admin_memberships (tenant_id, auth0_subject);

CREATE INDEX IF NOT EXISTS idx_tenant_admin_memberships_subject_active
    ON tenant_admin_memberships (auth0_subject, status)
    WHERE status = 'ACTIVE';

CREATE INDEX IF NOT EXISTS idx_tenant_admin_memberships_email_active
    ON tenant_admin_memberships (lower(email), status)
    WHERE email IS NOT NULL AND status = 'ACTIVE';
