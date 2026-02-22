-- Admin management audit and import history tables

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
