-- Migration: Message Cadence Engine (series + schedule + state + outbox)
-- File: migrations/011_message_cadence_engine.sql

BEGIN;

-- 1) Content series per product
CREATE TABLE IF NOT EXISTS product_message_series (
    id              BIGSERIAL PRIMARY KEY,
    partner_role_id INTEGER NOT NULL,
    product_id      INTEGER NOT NULL,
    name            TEXT NOT NULL,
    mode            TEXT NOT NULL DEFAULT 'SEQUENTIAL' CHECK (mode IN ('SEQUENTIAL', 'POOL')),
    content_version INTEGER NOT NULL DEFAULT 1,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (partner_role_id, product_id, name)
);

-- 2) Schedule rules per series
CREATE TABLE IF NOT EXISTS message_schedule_rules (
    series_id       BIGINT PRIMARY KEY REFERENCES product_message_series(id) ON DELETE CASCADE,
    rule_kind       TEXT NOT NULL CHECK (rule_kind IN ('DAILY', 'WEEKLY', 'EVERY_N_DAYS')),
    preferred_time  TIME NOT NULL,
    days_of_week    SMALLINT NULL,
    n_days          INT NULL,
    send_start_time TIME NOT NULL DEFAULT '08:00',
    send_end_time   TIME NOT NULL DEFAULT '20:00',
    timezone        TEXT NOT NULL DEFAULT 'Africa/Accra',
    max_per_day     INT NOT NULL DEFAULT 1,
    catchup_mode    TEXT NOT NULL DEFAULT 'THROTTLE' CHECK (catchup_mode IN ('SEND', 'SKIP', 'THROTTLE'))
);

-- 3) Content items (authored ahead of time)
CREATE TABLE IF NOT EXISTS message_content_items (
    id              BIGSERIAL PRIMARY KEY,
    series_id       BIGINT NOT NULL REFERENCES product_message_series(id) ON DELETE CASCADE,
    content_version INTEGER NOT NULL,
    seq_no          INT NULL,
    message_text    TEXT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (series_id, content_version, seq_no)
);

-- 4) Per-subscription state (cursor + next_send_at)
CREATE TABLE IF NOT EXISTS subscription_message_state (
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    series_id       BIGINT NOT NULL REFERENCES product_message_series(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'PAUSED', 'STOPPED')),
    cursor_seq      INT NOT NULL DEFAULT 1,
    next_send_at    TIMESTAMPTZ NOT NULL,
    last_sent_at    TIMESTAMPTZ NULL,
    inflight_job_id UUID NULL,
    inflight_until  TIMESTAMPTZ NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (subscription_id, series_id)
);

CREATE INDEX IF NOT EXISTS idx_sms_due
    ON subscription_message_state (status, next_send_at);

CREATE INDEX IF NOT EXISTS idx_sms_inflight
    ON subscription_message_state (inflight_until)
    WHERE inflight_until IS NOT NULL;

-- 5) Outbox for idempotent dispatch
CREATE TABLE IF NOT EXISTS message_outbox (
    job_id           UUID PRIMARY KEY,
    idempotency_key  TEXT NOT NULL UNIQUE,
    subscription_id  INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    series_id        BIGINT NOT NULL REFERENCES product_message_series(id) ON DELETE CASCADE,
    content_item_id  BIGINT NOT NULL REFERENCES message_content_items(id),
    planned_send_at  TIMESTAMPTZ NOT NULL,
    status           TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SENT', 'FAILED', 'RETRYING')),
    attempt          INT NOT NULL DEFAULT 0,
    last_error       TEXT NULL,
    sent_at          TIMESTAMPTZ NULL,
    processed_at     TIMESTAMPTZ NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON message_outbox (status, planned_send_at);

CREATE INDEX IF NOT EXISTS idx_outbox_processed
    ON message_outbox (status, processed_at);

-- 6) updated_at trigger (shared helper)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_subscription_message_state_updated_at ON subscription_message_state;
CREATE TRIGGER update_subscription_message_state_updated_at
    BEFORE UPDATE ON subscription_message_state
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_message_outbox_updated_at ON message_outbox;
CREATE TRIGGER update_message_outbox_updated_at
    BEFORE UPDATE ON message_outbox
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMIT;
