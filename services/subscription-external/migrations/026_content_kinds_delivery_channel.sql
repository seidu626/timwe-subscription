-- Migration: content kinds and per-series delivery channel.
-- Additive columns keep existing TEXT and USER_PREF behavior for old rows.

BEGIN;

ALTER TABLE message_content_items
    ADD COLUMN IF NOT EXISTS content_kind TEXT NOT NULL DEFAULT 'TEXT' CHECK (content_kind IN ('TEXT', 'LINK')),
    ADD COLUMN IF NOT EXISTS link_url TEXT,
    ADD COLUMN IF NOT EXISTS cta_label TEXT;

ALTER TABLE product_message_series
    ADD COLUMN IF NOT EXISTS delivery_channel TEXT NOT NULL DEFAULT 'USER_PREF' CHECK (delivery_channel IN ('USER_PREF', 'SMS', 'PUSH'));

ALTER TABLE message_outbox
    DROP CONSTRAINT IF EXISTS message_outbox_content_source_check;

ALTER TABLE message_outbox
    ADD CONSTRAINT message_outbox_content_source_check CHECK (
        (series_id IS NOT NULL AND content_item_id IS NOT NULL)
        OR
        (series_id IS NULL AND content_item_id IS NULL AND length(btrim(message_text)) > 0)
    ) NOT VALID;

ALTER TABLE message_outbox
    VALIDATE CONSTRAINT message_outbox_content_source_check;

COMMIT;
