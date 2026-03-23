-- Migration: Create postback outbox and attempts tables
-- These tables belong to the acquisition-api service boundary.
-- Uses IF NOT EXISTS for idempotency (safe to run if tables were already
-- created by the subscription-external 006 migration on a shared database).

-- ---------------------------------------------------------------------------
-- postback_outbox: queued postbacks for async delivery
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS postback_outbox (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id        UUID NOT NULL,
    event                 VARCHAR(50) NOT NULL,
    provider              VARCHAR(50) NOT NULL,
    url_template_rendered TEXT NOT NULL,
    http_method           VARCHAR(10) DEFAULT 'POST',
    headers               TEXT,
    body                  TEXT,

    -- Retry tracking
    attempt_count         INTEGER DEFAULT 0,
    max_attempts          INTEGER DEFAULT 5,
    next_retry_at         TIMESTAMPTZ,
    status                VARCHAR(20) DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED', 'DLQ')),

    -- Timestamps
    created_at            TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at            TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (transaction_id)
        REFERENCES acquisition_transactions(id) ON DELETE CASCADE
);

-- Composite index used by the dispatcher claim query
-- (WHERE status = 'PENDING' AND next_retry_at <= now())
CREATE INDEX IF NOT EXISTS idx_postback_outbox_status_retry
    ON postback_outbox (status, next_retry_at);

-- Admin / debugging lookup by transaction
CREATE INDEX IF NOT EXISTS idx_postback_outbox_transaction
    ON postback_outbox (transaction_id);

-- ---------------------------------------------------------------------------
-- postback_attempts: immutable log of every delivery attempt
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS postback_attempts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outbox_id        UUID NOT NULL,
    attempt_number   INTEGER NOT NULL,
    http_status      INTEGER,
    response_body    TEXT,
    error_message    TEXT,
    duration_ms      INTEGER,
    created_at       TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (outbox_id)
        REFERENCES postback_outbox(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_postback_attempts_outbox
    ON postback_attempts (outbox_id);

-- ---------------------------------------------------------------------------
-- updated_at trigger (create function if not yet present)
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- The trigger name is scoped to the table, so IF NOT EXISTS isn't available
-- for CREATE TRIGGER in all PG versions. Use a DO block instead.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgname = 'trg_postback_outbox_updated_at'
    ) THEN
        CREATE TRIGGER trg_postback_outbox_updated_at
            BEFORE UPDATE ON postback_outbox
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
END;
$$;

-- ---------------------------------------------------------------------------
-- Ensure CHARGED is an allowed status on acquisition_transactions
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    -- Attempt to add CHARGED to the CHECK constraint.
    -- If the constraint already includes it (or uses a different mechanism),
    -- the block catches the error and proceeds.
    ALTER TABLE acquisition_transactions
        DROP CONSTRAINT IF EXISTS acquisition_transactions_status_check;

    ALTER TABLE acquisition_transactions
        ADD CONSTRAINT acquisition_transactions_status_check
        CHECK (status IN (
            'PENDING', 'ACTION_REQUIRED', 'CONFIRM_REQUIRED',
            'SUBSCRIBED', 'FAILED', 'CANCELLED', 'CHARGED'
        ));
EXCEPTION
    WHEN others THEN
        RAISE NOTICE 'Could not update acquisition_transactions status constraint: %', SQLERRM;
END;
$$;
