-- Tenant event delivery ledger. Endpoint configuration uses tenant_events credentials.
CREATE TABLE IF NOT EXISTS tenant_event_outbox (
    event_id VARCHAR(128) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    channel_id UUID REFERENCES tenant_channels(id) ON DELETE RESTRICT,
    source_notification_id BIGINT NOT NULL,
    source_type VARCHAR(50) NOT NULL,
    transaction_ref TEXT,
    payload JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_http_status INTEGER,
    last_error TEXT,
    response_body TEXT,
    optout_status VARCHAR(20),
    optout_attempts INTEGER NOT NULL DEFAULT 0,
    optout_next_attempt_at TIMESTAMPTZ,
    optout_outcome TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, event_id),
    CONSTRAINT tenant_event_status CHECK (status IN ('pending','sending','delivered','failed','dead','skipped')),
    CONSTRAINT tenant_event_optout_status CHECK (optout_status IS NULL OR optout_status IN ('pending','sending','done','failed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS tenant_event_source_unique
    ON tenant_event_outbox(tenant_id, source_notification_id);
CREATE UNIQUE INDEX IF NOT EXISTS tenant_event_charge_unique
    ON tenant_event_outbox(tenant_id, transaction_ref)
    WHERE source_type IN ('CHARGE','USER_RENEWED') AND transaction_ref IS NOT NULL;
CREATE INDEX IF NOT EXISTS tenant_event_due
    ON tenant_event_outbox(next_attempt_at, created_at)
    WHERE status IN ('pending','sending');
CREATE INDEX IF NOT EXISTS tenant_event_optout_due
    ON tenant_event_outbox(optout_next_attempt_at)
    WHERE optout_status IN ('pending','sending');

CREATE INDEX IF NOT EXISTS tenant_event_msisdn_order
    ON tenant_event_outbox(tenant_id, (payload->>'msisdn'), created_at, source_notification_id)
    WHERE status IN ('pending','sending');
