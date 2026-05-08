-- Migration: Add opt-out/opt-in renewal tracking
-- This migration adds tables and functions for the renewal system

BEGIN;

-- Track renewal cycles
CREATE TABLE IF NOT EXISTS renewal_cycles (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
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

-- Create churn tracking table
CREATE TABLE IF NOT EXISTS churn_tracking (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    msisdn VARCHAR(20) NOT NULL,
    product_id VARCHAR(50) NOT NULL,
    churn_reason VARCHAR(200),
    churned_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_payment_date TIMESTAMP,
    hours_without_payment INT,
    renewal_attempts INT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

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

-- Add renewal tracking columns to subscriptions table
-- Note: This assumes the subscriptions table exists
-- If it doesn't exist, you'll need to create it first
DO $$
BEGIN
    -- Check if subscriptions table exists
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'subscriptions') THEN
        -- Add renewal tracking columns if they don't exist
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'renewal_status') THEN
            ALTER TABLE subscriptions ADD COLUMN renewal_status VARCHAR(50) DEFAULT 'active';
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'last_renewal_attempt') THEN
            ALTER TABLE subscriptions ADD COLUMN last_renewal_attempt TIMESTAMP;
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'total_renewal_attempts') THEN
            ALTER TABLE subscriptions ADD COLUMN total_renewal_attempts INT DEFAULT 0;
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'last_successful_payment') THEN
            ALTER TABLE subscriptions ADD COLUMN last_successful_payment TIMESTAMP;
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'consecutive_payment_failures') THEN
            ALTER TABLE subscriptions ADD COLUMN consecutive_payment_failures INT DEFAULT 0;
        END IF;
    END IF;
END $$;

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_renewal_cycles_msisdn ON renewal_cycles(msisdn, product_id);
CREATE INDEX IF NOT EXISTS idx_renewal_cycles_status ON renewal_cycles(billing_status, created_at);
CREATE INDEX IF NOT EXISTS idx_renewal_cycles_created ON renewal_cycles(created_at);
CREATE INDEX IF NOT EXISTS idx_renewal_cycles_subscription ON renewal_cycles(subscription_id);

CREATE INDEX IF NOT EXISTS idx_churn_tracking_msisdn ON churn_tracking(msisdn, product_id);
CREATE INDEX IF NOT EXISTS idx_churn_tracking_churned ON churn_tracking(churned_at);
CREATE INDEX IF NOT EXISTS idx_churn_tracking_decision ON churn_tracking(churn_decision);

CREATE INDEX IF NOT EXISTS idx_priority_retry_next ON priority_retry_queue(next_retry_at, status);
CREATE INDEX IF NOT EXISTS idx_priority_retry_priority ON priority_retry_queue(priority DESC, created_at);
CREATE INDEX IF NOT EXISTS idx_priority_retry_msisdn ON priority_retry_queue(msisdn, product_id);

-- Add indexes to subscriptions table for renewal queries
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'subscriptions') THEN
        -- Add indexes for renewal queries if they don't exist
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subscriptions_renewal_status') THEN
            CREATE INDEX idx_subscriptions_renewal_status ON subscriptions(renewal_status, last_successful_payment);
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subscriptions_last_renewal') THEN
            CREATE INDEX idx_subscriptions_last_renewal ON subscriptions(last_renewal_attempt, renewal_status);
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subscriptions_payment_status') THEN
            CREATE INDEX idx_subscriptions_payment_status ON subscriptions(last_successful_payment, renewal_status);
        END IF;
    END IF;
END $$;

-- Drop existing functions if they exist to avoid return type conflicts
DROP FUNCTION IF EXISTS get_subscriptions_needing_renewal(INTEGER, INTEGER);
DROP FUNCTION IF EXISTS get_renewal_statistics(INTEGER);
DROP FUNCTION IF EXISTS get_churn_candidates(INTEGER, INTEGER, INTEGER);
DROP FUNCTION IF EXISTS increment_renewal_attempt(VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS churn_subscription(VARCHAR, VARCHAR, VARCHAR);

-- Function to get subscriptions needing renewal
CREATE OR REPLACE FUNCTION get_subscriptions_needing_renewal(
    p_days_threshold INT DEFAULT 7,
    p_limit INT DEFAULT 1000
) RETURNS TABLE (
    subscription_id BIGINT,
    msisdn VARCHAR,
    product_id VARCHAR,
    last_payment TIMESTAMP,
    days_since_payment INT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        s.id,
        s.user_identifier,
        s.product_id,
        s.last_successful_payment,
        EXTRACT(DAY FROM NOW() - COALESCE(s.last_successful_payment, s.created_at))::INT
    FROM subscriptions s
    WHERE s.status = 'active'
        AND COALESCE(s.renewal_status, 'active') != 'churned'
        AND (
            s.last_successful_payment IS NULL 
            OR s.last_successful_payment < NOW() - INTERVAL '2 days'
        )
        AND (
            s.last_renewal_attempt IS NULL 
            OR s.last_renewal_attempt < NOW() - INTERVAL '24 hours'
        )
    ORDER BY s.last_successful_payment ASC NULLS FIRST
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- Function to get renewal statistics
CREATE OR REPLACE FUNCTION get_renewal_statistics(
    p_hours_back INT DEFAULT 24
) RETURNS TABLE (
    total_cycles INT,
    successful_optouts INT,
    successful_optins INT,
    failed_optouts INT,
    failed_optins INT,
    pending_billing INT,
    success_rate NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*)::INT as total_cycles,
        COUNT(CASE WHEN opt_out_status = 'SUCCESS' THEN 1 END)::INT as successful_optouts,
        COUNT(CASE WHEN opt_in_status = 'SUCCESS' THEN 1 END)::INT as successful_optins,
        COUNT(CASE WHEN opt_out_status = 'FAILED' THEN 1 END)::INT as failed_optouts,
        COUNT(CASE WHEN opt_in_status = 'FAILED' THEN 1 END)::INT as failed_optins,
        COUNT(CASE WHEN billing_status = 'PENDING' THEN 1 END)::INT as pending_billing,
        ROUND(
            CASE 
                WHEN COUNT(*) > 0 THEN 
                    (COUNT(CASE WHEN opt_out_status = 'SUCCESS' AND opt_in_status = 'SUCCESS' THEN 1 END)::NUMERIC / COUNT(*) * 100)
                ELSE 0 
            END, 2
        ) as success_rate
    FROM renewal_cycles 
    WHERE created_at > NOW() - INTERVAL '1 hour' * p_hours_back;
END;
$$ LANGUAGE plpgsql;

-- Function to get churn candidates
CREATE OR REPLACE FUNCTION get_churn_candidates(
    p_max_hours_without_payment INT DEFAULT 168, -- 7 days * 24 hours
    p_max_renewal_attempts INT DEFAULT 3,
    p_limit INT DEFAULT 100
) RETURNS TABLE (
    subscription_id BIGINT,
    msisdn VARCHAR,
    product_id VARCHAR,
    hours_without_payment INT,
    renewal_attempts INT,
    last_renewal_attempt TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        s.id,
        s.user_identifier,
        s.product_id,
        EXTRACT(EPOCH FROM NOW() - COALESCE(s.last_successful_payment, s.created_at))::INT / 3600 as hours_without_payment,
        COALESCE(s.total_renewal_attempts, 0) as renewal_attempts,
        s.last_renewal_attempt
    FROM subscriptions s
    WHERE s.status = 'active'
        AND COALESCE(s.renewal_status, 'active') != 'churned'
        AND (
            s.last_successful_payment IS NULL 
            OR s.last_successful_payment < NOW() - INTERVAL '1 hour' * p_max_hours_without_payment
        )
        AND COALESCE(s.total_renewal_attempts, 0) >= p_max_renewal_attempts
    ORDER BY s.last_successful_payment ASC NULLS FIRST
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- Function to update renewal attempt count
CREATE OR REPLACE FUNCTION increment_renewal_attempt(
    p_msisdn VARCHAR,
    p_product_id VARCHAR
) RETURNS VOID AS $$
BEGIN
    UPDATE subscriptions 
    SET 
        total_renewal_attempts = COALESCE(total_renewal_attempts, 0) + 1,
        last_renewal_attempt = NOW(),
        updated_at = NOW()
    WHERE user_identifier = p_msisdn AND product_id = p_product_id;
END;
$$ LANGUAGE plpgsql;

-- Function to mark subscription as churned
CREATE OR REPLACE FUNCTION churn_subscription(
    p_msisdn VARCHAR,
    p_product_id VARCHAR,
    p_reason VARCHAR
) RETURNS VOID AS $$
DECLARE
    v_subscription_id BIGINT;
    v_last_payment_date TIMESTAMP;
    v_total_attempts INT;
BEGIN
    -- Get subscription details
    SELECT id, last_successful_payment, total_renewal_attempts 
    INTO v_subscription_id, v_last_payment_date, v_total_attempts
    FROM subscriptions 
    WHERE user_identifier = p_msisdn AND product_id = p_product_id;
    
    IF v_subscription_id IS NOT NULL THEN
        -- Update subscription status
        UPDATE subscriptions 
        SET 
            renewal_status = 'churned',
            status = 'cancelled',
            updated_at = NOW()
        WHERE id = v_subscription_id;
        
        -- Create churn record
        INSERT INTO churn_tracking (
            subscription_id, msisdn, product_id, last_payment_date,
            hours_without_payment, renewal_attempts, churn_reason, churned_at
        ) VALUES (
            v_subscription_id, p_msisdn, p_product_id, v_last_payment_date,
            CASE 
                WHEN v_last_payment_date IS NOT NULL 
                THEN EXTRACT(EPOCH FROM NOW() - v_last_payment_date)::INT / 3600
                ELSE 0 
            END,
            COALESCE(v_total_attempts, 0),
            p_reason,
            NOW()
        );
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Create a view for easy monitoring
CREATE OR REPLACE VIEW renewal_monitoring AS
SELECT 
    'renewal_cycles' as table_name,
    COUNT(*) as total_records,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '1 hour' THEN 1 END) as last_hour,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h
FROM renewal_cycles
UNION ALL
SELECT 
    'churn_tracking' as table_name,
    COUNT(*) as total_records,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '1 hour' THEN 1 END) as last_hour,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h
FROM churn_tracking
UNION ALL
SELECT 
    'priority_retry_queue' as table_name,
    COUNT(*) as total_records,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '1 hour' THEN 1 END) as last_hour,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h
FROM priority_retry_queue;

-- Grant permissions (adjust as needed for your setup)
-- GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO your_user;
-- GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO your_user;

COMMIT; 