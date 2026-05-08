-- Migration: Add opt-out/opt-in renewal tracking tables and columns
-- File: 001_add_renewal_tracking.sql

BEGIN;

-- Track renewal cycles
CREATE TABLE IF NOT EXISTS renewal_cycles (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT,
    msisdn VARCHAR(20) NOT NULL,
    product_id VARCHAR(20) NOT NULL,
    cycle_number INT DEFAULT 1,
    opt_out_time TIMESTAMP,
    opt_out_status VARCHAR(50),
    opt_out_response TEXT,
    opt_in_time TIMESTAMP,
    opt_in_status VARCHAR(50),
    opt_in_response TEXT,
    billing_status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_renewal_cycles_msisdn ON renewal_cycles(msisdn, product_id);
CREATE INDEX idx_renewal_cycles_status ON renewal_cycles(billing_status, created_at);
CREATE INDEX idx_renewal_cycles_created ON renewal_cycles(created_at);

-- Track churn decisions and history
CREATE TABLE IF NOT EXISTS churn_tracking (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT,
    msisdn VARCHAR(20) NOT NULL,
    product_id VARCHAR(20) NOT NULL,
    last_payment_date TIMESTAMP,
    hours_without_payment INT,
    renewal_attempts INT DEFAULT 0,
    churn_decision VARCHAR(50),
    churn_reason VARCHAR(100),
    churned_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_churn_tracking_msisdn ON churn_tracking(msisdn, product_id);
CREATE INDEX idx_churn_tracking_churned ON churn_tracking(churned_at);
CREATE INDEX idx_churn_tracking_decision ON churn_tracking(churn_decision);

-- Priority retry queue for failed opt-ins
CREATE TABLE IF NOT EXISTS priority_retry_queue (
    id BIGSERIAL PRIMARY KEY,
    msisdn VARCHAR(20) NOT NULL,
    product_id VARCHAR(20) NOT NULL,
    reason VARCHAR(50),
    priority INT DEFAULT 1,
    retry_count INT DEFAULT 0,
    next_retry_at TIMESTAMP,
    last_attempt_at TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_priority_retry_next ON priority_retry_queue(next_retry_at, status);
CREATE INDEX idx_priority_retry_priority ON priority_retry_queue(priority DESC, created_at);
CREATE INDEX idx_priority_retry_status ON priority_retry_queue(status);

-- Add renewal tracking columns to subscriptions table
ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS renewal_status VARCHAR(50) DEFAULT 'active',
ADD COLUMN IF NOT EXISTS last_renewal_attempt TIMESTAMP,
ADD COLUMN IF NOT EXISTS total_renewal_attempts INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_successful_payment TIMESTAMP,
ADD COLUMN IF NOT EXISTS consecutive_payment_failures INT DEFAULT 0;

-- Create renewal metrics table
CREATE TABLE IF NOT EXISTS renewal_metrics (
    id BIGSERIAL PRIMARY KEY,
    total_processed BIGINT DEFAULT 0,
    successful_renewals BIGINT DEFAULT 0,
    failed_renewals BIGINT DEFAULT 0,
    churned_subscriptions BIGINT DEFAULT 0,
    success_rate DECIMAL(5,2),
    average_cycle_time DECIMAL(10,2),
    last_run_time TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Function to get subscriptions needing renewal
CREATE OR REPLACE FUNCTION get_subscriptions_needing_renewal(
    p_hours_threshold INT DEFAULT 168, -- 7 days in hours
    p_limit INT DEFAULT 1000
) RETURNS TABLE (
    subscription_id BIGINT,
    msisdn VARCHAR,
    product_id VARCHAR,
    last_payment TIMESTAMP,
    hours_since_payment INT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        s.id,
        s.user_identifier as msisdn,
        s.product_id,
        s.last_successful_payment,
        EXTRACT(HOUR FROM NOW() - COALESCE(s.last_successful_payment, s.created_at))::INT
    FROM subscriptions s
    WHERE s.status = 'active'
        AND s.renewal_status != 'churned'
        AND (
            s.last_successful_payment IS NULL 
            OR s.last_successful_payment < NOW() - INTERVAL '48 hours'
        )
        AND (
            s.last_renewal_attempt IS NULL 
            OR s.last_renewal_attempt < NOW() - INTERVAL '24 hours'
        )
    ORDER BY s.last_successful_payment ASC NULLS FIRST
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- Function to get daily churn count
CREATE OR REPLACE FUNCTION get_daily_churn_count(p_date DATE DEFAULT CURRENT_DATE)
RETURNS INT AS $$
DECLARE
    churn_count INT;
BEGIN
    SELECT COUNT(*) INTO churn_count
    FROM churn_tracking
    WHERE DATE(churned_at) = p_date;
    
    RETURN churn_count;
END;
$$ LANGUAGE plpgsql;

-- Function to update subscription renewal status
CREATE OR REPLACE FUNCTION update_subscription_renewal_status(
    p_msisdn VARCHAR,
    p_product_id VARCHAR,
    p_status VARCHAR
) RETURNS VOID AS $$
BEGIN
    UPDATE subscriptions
    SET renewal_status = p_status,
        updated_at = CURRENT_TIMESTAMP
    WHERE user_identifier = p_msisdn
        AND product_id = p_product_id;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_renewal_cycles_updated_at
    BEFORE UPDATE ON renewal_cycles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_priority_retry_queue_updated_at
    BEFORE UPDATE ON priority_retry_queue
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMIT;
