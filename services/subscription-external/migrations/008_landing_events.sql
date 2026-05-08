-- Migration: Landing Events for Acquisition Funnel Reporting
-- Stores anonymous landing page events (views, clicks) without PII
-- Used to compute full acquisition funnel: view → click → transaction → subscribed → charged

CREATE TABLE IF NOT EXISTS landing_events (
    id BIGSERIAL PRIMARY KEY,
    
    -- Event identification
    event_type VARCHAR(30) NOT NULL CHECK (event_type IN ('landing_view', 'landing_click', 'form_submit')),
    campaign_slug VARCHAR(100) NOT NULL,
    
    -- Attribution (no MSISDN stored here)
    click_id VARCHAR(255),
    ad_provider VARCHAR(50),
    
    -- Session correlation (optional, for deduplication)
    session_id VARCHAR(100),
    
    -- Request metadata (anonymized)
    ip_hash VARCHAR(64),          -- SHA256 hash of IP for rough geo/bot detection
    user_agent_hash VARCHAR(64),  -- SHA256 hash of UA
    referrer_domain VARCHAR(255), -- Domain only, not full URL
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Performance indexes for reporting queries
CREATE INDEX IF NOT EXISTS idx_landing_events_campaign_created 
    ON landing_events (campaign_slug, created_at);

CREATE INDEX IF NOT EXISTS idx_landing_events_event_type_created 
    ON landing_events (event_type, created_at);

CREATE INDEX IF NOT EXISTS idx_landing_events_click_id 
    ON landing_events (click_id) 
    WHERE click_id IS NOT NULL;

-- Composite index for funnel queries by campaign + event type + date
CREATE INDEX IF NOT EXISTS idx_landing_events_funnel 
    ON landing_events (campaign_slug, event_type, created_at DESC);

-- Partitioning hint: if volume grows, consider partitioning by created_at month
COMMENT ON TABLE landing_events IS 'Anonymous landing page events for acquisition funnel reporting. No PII stored.';
COMMENT ON COLUMN landing_events.ip_hash IS 'SHA256 hash of client IP for rough geo/bot detection only';
COMMENT ON COLUMN landing_events.session_id IS 'Optional session correlation ID for deduplication';
