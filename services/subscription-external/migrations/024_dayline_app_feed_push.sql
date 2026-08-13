-- Migration: Dayline app feed, devices, notification prefs, PUSH channel routing.
-- Owner: subscription-external. See docs/dayline-app-api-contract.md.
-- Additive only: nullable new columns/tables, NOT VALID + VALIDATE for the new
-- CHECK constraint so existing rows are never rewritten under an exclusive lock.

BEGIN;

-- Rich feed titles: nullable, fallback derived client/server-side from the
-- first 60 chars of message_text when absent.
ALTER TABLE message_content_items
    ADD COLUMN IF NOT EXISTS title TEXT;

-- Delivery channel per outbox job. Nullable, defaults to 'SMS' for all new
-- rows; existing rows stay NULL (treated as SMS by the dispatcher).
ALTER TABLE message_outbox
    ADD COLUMN IF NOT EXISTS channel TEXT DEFAULT 'SMS';

ALTER TABLE message_outbox
    ADD CONSTRAINT message_outbox_channel_check
    CHECK (channel IS NULL OR channel IN ('SMS', 'PUSH')) NOT VALID;

ALTER TABLE message_outbox
    VALIDATE CONSTRAINT message_outbox_channel_check;

-- Read state for feed items, keyed by msisdn (the Dayline app JWT `sub`).
CREATE TABLE IF NOT EXISTS app_feed_read_state (
    msisdn          TEXT NOT NULL,
    content_item_id BIGINT NOT NULL REFERENCES message_content_items(id) ON DELETE CASCADE,
    read_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (msisdn, content_item_id)
);

-- Registered push devices. One row per fcm_token; re-registering the same
-- token (e.g. app reinstall on the same device) upserts in place.
CREATE TABLE IF NOT EXISTS app_devices (
    id          BIGSERIAL PRIMARY KEY,
    msisdn      TEXT NOT NULL,
    tenant_key  TEXT NOT NULL,
    fcm_token   TEXT NOT NULL UNIQUE,
    platform    TEXT NOT NULL CHECK (platform IN ('android', 'ios')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_devices_msisdn
    ON app_devices (msisdn);

DROP TRIGGER IF EXISTS update_app_devices_updated_at ON app_devices;
CREATE TRIGGER update_app_devices_updated_at
    BEFORE UPDATE ON app_devices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Per-subscriber, per-product notification channel preference.
CREATE TABLE IF NOT EXISTS app_notification_prefs (
    msisdn       TEXT NOT NULL,
    product_slug TEXT NOT NULL,
    channel      TEXT NOT NULL DEFAULT 'SMS' CHECK (channel IN ('PUSH', 'SMS', 'BOTH')),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (msisdn, product_slug)
);

DROP TRIGGER IF EXISTS update_app_notification_prefs_updated_at ON app_notification_prefs;
CREATE TRIGGER update_app_notification_prefs_updated_at
    BEFORE UPDATE ON app_notification_prefs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMIT;
