-- Migration: Web Acquisition Campaigns and Transactions
-- Creates tables for campaign config, acquisition transactions, and postback tracking

-- Campaigns table: configurable campaign definitions
CREATE TABLE IF NOT EXISTS campaigns (
    id SERIAL PRIMARY KEY,
    slug VARCHAR(100) UNIQUE NOT NULL,
    language VARCHAR(10) DEFAULT 'en',
    country VARCHAR(10) NOT NULL,
    operator VARCHAR(50),
    
    -- Offer/product mapping
    offer_product_id INTEGER NOT NULL,
    pricepoint_id INTEGER,
    partner_role_id INTEGER,
    
    -- Flow configuration
    flow_type VARCHAR(20) NOT NULL DEFAULT 'OTP' 
        CHECK (flow_type IN ('CLICK_TO_SMS', 'OTP', 'REDIRECT', 'MIXED')),
    short_code VARCHAR(10),
    sms_keyword VARCHAR(50),
    
    -- Pricing
    price DECIMAL(10,2),
    billing_cycle VARCHAR(50),
    trial_flags JSONB,
    
    -- Compliance
    terms_url TEXT,
    inline_terms_text TEXT,
    consent_required BOOLEAN DEFAULT true,
    consent_version VARCHAR(50),
    
    -- Attribution and postback configuration
    attribution_mapping JSONB DEFAULT '{}',
    postback_rules JSONB DEFAULT '{}',
    
    -- Throttles and controls
    throttles JSONB DEFAULT '{}',
    allowed_referrers TEXT[],
    allowed_sources TEXT[],
    
    -- Metadata
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(50),
    updated_by VARCHAR(50)
);

CREATE INDEX idx_campaigns_slug ON campaigns(slug);
CREATE INDEX idx_campaigns_enabled ON campaigns(enabled);
CREATE INDEX idx_campaigns_country ON campaigns(country);

-- Landing versions table (optional, for compliance tracking)
CREATE TABLE IF NOT EXISTS landing_versions (
    id SERIAL PRIMARY KEY,
    campaign_slug VARCHAR(100) NOT NULL,
    version INTEGER NOT NULL,
    page_hash VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (campaign_slug) REFERENCES campaigns(slug) ON DELETE CASCADE
);

CREATE INDEX idx_landing_versions_slug ON landing_versions(campaign_slug);

-- Acquisition transactions: single source of truth for web acquisition attempts
CREATE TABLE IF NOT EXISTS acquisition_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    correlation_id UUID NOT NULL,
    
    -- Campaign and user
    campaign_slug VARCHAR(100) NOT NULL,
    msisdn VARCHAR(15) NOT NULL,
    
    -- Status and flow
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'ACTION_REQUIRED', 'CONFIRM_REQUIRED', 'SUBSCRIBED', 'FAILED', 'CANCELLED')),
    next_action VARCHAR(20),
    next_action_payload JSONB,
    
    -- Attribution
    ad_provider VARCHAR(50),
    click_id VARCHAR(255),
    attribution_data JSONB DEFAULT '{}',
    
    -- Request metadata
    ip_address INET,
    user_agent TEXT,
    
    -- Consent tracking
    consent_required BOOLEAN DEFAULT false,
    consent_checked BOOLEAN DEFAULT false,
    consent_version VARCHAR(50),
    consent_timestamp TIMESTAMP,
    landing_version_hash VARCHAR(64),
    
    -- TIMWE integration
    timwe_transaction_id VARCHAR(255),
    transaction_auth_code VARCHAR(50),
    timwe_status VARCHAR(50),
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (campaign_slug) REFERENCES campaigns(slug) ON DELETE RESTRICT
);

CREATE INDEX idx_acq_trans_correlation ON acquisition_transactions(correlation_id);
CREATE INDEX idx_acq_trans_campaign ON acquisition_transactions(campaign_slug);
CREATE INDEX idx_acq_trans_msisdn ON acquisition_transactions(msisdn);
CREATE INDEX idx_acq_trans_status ON acquisition_transactions(status);
CREATE INDEX idx_acq_trans_click_id ON acquisition_transactions(click_id);
CREATE INDEX idx_acq_trans_created ON acquisition_transactions(created_at);

-- Consents table (immutable consent records)
CREATE TABLE IF NOT EXISTS consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL,
    msisdn_hash VARCHAR(64),
    consent_version VARCHAR(50) NOT NULL,
    page_hash VARCHAR(64),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ip_address INET,
    
    FOREIGN KEY (transaction_id) REFERENCES acquisition_transactions(id) ON DELETE CASCADE
);

CREATE INDEX idx_consents_transaction ON consents(transaction_id);
CREATE INDEX idx_consents_msisdn_hash ON consents(msisdn_hash);

-- Postback outbox: queued postbacks for async delivery
CREATE TABLE IF NOT EXISTS postback_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL,
    event VARCHAR(50) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    url_template_rendered TEXT NOT NULL,
    http_method VARCHAR(10) DEFAULT 'POST',
    headers JSONB DEFAULT '{}',
    body JSONB,
    
    -- Retry tracking
    attempt_count INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 5,
    next_retry_at TIMESTAMP,
    status VARCHAR(20) DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED', 'DLQ')),
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (transaction_id) REFERENCES acquisition_transactions(id) ON DELETE CASCADE
);

CREATE INDEX idx_postback_outbox_status ON postback_outbox(status);
CREATE INDEX idx_postback_outbox_next_retry ON postback_outbox(next_retry_at);
CREATE INDEX idx_postback_outbox_transaction ON postback_outbox(transaction_id);
CREATE INDEX idx_postback_outbox_provider ON postback_outbox(provider);

-- Postback attempts: log of every postback attempt
CREATE TABLE IF NOT EXISTS postback_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outbox_id UUID NOT NULL,
    attempt_number INTEGER NOT NULL,
    http_status INTEGER,
    response_body TEXT,
    error_message TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (outbox_id) REFERENCES postback_outbox(id) ON DELETE CASCADE
);

CREATE INDEX idx_postback_attempts_outbox ON postback_attempts(outbox_id);
CREATE INDEX idx_postback_attempts_status ON postback_attempts(http_status);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_campaigns_updated_at BEFORE UPDATE ON campaigns
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_acquisition_transactions_updated_at BEFORE UPDATE ON acquisition_transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_postback_outbox_updated_at BEFORE UPDATE ON postback_outbox
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
