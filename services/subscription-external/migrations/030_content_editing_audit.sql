-- Migration: Post-publish content editing support
-- File: migrations/030_content_editing_audit.sql
--
-- 1) updated_at on message_content_items so the console can show freshness.
-- 2) message_content_revisions: prior values snapshotted on every UPDATE of a
--    content item, so "what did subscribers actually receive on date X" stays
--    answerable after live edits. The actor comes from the per-transaction
--    setting app.actor (set by the admin API); NULL when changed outside it.

BEGIN;

ALTER TABLE message_content_items
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DROP TRIGGER IF EXISTS update_message_content_items_updated_at ON message_content_items;
CREATE TRIGGER update_message_content_items_updated_at
    BEFORE UPDATE ON message_content_items
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS message_content_revisions (
    id              BIGSERIAL PRIMARY KEY,
    content_item_id BIGINT NOT NULL REFERENCES message_content_items(id) ON DELETE CASCADE,
    series_id       BIGINT NOT NULL,
    content_version INTEGER NOT NULL,
    seq_no          INT NULL,
    message_text    TEXT NOT NULL,
    content_kind    TEXT NOT NULL DEFAULT 'TEXT',
    link_url        TEXT NULL,
    cta_label       TEXT NULL,
    is_active       BOOLEAN NOT NULL,
    changed_by      TEXT NULL,
    changed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mcr_item_changed
    ON message_content_revisions (content_item_id, changed_at DESC);

CREATE OR REPLACE FUNCTION log_content_item_revision()
RETURNS TRIGGER AS $$
BEGIN
    IF (OLD.message_text IS DISTINCT FROM NEW.message_text
        OR OLD.content_kind IS DISTINCT FROM NEW.content_kind
        OR OLD.link_url IS DISTINCT FROM NEW.link_url
        OR OLD.cta_label IS DISTINCT FROM NEW.cta_label
        OR OLD.is_active IS DISTINCT FROM NEW.is_active) THEN
        INSERT INTO message_content_revisions (
            content_item_id, series_id, content_version, seq_no,
            message_text, content_kind, link_url, cta_label, is_active, changed_by
        ) VALUES (
            OLD.id, OLD.series_id, OLD.content_version, OLD.seq_no,
            OLD.message_text, OLD.content_kind, OLD.link_url, OLD.cta_label, OLD.is_active,
            NULLIF(current_setting('app.actor', true), '')
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS log_message_content_items_revision ON message_content_items;
CREATE TRIGGER log_message_content_items_revision
    AFTER UPDATE ON message_content_items
    FOR EACH ROW
    EXECUTE FUNCTION log_content_item_revision();

COMMIT;
