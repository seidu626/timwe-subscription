-- Migration: Notifications-Based Charging Failure Strategy (FIXED)
-- File: migrations/002_notifications_based_charging_fixed.sql
-- Based on FINAL_CHARGING_STRATEGY.md
-- FIXED: Proper order - add columns first, then create indexes

-- =========================================
-- 1. Add charging notification tracking columns
-- =========================================

-- Add columns to track charging status from notifications
ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS last_charge_notification_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS total_charge_notifications INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS charging_health_status VARCHAR(50),
ADD COLUMN IF NOT EXISTS last_optin_notification_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS charging_failure_reason VARCHAR(255);

-- =========================================
-- 2. Create indexes for performance (AFTER columns exist)
-- =========================================

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_subscriptions_charge_notification 
    ON subscriptions(last_charge_notification_at);

CREATE INDEX IF NOT EXISTS idx_subscriptions_charging_health 
    ON subscriptions(charging_health_status);

CREATE INDEX IF NOT EXISTS idx_subscriptions_optin_notification 
    ON subscriptions(last_optin_notification_at);

-- =========================================
-- 3. Create charging failed subscriptions view
-- =========================================

CREATE OR REPLACE VIEW charging_failed_subscriptions AS
WITH active_subs AS (
    SELECT 
        s.id,
        s.user_identifier as msisdn,
        s.product_id,
        s.entry_channel,
        s.created_at,
        s.status
    FROM subscriptions s
    WHERE (s.status = 'active' OR s.status IS NULL)
    AND s.created_at < NOW() - INTERVAL '1 day' -- Allow time for initial charging
),
charge_history AS (
    SELECT 
        n.msisdn,
        n.product_id,
        MAX(n.created_at) as last_charge,
        COUNT(*) as total_charges
    FROM notifications n
    WHERE n.type IN ('CHARGE', 'USER_RENEWED')
    GROUP BY n.msisdn, n.product_id
),
optin_history AS (
    SELECT 
        n.msisdn,
        n.product_id,
        MAX(n.created_at) as last_optin
    FROM notifications n
    WHERE n.type = 'USER_OPTIN'
    GROUP BY n.msisdn, n.product_id
)
SELECT 
    a.id as subscription_id,
    a.msisdn,
    a.product_id,
    a.entry_channel,
    a.created_at as subscription_date,
    ch.last_charge,
    ch.total_charges,
    oh.last_optin,
    CASE
        WHEN ch.msisdn IS NULL THEN 'NEVER_CHARGED'
        WHEN ch.last_charge < NOW() - INTERVAL '30 days' THEN 'CHARGING_STALE'
        WHEN ch.last_charge < NOW() - INTERVAL '7 days' THEN 'CHARGING_DELAYED'
        ELSE 'CHARGING_RECENT'
    END as charging_status,
    EXTRACT(DAY FROM NOW() - a.created_at) as days_since_subscription,
    CASE 
        WHEN ch.last_charge IS NULL THEN 
            EXTRACT(DAY FROM NOW() - a.created_at)
        ELSE 
            EXTRACT(DAY FROM NOW() - ch.last_charge)
    END as days_since_last_charge,
    CASE
        WHEN oh.last_optin IS NULL THEN 'NO_OPTIN'
        WHEN ch.last_charge IS NULL THEN 'OPTIN_NO_CHARGE'
        WHEN ch.last_charge < NOW() - INTERVAL '30 days' THEN 'OPTIN_STALE_CHARGE'
        ELSE 'OPTIN_RECENT_CHARGE'
    END as optin_charge_status
FROM active_subs a
LEFT JOIN charge_history ch ON a.msisdn = ch.msisdn AND a.product_id = ch.product_id
LEFT JOIN optin_history oh ON a.msisdn = oh.msisdn AND a.product_id = oh.product_id
WHERE ch.msisdn IS NULL  -- Never charged
   OR ch.last_charge < NOW() - INTERVAL '30 days';  -- Or not charged recently

-- =========================================
-- 4. Create function to identify charging failures
-- =========================================

CREATE OR REPLACE FUNCTION identify_charging_failures(
    days_threshold INTEGER DEFAULT 30,
    product_ids INTEGER[] DEFAULT NULL,
    limit_count INTEGER DEFAULT 1000
) RETURNS TABLE (
    subscription_id INTEGER,
    msisdn VARCHAR,
    product_id INTEGER,
    entry_channel VARCHAR,
    subscription_date TIMESTAMP,
    last_charge TIMESTAMP,
    total_charges INTEGER,
    last_optin TIMESTAMP,
    charging_status VARCHAR,
    days_since_subscription INTEGER,
    days_since_last_charge INTEGER,
    optin_charge_status VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        cfs.subscription_id,
        cfs.msisdn,
        cfs.product_id,
        cfs.entry_channel,
        cfs.subscription_date,
        cfs.last_charge,
        cfs.total_charges,
        cfs.last_optin,
        cfs.charging_status,
        cfs.days_since_subscription,
        cfs.days_since_last_charge,
        cfs.optin_charge_status
    FROM charging_failed_subscriptions cfs
    WHERE (product_ids IS NULL OR cfs.product_id = ANY(product_ids))
    AND (cfs.last_charge IS NULL OR cfs.last_charge < NOW() - INTERVAL '1 day' * days_threshold)
    ORDER BY 
        CASE 
            WHEN cfs.last_optin IS NULL THEN 1  -- No optin, highest priority
            WHEN cfs.last_charge IS NULL THEN 2 -- Optin but no charge
            ELSE 3                              -- Stale charge, lowest priority
        END,
        cfs.days_since_last_charge DESC NULLS FIRST
    LIMIT limit_count;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 5. Create function to get charging failure statistics
-- =========================================

CREATE OR REPLACE FUNCTION get_charging_failure_stats() RETURNS TABLE (
    category VARCHAR,
    count BIGINT,
    percentage NUMERIC
) AS $$
DECLARE
    total_count BIGINT;
BEGIN
    -- Get total count
    SELECT COUNT(*) INTO total_count FROM charging_failed_subscriptions;
    
    -- Return categorized statistics
    RETURN QUERY
    SELECT 
        'Total Charging Failures'::VARCHAR as category,
        total_count as count,
        CASE WHEN total_count > 0 THEN 100.0 ELSE 0.0 END as percentage
    UNION ALL
    SELECT 
        'Never Charged'::VARCHAR,
        COUNT(*) as count,
        CASE WHEN total_count > 0 THEN (COUNT(*)::NUMERIC / total_count * 100) ELSE 0 END as percentage
    FROM charging_failed_subscriptions 
    WHERE last_charge IS NULL
    UNION ALL
    SELECT 
        'Stale Charges (>30 days)'::VARCHAR,
        COUNT(*) as count,
        CASE WHEN total_count > 0 THEN (COUNT(*)::NUMERIC / total_count * 100) ELSE 0 END as percentage
    FROM charging_failed_subscriptions 
    WHERE last_charge < NOW() - INTERVAL '30 days'
    UNION ALL
    SELECT 
        'No Optin'::VARCHAR,
        COUNT(*) as count,
        CASE WHEN total_count > 0 THEN (COUNT(*)::NUMERIC / total_count * 100) ELSE 0 END as percentage
    FROM charging_failed_subscriptions 
    WHERE last_optin IS NULL
    UNION ALL
    SELECT 
        'Optin but No Charge'::VARCHAR,
        COUNT(*) as count,
        CASE WHEN total_count > 0 THEN (COUNT(*)::NUMERIC / total_count * 100) ELSE 0 END as percentage
    FROM charging_failed_subscriptions 
    WHERE last_optin IS NOT NULL AND last_charge IS NULL;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 6. Create function to get total charging failure count
-- =========================================

CREATE OR REPLACE FUNCTION get_total_charging_failures() RETURNS BIGINT AS $$
DECLARE
    total_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO total_count FROM charging_failed_subscriptions;
    RETURN total_count;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 7. Update existing records with charging data (initial population)
-- =========================================

-- Update subscriptions with charging notification data
UPDATE subscriptions s SET
    last_charge_notification_at = ch.last_charge,
    total_charge_notifications = ch.total_charges
FROM (
    SELECT 
        n.msisdn,
        n.product_id,
        MAX(n.created_at) as last_charge,
        COUNT(*) as total_charges
    FROM notifications n
    WHERE n.type IN ('CHARGE', 'USER_RENEWED')
    GROUP BY n.msisdn, n.product_id
) ch
WHERE s.user_identifier = ch.msisdn AND s.product_id = ch.product_id;

-- Update subscriptions with optin notification data
UPDATE subscriptions s SET
    last_optin_notification_at = oh.last_optin
FROM (
    SELECT 
        n.msisdn,
        n.product_id,
        MAX(n.created_at) as last_optin
    FROM notifications n
    WHERE n.type = 'USER_OPTIN'
    GROUP BY n.msisdn, n.product_id
) oh
WHERE s.user_identifier = oh.msisdn AND s.product_id = oh.product_id;

-- Update charging health status based on data
UPDATE subscriptions SET
    charging_health_status = CASE
        WHEN last_charge_notification_at IS NULL THEN 'NEVER_CHARGED'
        WHEN last_charge_notification_at < NOW() - INTERVAL '30 days' THEN 'CHARGING_STALE'
        WHEN last_charge_notification_at < NOW() - INTERVAL '7 days' THEN 'CHARGING_DELAYED'
        ELSE 'CHARGING_RECENT'
    END
WHERE charging_health_status IS NULL;

-- =========================================
-- 8. Create additional indexes for performance
-- =========================================

CREATE INDEX IF NOT EXISTS idx_subscriptions_charging_health_status 
    ON subscriptions(charging_health_status);

CREATE INDEX IF NOT EXISTS idx_subscriptions_resubscribe_status 
    ON subscriptions(status);

CREATE INDEX IF NOT EXISTS idx_subscriptions_charging_failure_query 
    ON subscriptions(user_identifier, product_id, charging_health_status);

-- =========================================
-- 9. Create summary view for monitoring
-- =========================================

CREATE OR REPLACE VIEW charging_failure_summary AS
SELECT 
    charging_health_status,
    COUNT(*) as subscription_count,
    ROUND(COUNT(*)::NUMERIC / (SELECT COUNT(*) FROM subscriptions) * 100, 2) as percentage
FROM subscriptions 
WHERE charging_health_status IS NOT NULL
GROUP BY charging_health_status
ORDER BY subscription_count DESC;

-- =========================================
-- 10. Grant permissions
-- =========================================

GRANT SELECT ON charging_failed_subscriptions TO sm_admin;
GRANT SELECT ON charging_failure_summary TO sm_admin;
GRANT EXECUTE ON FUNCTION identify_charging_failures TO sm_admin;
GRANT EXECUTE ON FUNCTION get_charging_failure_stats TO sm_admin;
GRANT EXECUTE ON FUNCTION get_total_charging_failures TO sm_admin;

-- =========================================
-- 11. Create rollback function
-- =========================================

CREATE OR REPLACE FUNCTION rollback_notifications_charging_migration() RETURNS VOID AS $$
BEGIN
    -- Drop views
    DROP VIEW IF EXISTS charging_failed_subscriptions CASCADE;
    DROP VIEW IF EXISTS charging_failure_summary CASCADE;
    
    -- Drop functions
    DROP FUNCTION IF EXISTS identify_charging_failures(INTEGER, INTEGER[], INTEGER);
    DROP FUNCTION IF EXISTS get_charging_failure_stats();
    DROP FUNCTION IF EXISTS get_total_charging_failures();
    DROP FUNCTION IF EXISTS rollback_notifications_charging_migration();
    
    -- Drop indexes
    DROP INDEX IF EXISTS idx_subscriptions_charge_notification;
    DROP INDEX IF EXISTS idx_subscriptions_charging_health;
    DROP INDEX IF EXISTS idx_subscriptions_optin_notification;
    DROP INDEX IF EXISTS idx_subscriptions_charging_health_status;
    DROP INDEX IF EXISTS idx_subscriptions_resubscribe_status;
    DROP INDEX IF EXISTS idx_subscriptions_charging_failure_query;
    
    -- Remove columns
    ALTER TABLE subscriptions 
    DROP COLUMN IF EXISTS last_charge_notification_at,
    DROP COLUMN IF EXISTS total_charge_notifications,
    DROP COLUMN IF EXISTS charging_health_status,
    DROP COLUMN IF EXISTS last_optin_notification_at,
    DROP COLUMN IF EXISTS charging_failure_reason;
    
    RAISE NOTICE 'Notifications charging migration rolled back successfully';
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- Migration Complete
-- =========================================

COMMENT ON FUNCTION identify_charging_failures IS 'Identify subscriptions with charging failures based on notifications';
COMMENT ON FUNCTION get_charging_failure_stats IS 'Get statistics about charging failures by category';
COMMENT ON FUNCTION get_total_charging_failures IS 'Get total count of subscriptions with charging failures';
COMMENT ON FUNCTION rollback_notifications_charging_migration IS 'Rollback function for notifications charging migration';
COMMENT ON VIEW charging_failed_subscriptions IS 'View of subscriptions with charging failures based on notifications analysis';
COMMENT ON VIEW charging_failure_summary IS 'Summary view of charging failure statistics by health status'; 