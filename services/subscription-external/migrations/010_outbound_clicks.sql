-- Migration: Outbound Clicks table for click-out redirect flow
-- Stores server-generated click_ids before redirecting users to ad networks (e.g. Mobplus)
-- These click_ids are later used in conversion postbacks

CREATE TABLE IF NOT EXISTS outbound_clicks (
    -- Primary key is the server-minted click_id (UUID)
    click_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Partner/provider identification (e.g. 'mobplus', 'generic')
    partner VARCHAR(50) NOT NULL,
    
    -- Campaign context (at least one should be set)
    campaign_slug VARCHAR(100),
    offer_product_id INTEGER,
    
    -- Destination info
    dest_key VARCHAR(50) NOT NULL,        -- Allowlist key (e.g. 'mobplus_track', 'landing_web')
    dest_url TEXT NOT NULL,               -- Fully rendered redirect URL
    
    -- Inbound request snapshot (for debugging/analytics)
    query_params JSONB DEFAULT '{}',      -- Raw query params from incoming request
    
    -- Request metadata (anonymized for privacy)
    referrer_domain VARCHAR(255),
    ip_hash VARCHAR(64),                  -- SHA256 hash of IP
    user_agent_hash VARCHAR(64),          -- SHA256 hash of User-Agent
    
    -- Status tracking
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED' 
        CHECK (status IN ('CREATED', 'REDIRECTED', 'CONVERTED', 'EXPIRED')),
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_outbound_clicks_partner_created 
    ON outbound_clicks (partner, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_outbound_clicks_campaign_created 
    ON outbound_clicks (campaign_slug, created_at DESC) 
    WHERE campaign_slug IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_outbound_clicks_status 
    ON outbound_clicks (status) 
    WHERE status != 'CONVERTED';

-- Trigger for updated_at
CREATE TRIGGER update_outbound_clicks_updated_at 
    BEFORE UPDATE ON outbound_clicks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments
COMMENT ON TABLE outbound_clicks IS 'Server-minted click_ids for outbound redirect flow. Used to track clicks we send to ad networks and correlate with conversion postbacks.';
COMMENT ON COLUMN outbound_clicks.click_id IS 'Server-generated UUID used as click identifier across the conversion funnel';
COMMENT ON COLUMN outbound_clicks.dest_key IS 'Allowlist key for destination validation (prevents open redirect)';
COMMENT ON COLUMN outbound_clicks.ip_hash IS 'SHA256 hash of client IP for analytics only (no raw IP stored)';

-- Retention policy note: Consider adding a cleanup job to purge old clicks
-- Example: DELETE FROM outbound_clicks WHERE created_at < NOW() - INTERVAL '90 days' AND status IN ('CREATED', 'EXPIRED');
