-- Migration: Add tracking_config column to campaigns table
-- Description: Stores pixel and analytics configuration per campaign
-- Author: Claude Code
-- Date: 2026-01-23

-- Add tracking_config JSONB column for pixel and analytics configuration
-- Example structure:
-- {
--   "pixels": {
--     "facebook": { "pixel_id": "123456789", "enabled": true },
--     "google": { "measurement_id": "G-XXXXXX", "ads_id": "AW-XXXXXX", "enabled": true },
--     "tiktok": { "pixel_id": "CXXXXXXX", "enabled": false }
--   },
--   "attribution": {
--     "model": "last_touch",
--     "window_days": 7
--   },
--   "experiments": [
--     { "id": "cta-color", "variants": [...], "enabled": true }
--   ]
-- }

ALTER TABLE campaigns
ADD COLUMN IF NOT EXISTS tracking_config JSONB DEFAULT '{}';

-- Add GIN index for efficient JSONB queries
CREATE INDEX IF NOT EXISTS idx_campaigns_tracking_config
ON campaigns USING GIN (tracking_config);

-- Add comment for documentation
COMMENT ON COLUMN campaigns.tracking_config IS 'JSONB configuration for tracking pixels (Facebook, Google, TikTok), attribution models, and A/B experiments';

-- Backfill existing campaigns with empty tracking config (already handled by DEFAULT)
-- No data migration needed as DEFAULT '{}' handles this
