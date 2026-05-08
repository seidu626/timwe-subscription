CREATE TABLE IF NOT EXISTS admin_subscription_action_logs (
    id UUID PRIMARY KEY,
    operation VARCHAR(20) NOT NULL,
    msisdn VARCHAR(50) NOT NULL,
    product_id INTEGER NOT NULL,
    partner_role_id INTEGER NOT NULL,
    external_tx_id VARCHAR(255),
    admin_request_id VARCHAR(255),
    request_method VARCHAR(10) NOT NULL,
    request_url TEXT NOT NULL,
    request_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_body JSONB,
    request_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    response_status_code INTEGER NOT NULL DEFAULT 0,
    response_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_body JSONB,
    response_timestamp TIMESTAMP WITH TIME ZONE,
    service_result JSONB,
    error_payload JSONB,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_operation_created_at
    ON admin_subscription_action_logs (operation, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_msisdn_created_at
    ON admin_subscription_action_logs (msisdn, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_external_tx_id
    ON admin_subscription_action_logs (external_tx_id);

CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_admin_request_id
    ON admin_subscription_action_logs (admin_request_id);
