-- Migration: supporting index for the Dayline app feed hot path.
-- Owner: subscription-external. See docs/dayline-app-api-contract.md.
-- Additive only: CREATE INDEX IF NOT EXISTS, no locking beyond a normal index build.

BEGIN;

-- internal/repository/app_feed.go's feedSelectColumns (ListFeed, GetFeedItem,
-- MarkRead) joins message_outbox -> subscriptions on s.id = mo.subscription_id
-- and filters mo.status = 'SENT'. message_outbox only has idx_outbox_pending
-- (status, planned_send_at) and idx_outbox_processed (status, processed_at),
-- neither of which leads on subscription_id, so the join has no supporting
-- index in either direction.
--
-- subscriptions(user_identifier) itself is already covered by
-- idx_subscriptions_user_product / idx_subscriptions_user_product_status in
-- services/pg_schema.sql (both lead on user_identifier), so no new index is
-- added there.
CREATE INDEX IF NOT EXISTS idx_outbox_subscription_status
    ON message_outbox (subscription_id, status);

COMMIT;
