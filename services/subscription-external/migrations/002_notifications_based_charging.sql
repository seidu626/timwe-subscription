-- Migration: Notifications-Based Charging Failure Strategy
-- File: migrations/002_notifications_based_charging.sql
-- Based on FINAL_CHARGING_STRATEGY.md

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

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_subscriptions_charge_notification 
    ON subscriptions(last_charge_notification_at);

CREATE INDEX IF NOT EXISTS idx_subscriptions_charging_health 
    ON subscriptions(charging_health_status);

CREATE INDEX IF NOT EXISTS idx_subscriptions_optin_notification 
    ON subscriptions(last_optin_notification_at);

-- =========================================
-- 2. Create charging failed subscriptions view
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
-- 3. Create function to identify charging failures
-- =========================================

CREATE OR REPLACE FUNCTION identify_charging_failures(
    p_limit INTEGER DEFAULT NULL,
    p_offset INTEGER DEFAULT 0,
    p_days_threshold INTEGER DEFAULT 30
)
RETURNS TABLE (
    subscription_id INTEGER,
    msisdn VARCHAR,
    product_id INTEGER,
    entry_channel VARCHAR,
    subscription_date TIMESTAMP,
    last_charge_date TIMESTAMP,
    last_optin_date TIMESTAMP,
    days_without_charge INTEGER,
    charging_status VARCHAR,
    optin_charge_status VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    WITH charge_data AS (
        SELECT 
            n.msisdn,
            n.product_id,
            MAX(n.created_at) as last_charge
        FROM notifications n
        WHERE n.type IN ('CHARGE', 'USER_RENEWED')
        GROUP BY n.msisdn, n.product_id
    ),
    optin_data AS (
        SELECT 
            n.msisdn,
            n.product_id,
            MAX(n.created_at) as last_optin
        FROM notifications n
        WHERE n.type = 'USER_OPTIN'
        GROUP BY n.msisdn, n.product_id
    )
    SELECT 
        s.id as subscription_id,
        s.user_identifier as msisdn,
        s.product_id,
        s.entry_channel,
        s.created_at as subscription_date,
        cd.last_charge as last_charge_date,
        od.last_optin as last_optin_date,
        CASE 
            WHEN cd.last_charge IS NULL THEN 
                EXTRACT(DAY FROM NOW() - s.created_at)::INTEGER
            ELSE 
                EXTRACT(DAY FROM NOW() - cd.last_charge)::INTEGER
        END as days_without_charge,
        CASE
            WHEN cd.last_charge IS NULL THEN 'NEVER_CHARGED'
            WHEN cd.last_charge < NOW() - (p_days_threshold || ' days')::INTERVAL THEN 'STALE_CHARGE'
            ELSE 'RECENT_CHARGE'
        END as charging_status,
        CASE
            WHEN od.last_optin IS NULL THEN 'NO_OPTIN'
            WHEN cd.last_charge IS NULL THEN 'OPTIN_NO_CHARGE'
            WHEN cd.last_charge < NOW() - (p_days_threshold || ' days')::INTERVAL THEN 'OPTIN_STALE_CHARGE'
            ELSE 'OPTIN_RECENT_CHARGE'
        END as optin_charge_status
    FROM subscriptions s
    LEFT JOIN charge_data cd ON s.user_identifier = cd.msisdn 
                              AND s.product_id = cd.product_id
    LEFT JOIN optin_data od ON s.user_identifier = od.msisdn 
                              AND s.product_id = od.product_id
    WHERE (s.status = 'active' OR s.status IS NULL)
      AND s.created_at < NOW() - INTERVAL '1 day'
      AND (cd.last_charge IS NULL 
           OR cd.last_charge < NOW() - (p_days_threshold || ' days')::INTERVAL)
      AND (s.resubscribe_status IS NULL OR s.resubscribe_status != 'completed')
    ORDER BY days_without_charge DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 4. Create function to get charging failure statistics
-- =========================================

CREATE OR REPLACE FUNCTION get_charging_failure_stats()
RETURNS TABLE (
    metric VARCHAR,
    count BIGINT,
    percentage NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    WITH charge_analysis AS (
        SELECT 
            s.user_identifier as msisdn,
            s.product_id,
            s.created_at as sub_date,
            (SELECT MAX(created_at) FROM notifications n 
             WHERE n.msisdn = s.user_identifier 
             AND n.product_id = s.product_id 
             AND n.type IN ('CHARGE', 'USER_RENEWED')) as last_charge,
            (SELECT MAX(created_at) FROM notifications n 
             WHERE n.msisdn = s.user_identifier 
             AND n.product_id = s.product_id 
             AND n.type = 'USER_OPTIN') as last_optin
        FROM subscriptions s
        WHERE (s.status = 'active' OR s.status IS NULL)
        AND s.created_at < NOW() - INTERVAL '1 day'
    ),
    categorized AS (
        SELECT 
            CASE
                WHEN last_charge IS NULL AND last_optin IS NULL THEN 'No Optin, No Charge'
                WHEN last_charge IS NULL AND last_optin IS NOT NULL THEN 'Optin, No Charge'
                WHEN last_charge < NOW() - INTERVAL '60 days' THEN '> 60 days ago'
                WHEN last_charge < NOW() - INTERVAL '30 days' THEN '30-60 days ago'
                WHEN last_charge < NOW() - INTERVAL '7 days' THEN '7-30 days ago'
                ELSE '< 7 days ago'
            END as category,
            COUNT(*) as cnt
        FROM charge_analysis
        WHERE last_charge IS NULL OR last_charge < NOW() - INTERVAL '30 days'
        GROUP BY 
            CASE
                WHEN last_charge IS NULL AND last_optin IS NULL THEN 'No Optin, No Charge'
                WHEN last_charge IS NULL AND last_optin IS NOT NULL THEN 'Optin, No Charge'
                WHEN last_charge < NOW() - INTERVAL '60 days' THEN '> 60 days ago'
                WHEN last_charge < NOW() - INTERVAL '30 days' THEN '30-60 days ago'
                WHEN last_charge < NOW() - INTERVAL '7 days' THEN '7-30 days ago'
                ELSE '< 7 days ago'
            END
    ),
    total_failures AS (
        SELECT SUM(cnt) as total FROM categorized
    )
    SELECT 
        c.category as metric,
        c.cnt as count,
        ROUND(c.cnt::numeric / tf.total * 100, 2) as percentage
    FROM categorized c
    CROSS JOIN total_failures tf
    ORDER BY 
        CASE c.category
            WHEN 'No Optin, No Charge' THEN 1
            WHEN 'Optin, No Charge' THEN 2
            WHEN '> 60 days ago' THEN 3
            WHEN '30-60 days ago' THEN 4
            WHEN '7-30 days ago' THEN 5
            ELSE 6
        END;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 5. Create function to get total charging failure count
-- =========================================

CREATE OR REPLACE FUNCTION get_total_charging_failures()
RETURNS BIGINT AS $$
DECLARE
    failure_count BIGINT;
BEGIN
    SELECT COUNT(DISTINCT s.id)
    INTO failure_count
    FROM subscriptions s
    LEFT JOIN LATERAL (
        SELECT MAX(created_at) as last_charge
        FROM notifications n
        WHERE n.msisdn = s.user_identifier 
        AND n.product_id = s.product_id
        AND n.type IN ('CHARGE', 'USER_RENEWED')
    ) ch ON true
    WHERE (s.status = 'active' OR s.status IS NULL)
    AND s.created_at < NOW() - INTERVAL '1 day'
    AND (ch.last_charge IS NULL OR ch.last_charge < NOW() - INTERVAL '30 days')
    AND (s.resubscribe_status IS NULL OR s.resubscribe_status != 'completed');
    
    RETURN failure_count;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 6. Update existing records with charging data
-- =========================================

-- Update subscriptions with charging notification data
UPDATE subscriptions s
SET 
    last_charge_notification_at = ch.last_charge,
    total_charge_notifications = ch.total_charges,
    last_optin_notification_at = oh.last_optin,
    charging_health_status = CASE
        WHEN ch.last_charge IS NULL THEN 'NEVER_CHARGED'
        WHEN ch.last_charge < NOW() - INTERVAL '30 days' THEN 'STALE'
        WHEN ch.last_charge < NOW() - INTERVAL '7 days' THEN 'DELAYED'
        ELSE 'HEALTHY'
    END
FROM (
    SELECT 
        msisdn,
        product_id,
        MAX(created_at) as last_charge,
        COUNT(*) as total_charges
    FROM notifications
    WHERE type IN ('CHARGE', 'USER_RENEWED')
    GROUP BY msisdn, product_id
) ch
LEFT JOIN (
    SELECT 
        msisdn,
        product_id,
        MAX(created_at) as last_optin
    FROM notifications
    WHERE type = 'USER_OPTIN'
    GROUP BY msisdn, product_id
) oh ON ch.msisdn = oh.msisdn AND ch.product_id = oh.product_id
WHERE s.user_identifier = ch.msisdn 
  AND s.product_id = ch.product_id;

-- Update subscriptions without any charge notifications
UPDATE subscriptions s
SET 
    charging_health_status = 'NEVER_CHARGED',
    charging_failure_reason = CASE
        WHEN EXISTS (
            SELECT 1 FROM notifications n 
            WHERE n.msisdn = s.user_identifier 
            AND n.product_id = s.product_id 
            AND n.type = 'USER_OPTIN'
        ) THEN 'OPTIN_NO_CHARGE'
        ELSE 'NO_OPTIN_NO_CHARGE'
    END
WHERE s.last_charge_notification_at IS NULL
  AND (s.status = 'active' OR s.status IS NULL)
  AND s.created_at < NOW() - INTERVAL '1 day';

-- =========================================
-- 7. Create indexes for performance
-- =========================================

-- Index for charging health status queries
CREATE INDEX IF NOT EXISTS idx_subscriptions_charging_health_status 
    ON subscriptions(charging_health_status) 
    WHERE charging_health_status IN ('NEVER_CHARGED', 'STALE', 'DELAYED');

-- Index for resubscribe status queries
CREATE INDEX IF NOT EXISTS idx_subscriptions_resubscribe_status 
    ON subscriptions(resubscribe_status) 
    WHERE resubscribe_status IS NULL OR resubscribe_status != 'completed';

-- Composite index for efficient charging failure queries
CREATE INDEX IF NOT EXISTS idx_subscriptions_charging_failure_query 
    ON subscriptions(status, created_at, charging_health_status, resubscribe_status);

-- =========================================
-- 8. Create summary view for monitoring
-- =========================================

CREATE OR REPLACE VIEW charging_failure_summary AS
SELECT 
    'Total Active Subscriptions' as metric,
    COUNT(*) as count,
    100.0 as percentage
FROM subscriptions
WHERE (status = 'active' OR status IS NULL)
AND created_at < NOW() - INTERVAL '1 day'

UNION ALL

SELECT 
    'Never Received Charge Notification' as metric,
    COUNT(DISTINCT s.id) as count,
    ROUND(COUNT(DISTINCT s.id)::numeric / 
          (SELECT COUNT(*) FROM subscriptions WHERE (status = 'active' OR status IS NULL) AND created_at < NOW() - INTERVAL '1 day') * 100, 2) as percentage
FROM subscriptions s
LEFT JOIN notifications n ON s.user_identifier = n.msisdn 
    AND s.product_id = n.product_id
    AND n.type IN ('CHARGE', 'USER_RENEWED')
WHERE (s.status = 'active' OR s.status IS NULL)
AND s.created_at < NOW() - INTERVAL '1 day'
AND n.id IS NULL

UNION ALL

SELECT 
    'Not Charged in 30+ Days' as metric,
    COUNT(DISTINCT s.id) as count,
    ROUND(COUNT(DISTINCT s.id)::numeric / 
          (SELECT COUNT(*) FROM subscriptions WHERE (status = 'active' OR status IS NULL) AND created_at < NOW() - INTERVAL '1 day') * 100, 2) as percentage
FROM subscriptions s
LEFT JOIN LATERAL (
    SELECT MAX(created_at) as last_charge
    FROM notifications n
    WHERE n.msisdn = s.user_identifier 
    AND n.product_id = s.product_id
    AND n.type IN ('CHARGE', 'USER_RENEWED')
) ch ON true
WHERE (s.status = 'active' OR s.status IS NULL)
AND s.created_at < NOW() - INTERVAL '1 day'
AND ch.last_charge < NOW() - INTERVAL '30 days'

UNION ALL

SELECT 
    'TOTAL CHARGING FAILURES' as metric,
    COUNT(DISTINCT s.id) as count,
    ROUND(COUNT(DISTINCT s.id)::numeric / 
          (SELECT COUNT(*) FROM subscriptions WHERE (status = 'active' OR status IS NULL) AND created_at < NOW() - INTERVAL '1 day') * 100, 2) as percentage
FROM subscriptions s
LEFT JOIN LATERAL (
    SELECT MAX(created_at) as last_charge
    FROM notifications n
    WHERE n.msisdn = s.user_identifier 
    AND n.product_id = s.product_id
    AND n.type IN ('CHARGE', 'USER_RENEWED')
) ch ON true
WHERE (s.status = 'active' OR s.status IS NULL)
AND s.created_at < NOW() - INTERVAL '1 day'
AND (ch.last_charge IS NULL OR ch.last_charge < NOW() - INTERVAL '30 days')
AND (s.resubscribe_status IS NULL OR s.resubscribe_status != 'completed');

-- =========================================
-- 9. Grant permissions
-- =========================================

-- Grant permissions to the view and functions
GRANT SELECT ON charging_failed_subscriptions TO sm_admin;
GRANT SELECT ON charging_failure_summary TO sm_admin;
GRANT EXECUTE ON FUNCTION identify_charging_failures TO sm_admin;
GRANT EXECUTE ON FUNCTION get_charging_failure_stats TO sm_admin;
GRANT EXECUTE ON FUNCTION get_total_charging_failures TO sm_admin;

-- =========================================
-- 10. Create rollback function
-- =========================================

CREATE OR REPLACE FUNCTION rollback_notifications_charging_migration()
RETURNS VOID AS $$
BEGIN
    -- Drop views
    DROP VIEW IF EXISTS charging_failed_subscriptions CASCADE;
    DROP VIEW IF EXISTS charging_failure_summary CASCADE;
    
    -- Drop functions
    DROP FUNCTION IF EXISTS identify_charging_failures;
    DROP FUNCTION IF EXISTS get_charging_failure_stats;
    DROP FUNCTION IF EXISTS get_total_charging_failures;
    DROP FUNCTION IF EXISTS rollback_notifications_charging_migration;
    
    -- Drop indexes
    DROP INDEX IF EXISTS idx_subscriptions_charge_notification;
    DROP INDEX IF EXISTS idx_subscriptions_charging_health;
    DROP INDEX IF EXISTS idx_subscriptions_optin_notification;
    DROP INDEX IF EXISTS idx_subscriptions_charging_health_status;
    DROP INDEX IF EXISTS idx_subscriptions_resubscribe_status;
    DROP INDEX IF EXISTS idx_subscriptions_charging_failure_query;
    
    -- Remove columns
    ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_charge_notification_at;
    ALTER TABLE subscriptions DROP COLUMN IF EXISTS total_charge_notifications;
    ALTER TABLE subscriptions DROP COLUMN IF EXISTS charging_health_status;
    ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_optin_notification_at;
    ALTER TABLE subscriptions DROP COLUMN IF EXISTS charging_failure_reason;
    
    RAISE NOTICE 'Rollback completed successfully';
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- Migration Complete
-- =========================================

-- Display migration summary
DO $$
BEGIN
    RAISE NOTICE 'Notifications-based charging migration completed successfully!';
    RAISE NOTICE 'New views created: charging_failed_subscriptions, charging_failure_summary';
    RAISE NOTICE 'New functions created: identify_charging_failures, get_charging_failure_stats, get_total_charging_failures';
    RAISE NOTICE 'New columns added to subscriptions table for charging notification tracking';
    RAISE NOTICE 'Use rollback_notifications_charging_migration() to undo changes if needed';
END $$; 