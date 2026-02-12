-- Migration: Fix renewal functions to use correct column names
-- This migration fixes the column name mismatch between 'msisdn' and 'user_identifier'

BEGIN;

-- Drop existing functions to avoid return type conflicts
DROP FUNCTION IF EXISTS get_subscriptions_needing_renewal(INTEGER, INTEGER);
DROP FUNCTION IF EXISTS get_churn_candidates(INTEGER, INTEGER, INTEGER);
DROP FUNCTION IF EXISTS increment_renewal_attempt(VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS churn_subscription(VARCHAR, VARCHAR, VARCHAR);

-- Function to get subscriptions needing renewal (FIXED: uses user_identifier)
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

-- Function to get churn candidates (FIXED: uses user_identifier)
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

-- Function to update renewal attempt count (FIXED: uses user_identifier)
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

-- Function to mark subscription as churned (FIXED: uses user_identifier)
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

COMMIT; 