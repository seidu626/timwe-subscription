-- 031: track true handset delivery for gateway-sent SMS jobs.
-- SENT has only ever meant "the SMS gateway accepted the message"; observed
-- 2026-08-19 on prod: Arkesel accepted a welcome SMS (job 2da283e3) that never
-- reached the handset. Store the provider's message id at send time so a
-- poller can resolve the real delivery outcome afterwards.
ALTER TABLE message_outbox
    ADD COLUMN IF NOT EXISTS provider_message_id TEXT,
    ADD COLUMN IF NOT EXISTS delivery_status TEXT,
    ADD COLUMN IF NOT EXISTS delivery_detail TEXT,
    ADD COLUMN IF NOT EXISTS delivery_checked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;

-- Partial index for the delivery poller's claim query: only SENT jobs that
-- carry a provider id and are still awaiting a terminal delivery verdict.
CREATE INDEX IF NOT EXISTS idx_message_outbox_delivery_poll
    ON message_outbox (sent_at)
    WHERE status = 'SENT'
      AND provider_message_id IS NOT NULL
      AND (delivery_status IS NULL OR delivery_status = 'PENDING');
