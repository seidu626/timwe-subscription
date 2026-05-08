-- Migration: Optimize get_charging_failure_stats() Function
-- File: migrations/003_optimize_charging_failure_stats.sql
-- Purpose: Fix timeout issue by querying the view only ONCE instead of 6 times
-- Issue: Original function queries charging_failed_subscriptions 6 times causing 30s+ timeout

-- =========================================
-- 1. Optimized get_charging_failure_stats function
-- =========================================

CREATE OR REPLACE FUNCTION get_charging_failure_stats() RETURNS TABLE (
    category VARCHAR,
    count BIGINT,
    percentage NUMERIC
) AS $$
DECLARE
    total_count BIGINT;
    never_charged_count BIGINT;
    stale_charges_count BIGINT;
    no_optin_count BIGINT;
    optin_no_charge_count BIGINT;
BEGIN
    -- Query the view ONCE and aggregate all stats in a single pass
    SELECT 
        COUNT(*),
        COUNT(*) FILTER (WHERE cfs.last_charge IS NULL),
        COUNT(*) FILTER (WHERE cfs.last_charge < NOW() - INTERVAL '30 days'),
        COUNT(*) FILTER (WHERE cfs.last_optin IS NULL),
        COUNT(*) FILTER (WHERE cfs.last_optin IS NOT NULL AND cfs.last_charge IS NULL)
    INTO 
        total_count,
        never_charged_count,
        stale_charges_count,
        no_optin_count,
        optin_no_charge_count
    FROM charging_failed_subscriptions cfs;
    
    -- Return categorized statistics from the pre-computed values
    RETURN QUERY
    SELECT 
        'Total Charging Failures'::VARCHAR as category,
        total_count as count,
        CASE WHEN total_count > 0 THEN 100.0 ELSE 0.0 END as percentage
    UNION ALL
    SELECT 
        'Never Charged'::VARCHAR,
        never_charged_count,
        CASE WHEN total_count > 0 THEN (never_charged_count::NUMERIC / total_count * 100) ELSE 0 END
    UNION ALL
    SELECT 
        'Stale Charges (>30 days)'::VARCHAR,
        stale_charges_count,
        CASE WHEN total_count > 0 THEN (stale_charges_count::NUMERIC / total_count * 100) ELSE 0 END
    UNION ALL
    SELECT 
        'No Optin'::VARCHAR,
        no_optin_count,
        CASE WHEN total_count > 0 THEN (no_optin_count::NUMERIC / total_count * 100) ELSE 0 END
    UNION ALL
    SELECT 
        'Optin but No Charge'::VARCHAR,
        optin_no_charge_count,
        CASE WHEN total_count > 0 THEN (optin_no_charge_count::NUMERIC / total_count * 100) ELSE 0 END;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 2. Optimized get_total_charging_failures function
-- Uses the pre-computed charging_health_status column for faster results
-- =========================================

CREATE OR REPLACE FUNCTION get_total_charging_failures() RETURNS BIGINT AS $$
DECLARE
    total_count BIGINT;
BEGIN
    -- Use the pre-computed column instead of querying the complex view
    SELECT COUNT(*) INTO total_count 
    FROM subscriptions 
    WHERE charging_health_status IN ('NEVER_CHARGED', 'CHARGING_STALE');
    
    RETURN total_count;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 3. Add missing index for notifications join performance
-- =========================================

-- Composite index for the notifications-based charging analysis
CREATE INDEX IF NOT EXISTS idx_notifications_charging_lookup 
    ON notifications(type, msisdn, product_id, created_at DESC);

-- Index for charge/renewal notifications
CREATE INDEX IF NOT EXISTS idx_notifications_charge_types 
    ON notifications(msisdn, product_id, created_at) 
    WHERE type IN ('CHARGE', 'USER_RENEWED');

-- Index for optin notifications
CREATE INDEX IF NOT EXISTS idx_notifications_optin_type 
    ON notifications(msisdn, product_id, created_at) 
    WHERE type = 'USER_OPTIN';

-- =========================================
-- 4. Add index for subscriptions active query
-- =========================================

-- Index for the active subscriptions CTE in the view
CREATE INDEX IF NOT EXISTS idx_subscriptions_active_charging 
    ON subscriptions(status, created_at) 
    WHERE status = 'active' OR status IS NULL;

-- =========================================
-- 5. Alternative: Create a fast stats function using pre-computed column
-- This is even faster as it bypasses the view entirely
-- =========================================

CREATE OR REPLACE FUNCTION get_charging_failure_stats_fast() RETURNS TABLE (
    category VARCHAR,
    count BIGINT,
    percentage NUMERIC
) AS $$
DECLARE
    total_active BIGINT;
    never_charged_count BIGINT;
    stale_count BIGINT;
    delayed_count BIGINT;
    recent_count BIGINT;
    total_failures BIGINT;
BEGIN
    -- Use the pre-computed charging_health_status column
    SELECT 
        COUNT(*),
        COUNT(*) FILTER (WHERE charging_health_status = 'NEVER_CHARGED'),
        COUNT(*) FILTER (WHERE charging_health_status = 'CHARGING_STALE'),
        COUNT(*) FILTER (WHERE charging_health_status = 'CHARGING_DELAYED'),
        COUNT(*) FILTER (WHERE charging_health_status = 'CHARGING_RECENT')
    INTO 
        total_active,
        never_charged_count,
        stale_count,
        delayed_count,
        recent_count
    FROM subscriptions
    WHERE (status = 'active' OR status IS NULL)
      AND charging_health_status IS NOT NULL;
    
    total_failures := never_charged_count + stale_count;
    
    RETURN QUERY
    SELECT 
        'Total Subscriptions'::VARCHAR as category,
        total_active as count,
        100.0 as percentage
    UNION ALL
    SELECT 
        'Total Charging Failures'::VARCHAR,
        total_failures,
        CASE WHEN total_active > 0 THEN (total_failures::NUMERIC / total_active * 100) ELSE 0 END
    UNION ALL
    SELECT 
        'Never Charged'::VARCHAR,
        never_charged_count,
        CASE WHEN total_active > 0 THEN (never_charged_count::NUMERIC / total_active * 100) ELSE 0 END
    UNION ALL
    SELECT 
        'Stale Charges (>30 days)'::VARCHAR,
        stale_count,
        CASE WHEN total_active > 0 THEN (stale_count::NUMERIC / total_active * 100) ELSE 0 END
    UNION ALL
    SELECT 
        'Charging Delayed (7-30 days)'::VARCHAR,
        delayed_count,
        CASE WHEN total_active > 0 THEN (delayed_count::NUMERIC / total_active * 100) ELSE 0 END
    UNION ALL
    SELECT 
        'Charging Recent (<7 days)'::VARCHAR,
        recent_count,
        CASE WHEN total_active > 0 THEN (recent_count::NUMERIC / total_active * 100) ELSE 0 END;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions
GRANT EXECUTE ON FUNCTION get_charging_failure_stats_fast TO sm_admin;

-- =========================================
-- 6. Update comment
-- =========================================

COMMENT ON FUNCTION get_charging_failure_stats IS 'Get statistics about charging failures - OPTIMIZED: single view scan instead of 6';
COMMENT ON FUNCTION get_charging_failure_stats_fast IS 'Fast stats using pre-computed charging_health_status column - no view required';

-- =========================================
-- Migration Complete
-- =========================================
-- Expected improvement: Query time from 30+ seconds to <1 second
-- Original: 6 full view scans (each scan = full table joins)
-- Optimized: 1 full view scan with FILTER aggregation
-- Fast version: Direct index scan on subscriptions table
