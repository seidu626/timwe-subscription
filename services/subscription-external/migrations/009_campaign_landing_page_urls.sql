-- Migration: Add landing_page_urls to campaigns table
-- Allows binding multiple landing page URLs per campaign (for multi-domain/LP support)

ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS landing_page_urls TEXT[];

COMMENT ON COLUMN campaigns.landing_page_urls IS 'Array of landing page URLs bound to this campaign (for preview/routing). Empty = use default /lp/{slug}';

-- Optional index if we need to query campaigns by LP URL in the future
-- CREATE INDEX IF NOT EXISTS idx_campaigns_landing_page_urls ON campaigns USING GIN (landing_page_urls);
