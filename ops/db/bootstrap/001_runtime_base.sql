-- Runtime compose base schema.
--
-- This file contains only the cross-service prerequisite tables that older
-- service-owned migrations assume already exist. Service-owned migrations stay
-- canonical for admin management, notification cadence, and postback routing.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    product_id VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    price_point_id INT NOT NULL,
    price_point_value DECIMAL(10, 2) NOT NULL,
    short_code VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS userbase (
    id SERIAL PRIMARY KEY,
    msisdn VARCHAR(15) NOT NULL UNIQUE,
    type VARCHAR(20) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_userbase_msisdn ON userbase (msisdn);
CREATE INDEX IF NOT EXISTS idx_userbase_type ON userbase (type);

CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    partner_role_id INTEGER NOT NULL,
    user_identifier VARCHAR(50) NOT NULL,
    user_identifier_type VARCHAR(20) NOT NULL DEFAULT 'MSISDN',
    product_id INTEGER NOT NULL,
    mcc VARCHAR(5),
    mnc VARCHAR(5),
    entry_channel VARCHAR(20),
    large_account VARCHAR(50),
    sub_keyword VARCHAR(50),
    tracking_id VARCHAR(50),
    client_ip VARCHAR(50),
    campaign_url VARCHAR(255),
    transaction_auth_code VARCHAR(100),
    status VARCHAR(20) DEFAULT 'active',
    renewal_status VARCHAR(20) DEFAULT 'active',
    cancel_reason INTEGER,
    cancel_source INTEGER,
    start_date TIMESTAMP DEFAULT NOW(),
    end_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    tenant_id UUID,
    channel_id UUID
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_status_renewal
    ON subscriptions (status, renewal_status, start_date);

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_channel
    ON subscriptions (tenant_id, channel_id, status, renewal_status)
    WHERE tenant_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS campaigns (
    id SERIAL PRIMARY KEY,
    slug VARCHAR(100) UNIQUE NOT NULL,
    language VARCHAR(10) DEFAULT 'en',
    country VARCHAR(10) NOT NULL,
    operator VARCHAR(50),
    offer_product_id INTEGER NOT NULL,
    pricepoint_id INTEGER,
    partner_role_id INTEGER,
    flow_type VARCHAR(20) NOT NULL DEFAULT 'OTP'
        CHECK (flow_type IN ('CLICK_TO_SMS', 'OTP', 'REDIRECT', 'MIXED')),
    short_code VARCHAR(10),
    sms_keyword VARCHAR(50),
    price DECIMAL(10,2),
    billing_cycle VARCHAR(50),
    trial_flags JSONB,
    terms_url TEXT,
    inline_terms_text TEXT,
    consent_required BOOLEAN DEFAULT true,
    consent_version VARCHAR(50),
    attribution_mapping JSONB DEFAULT '{}',
    postback_rules JSONB DEFAULT '{}',
    throttles JSONB DEFAULT '{}',
    allowed_referrers TEXT[],
    allowed_sources TEXT[],
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(50),
    updated_by VARCHAR(50)
);

CREATE INDEX IF NOT EXISTS idx_campaigns_slug ON campaigns(slug);
CREATE INDEX IF NOT EXISTS idx_campaigns_enabled ON campaigns(enabled);
CREATE INDEX IF NOT EXISTS idx_campaigns_country ON campaigns(country);

CREATE TABLE IF NOT EXISTS acquisition_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    correlation_id UUID NOT NULL,
    campaign_slug VARCHAR(100) NOT NULL,
    msisdn VARCHAR(15) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'ACTION_REQUIRED', 'CONFIRM_REQUIRED', 'SUBSCRIBED', 'FAILED', 'CANCELLED')),
    next_action VARCHAR(20),
    next_action_payload JSONB,
    ad_provider VARCHAR(50),
    click_id VARCHAR(255),
    attribution_data JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    consent_required BOOLEAN DEFAULT false,
    consent_checked BOOLEAN DEFAULT false,
    consent_version VARCHAR(50),
    consent_timestamp TIMESTAMP,
    landing_version_hash VARCHAR(64),
    timwe_transaction_id VARCHAR(255),
    transaction_auth_code VARCHAR(50),
    timwe_status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (campaign_slug) REFERENCES campaigns(slug) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_acq_trans_correlation ON acquisition_transactions(correlation_id);
CREATE INDEX IF NOT EXISTS idx_acq_trans_campaign ON acquisition_transactions(campaign_slug);
CREATE INDEX IF NOT EXISTS idx_acq_trans_msisdn ON acquisition_transactions(msisdn);
CREATE INDEX IF NOT EXISTS idx_acq_trans_status ON acquisition_transactions(status);
CREATE INDEX IF NOT EXISTS idx_acq_trans_click_id ON acquisition_transactions(click_id);
CREATE INDEX IF NOT EXISTS idx_acq_trans_created ON acquisition_transactions(created_at);

CREATE TABLE IF NOT EXISTS consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL,
    msisdn_hash VARCHAR(64),
    consent_version VARCHAR(50) NOT NULL,
    page_hash VARCHAR(64),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (transaction_id) REFERENCES acquisition_transactions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_consents_transaction ON consents(transaction_id);
CREATE INDEX IF NOT EXISTS idx_consents_msisdn_hash ON consents(msisdn_hash);
