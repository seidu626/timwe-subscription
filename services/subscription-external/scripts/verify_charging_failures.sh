#!/bin/bash
# Quick script to verify charging failure count
# File: scripts/verify_charging_failures.sh

set -e

DB_HOST="${DB_HOST:-localhost}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"

echo "========================================="
echo "  VERIFYING CHARGING FAILURE COUNT"
echo "========================================="
echo ""

psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" << EOF

-- Summary of charging failures
WITH summary AS (
    SELECT 
        'Total Active Subscriptions' as category,
        COUNT(DISTINCT s.id) as count
    FROM subscriptions s
    WHERE s.status = 'active' OR s.status IS NULL
    
    UNION ALL
    
    SELECT 
        'Never Received CHARGE Notification' as category,
        COUNT(DISTINCT s.id) as count
    FROM subscriptions s
    LEFT JOIN notifications n ON s.user_identifier = n.msisdn 
        AND s.product_id = n.product_id
        AND n.type IN ('CHARGE', 'USER_RENEWED')
    WHERE (s.status = 'active' OR s.status IS NULL)
        AND s.created_at < NOW() - INTERVAL '1 day'
        AND n.id IS NULL
    
    UNION ALL
    
    SELECT 
        'Not Charged in Last 30 Days' as category,
        COUNT(DISTINCT s.id) as count
    FROM subscriptions s
    INNER JOIN (
        SELECT msisdn, product_id, MAX(created_at) as last_charge
        FROM notifications
        WHERE type IN ('CHARGE', 'USER_RENEWED')
        GROUP BY msisdn, product_id
        HAVING MAX(created_at) < NOW() - INTERVAL '30 days'
    ) old_charges ON s.user_identifier = old_charges.msisdn 
                   AND s.product_id = old_charges.product_id
    WHERE s.status = 'active' OR s.status IS NULL
    
    UNION ALL
    
    SELECT 
        '*** TOTAL CHARGING FAILURES ***' as category,
        COUNT(DISTINCT s.id) as count
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
)
SELECT 
    category,
    TO_CHAR(count, 'FM999,999,999') as formatted_count
FROM summary
ORDER BY 
    CASE category
        WHEN 'Total Active Subscriptions' THEN 1
        WHEN 'Never Received CHARGE Notification' THEN 2
        WHEN 'Not Charged in Last 30 Days' THEN 3
        ELSE 4
    END;

EOF

echo ""
echo "If TOTAL CHARGING FAILURES ≈ 25,169,944, we've identified the issue correctly!"
echo ""
