-- Persist campaign offer context on each acquisition transaction so OTP confirmation
-- keeps using the original product/pricepoint/partner-role even if campaign settings change.
ALTER TABLE acquisition_transactions
    ADD COLUMN IF NOT EXISTS offer_product_id INTEGER,
    ADD COLUMN IF NOT EXISTS pricepoint_id INTEGER,
    ADD COLUMN IF NOT EXISTS partner_role_id INTEGER;

-- Best-effort backfill for existing rows.
UPDATE acquisition_transactions t
SET
    offer_product_id = c.offer_product_id,
    pricepoint_id = c.pricepoint_id,
    partner_role_id = c.partner_role_id
FROM campaigns c
WHERE t.campaign_slug = c.slug
  AND (
      t.offer_product_id IS NULL
      OR t.pricepoint_id IS NULL
      OR t.partner_role_id IS NULL
  );
