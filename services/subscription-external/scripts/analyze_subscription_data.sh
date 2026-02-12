#!/bin/bash
# Script to analyze what subscription data we actually have
# File: scripts/analyze_subscription_data.sh

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================="
echo "    SUBSCRIPTION DATA ANALYSIS"
echo "========================================="
echo ""

# Function to run query
run_query() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "$1"
}

# Function to run detailed query
run_detailed_query() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$1"
}

echo -e "${BLUE}1. SUBSCRIPTION STATISTICS${NC}"
echo "----------------------------------------"

# Total subscriptions
TOTAL_SUBS=$(run_query "SELECT COUNT(*) FROM subscriptions" | tr -d ' ')
echo "Total Subscriptions: $TOTAL_SUBS"

# Active subscriptions
ACTIVE_SUBS=$(run_query "SELECT COUNT(*) FROM subscriptions WHERE status = 'active' OR status IS NULL" | tr -d ' ')
echo "Active Subscriptions: $ACTIVE_SUBS"

# Subscriptions by status
echo ""
echo "Subscriptions by Status:"
run_detailed_query "
    SELECT 
        COALESCE(status, 'NULL') as status,
        COUNT(*) as count,
        ROUND(COUNT(*)::numeric / (SELECT COUNT(*) FROM subscriptions) * 100, 2) as percentage
    FROM subscriptions
    GROUP BY status
    ORDER BY count DESC
"

echo ""
echo -e "${BLUE}2. SUBSCRIPTION AGE ANALYSIS${NC}"
echo "----------------------------------------"

run_detailed_query "
    SELECT 
        CASE 
            WHEN created_at > NOW() - INTERVAL '7 days' THEN '< 1 week'
            WHEN created_at > NOW() - INTERVAL '30 days' THEN '1 week - 1 month'
            WHEN created_at > NOW() - INTERVAL '90 days' THEN '1-3 months'
            WHEN created_at > NOW() - INTERVAL '180 days' THEN '3-6 months'
            ELSE '> 6 months'
        END as age_group,
        COUNT(*) as count
    FROM subscriptions
    WHERE status = 'active' OR status IS NULL
    GROUP BY age_group
    ORDER BY 
        CASE age_group
            WHEN '< 1 week' THEN 1
            WHEN '1 week - 1 month' THEN 2
            WHEN '1-3 months' THEN 3
            WHEN '3-6 months' THEN 4
            ELSE 5
        END
"

echo ""
echo -e "${BLUE}3. INVALID MSISDN LOGS ANALYSIS${NC}"
echo "----------------------------------------"

# Check what response codes we have
echo "Response Codes in invalid_msisdn_logs:"
run_detailed_query "
    SELECT 
        response_code,
        COUNT(*) as count
    FROM invalid_msisdn_logs
    WHERE response_code IS NOT NULL
    GROUP BY response_code
    ORDER BY count DESC
    LIMIT 20
"

echo ""
echo "Response Messages containing 'charging' or 'billing':"
CHARGING_MSGS=$(run_query "
    SELECT COUNT(*) 
    FROM invalid_msisdn_logs 
    WHERE response_message LIKE '%charging%' 
       OR response_message LIKE '%billing%'
       OR response_message LIKE '%CHARGING%'
       OR response_message LIKE '%BILLING%'
" | tr -d ' ')
echo "Count: $CHARGING_MSGS"

if [ "$CHARGING_MSGS" -gt "0" ]; then
    echo "Sample messages:"
    run_detailed_query "
        SELECT DISTINCT response_message
        FROM invalid_msisdn_logs
        WHERE response_message LIKE '%charging%' 
           OR response_message LIKE '%billing%'
        LIMIT 5
    "
fi

echo ""
echo -e "${BLUE}4. TIMWE API RESPONSES${NC}"
echo "----------------------------------------"

echo "Looking for TIMWE status responses:"
run_detailed_query "
    SELECT 
        response_code,
        COUNT(*) as count
    FROM invalid_msisdn_logs
    WHERE response_code IN (
        'OPTIN_ACTIVE_WAIT_CHARGING',
        'OPTIN_ALREADY_ACTIVE',
        'OPTIN_CONFIG_NOT_FOUND',
        'ALREADY_ACTIVE',
        'ACTIVE_WAIT_CHARGING'
    )
    GROUP BY response_code
    ORDER BY count DESC
"

echo ""
echo -e "${BLUE}5. SUBSCRIPTION PATTERNS${NC}"
echo "----------------------------------------"

echo "Subscriptions with multiple products:"
run_detailed_query "
    SELECT 
        COUNT(*) as msisdn_count,
        products_per_user
    FROM (
        SELECT 
            user_identifier,
            COUNT(DISTINCT product_id) as products_per_user
        FROM subscriptions
        WHERE status = 'active' OR status IS NULL
        GROUP BY user_identifier
    ) t
    GROUP BY products_per_user
    ORDER BY products_per_user
"

echo ""
echo "Top Products by Subscription Count:"
run_detailed_query "
    SELECT 
        s.product_id,
        p.name as product_name,
        COUNT(*) as subscription_count
    FROM subscriptions s
    LEFT JOIN products p ON s.product_id::text = p.product_id
    WHERE s.status = 'active' OR s.status IS NULL
    GROUP BY s.product_id, p.name
    ORDER BY subscription_count DESC
    LIMIT 10
"

echo ""
echo -e "${BLUE}6. POTENTIAL INDICATORS OF ISSUES${NC}"
echo "----------------------------------------"

# Check for old subscriptions with no recent activity
echo "Active subscriptions older than 30 days:"
OLD_ACTIVE=$(run_query "
    SELECT COUNT(*) 
    FROM subscriptions
    WHERE (status = 'active' OR status IS NULL)
    AND created_at < NOW() - INTERVAL '30 days'
" | tr -d ' ')
echo "Count: $OLD_ACTIVE"

echo ""
echo "Subscriptions by entry channel:"
run_detailed_query "
    SELECT 
        entry_channel,
        COUNT(*) as count
    FROM subscriptions
    WHERE status = 'active' OR status IS NULL
    GROUP BY entry_channel
    ORDER BY count DESC
"

echo ""
echo -e "${BLUE}7. DATA QUALITY CHECK${NC}"
echo "----------------------------------------"

# Check for any charging-related columns
echo "Checking for charging-related columns in subscriptions table:"
run_detailed_query "
    SELECT 
        column_name,
        data_type,
        is_nullable
    FROM information_schema.columns
    WHERE table_name = 'subscriptions'
    AND (
        column_name LIKE '%charg%'
        OR column_name LIKE '%bill%'
        OR column_name LIKE '%payment%'
        OR column_name LIKE '%revenue%'
    )
"

# Check notifications table
echo ""
echo "Checking notifications table for charging-related entries:"
NOTIF_COUNT=$(run_query "SELECT COUNT(*) FROM notifications WHERE type LIKE '%CHARG%' OR type LIKE '%BILL%'" 2>/dev/null | tr -d ' ') || NOTIF_COUNT=0
if [ "$NOTIF_COUNT" -gt "0" ]; then
    echo "Found $NOTIF_COUNT charging-related notifications"
    run_detailed_query "
        SELECT type, COUNT(*) as count
        FROM notifications
        WHERE type LIKE '%CHARG%' OR type LIKE '%BILL%'
        GROUP BY type
    "
else
    echo "No charging-related notifications found"
fi

echo ""
echo -e "${BLUE}8. SUMMARY${NC}"
echo "----------------------------------------"

echo -e "${YELLOW}Key Findings:${NC}"
echo ""

# Determine if we can identify charging failures
CAN_IDENTIFY_FAILURES=false

if [ "$CHARGING_MSGS" -gt "0" ]; then
    echo "✓ Found $CHARGING_MSGS messages mentioning charging/billing"
    CAN_IDENTIFY_FAILURES=true
else
    echo "✗ No charging/billing failure messages in invalid_msisdn_logs"
fi

# Check for WAIT_CHARGING status
WAIT_CHARGING=$(run_query "
    SELECT COUNT(*) 
    FROM invalid_msisdn_logs 
    WHERE response_code = 'OPTIN_ACTIVE_WAIT_CHARGING'
" | tr -d ' ')

if [ "$WAIT_CHARGING" -gt "0" ]; then
    echo "⚠ Found $WAIT_CHARGING subscriptions with OPTIN_ACTIVE_WAIT_CHARGING status"
    echo "  These MAY have charging issues but cannot be confirmed"
fi

echo ""
if [ "$CAN_IDENTIFY_FAILURES" = true ]; then
    echo -e "${GREEN}✅ Some charging failures can be identified from existing data${NC}"
else
    echo -e "${RED}❌ Cannot reliably identify charging failures from existing data${NC}"
    echo ""
    echo -e "${YELLOW}Recommendations:${NC}"
    echo "1. Request failed MSISDN list from TIMWE"
    echo "2. Check if billing system has CDR data"
    echo "3. Analyze revenue reports to identify non-paying subscriptions"
    echo "4. Consider querying TIMWE API for subscription status"
fi

echo ""
echo "Total Active Subscriptions: $ACTIVE_SUBS"
echo "Subscriptions > 30 days old: $OLD_ACTIVE"
echo "Potential failure rate: $(echo "scale=2; $OLD_ACTIVE * 100 / $ACTIVE_SUBS" | bc)%"

echo ""
echo "========================================="
echo "         ANALYSIS COMPLETE"
echo "========================================="
