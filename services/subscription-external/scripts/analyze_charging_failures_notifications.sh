#!/bin/bash
# Analyze Charging Failures Using Notifications Table
# File: scripts/analyze_charging_failures_notifications.sh
# Based on FINAL_CHARGING_STRATEGY.md

set -e

# Configuration
DB_HOST="${DB_HOST:-139.59.135.253}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
OUTPUT_DIR="/tmp/charging_analysis_$(date +%Y%m%d_%H%M%S)"
LOG_FILE="$OUTPUT_DIR/analysis.log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Function to log messages
log_message() {
    local level=$1
    local message=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        "INFO") echo -e "${BLUE}[$timestamp] INFO: $message${NC}" | tee -a "$LOG_FILE" ;;
        "SUCCESS") echo -e "${GREEN}[$timestamp] SUCCESS: $message${NC}" | tee -a "$LOG_FILE" ;;
        "WARNING") echo -e "${YELLOW}[$timestamp] WARNING: $message${NC}" | tee -a "$LOG_FILE" ;;
        "ERROR") echo -e "${RED}[$timestamp] ERROR: $message${NC}" | tee -a "$LOG_FILE" ;;
        "CRITICAL") echo -e "${PURPLE}[$timestamp] CRITICAL: $message${NC}" | tee -a "$LOG_FILE" ;;
    esac
}

# Function to run SQL query and save results
run_query() {
    local query_name="$1"
    local sql_query="$2"
    local output_file="$3"
    
    log_message "INFO" "Running query: $query_name"
    
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        -c "$sql_query" > "$output_file" 2>&1; then
        log_message "SUCCESS" "Query completed: $query_name"
        return 0
    else
        log_message "ERROR" "Query failed: $query_name"
        return 1
    fi
}

# Function to check database connectivity
check_database_connectivity() {
    log_message "INFO" "Checking database connectivity..."
    
    if pg_isready -h "$DB_HOST" -p "$DB_PORT" > /dev/null 2>&1; then
        log_message "SUCCESS" "Database is accessible at $DB_HOST:$DB_PORT"
    else
        log_message "ERROR" "Cannot connect to database at $DB_HOST:$DB_PORT"
        exit 1
    fi
    
    # Test actual connection with credentials
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
        log_message "SUCCESS" "Database authentication successful"
    else
        log_message "ERROR" "Database authentication failed for user $DB_USER"
        exit 1
    fi
}

# Function to check if notifications table exists
check_notifications_table() {
    log_message "INFO" "Checking if notifications table exists..."
    
    local table_exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT EXISTS(
            SELECT 1 FROM information_schema.tables 
            WHERE table_schema = 'public' 
            AND table_name = 'notifications'
        );
    " | tr -d ' ')
    
    if [ "$table_exists" = "t" ]; then
        log_message "SUCCESS" "Notifications table exists"
        return 0
    else
        log_message "ERROR" "Notifications table does not exist"
        return 1
    fi
}

# Function to analyze notification types
analyze_notification_types() {
    log_message "INFO" "Analyzing notification types in notifications table..."
    
    local query="
        SELECT 
            type,
            COUNT(*) as count,
            ROUND(COUNT(*)::numeric / SUM(COUNT(*)) OVER() * 100, 2) as percentage,
            MIN(created_at) as earliest,
            MAX(created_at) as latest
        FROM notifications
        GROUP BY type
        ORDER BY count DESC;
    "
    
    run_query "Notification Types Analysis" "$query" "$OUTPUT_DIR/01_notification_types.txt"
}

# Function to analyze charging failures using notifications
analyze_charging_failures_notifications() {
    log_message "INFO" "Analyzing charging failures using notifications table..."
    
    # Query 1: Basic charging failure analysis
    local query1="
        WITH charge_analysis AS (
            SELECT 
                s.user_identifier as msisdn,
                s.product_id,
                s.created_at as sub_date,
                s.status,
                s.entry_channel,
                -- Check for OPTIN notification
                EXISTS(
                    SELECT 1 FROM notifications n 
                    WHERE n.msisdn = s.user_identifier 
                    AND n.product_id = s.product_id 
                    AND n.type = 'USER_OPTIN'
                ) as has_optin,
                -- Check for CHARGE notification
                EXISTS(
                    SELECT 1 FROM notifications n 
                    WHERE n.msisdn = s.user_identifier 
                    AND n.product_id = s.product_id 
                    AND n.type IN ('CHARGE', 'USER_RENEWED')
                ) as has_charge,
                -- Get last charge date
                (
                    SELECT MAX(n.created_at) 
                    FROM notifications n 
                    WHERE n.msisdn = s.user_identifier 
                    AND n.product_id = s.product_id 
                    AND n.type IN ('CHARGE', 'USER_RENEWED')
                ) as last_charge_date
            FROM subscriptions s
            WHERE (s.status = 'active' OR s.status IS NULL)
            AND s.created_at < NOW() - INTERVAL '1 day'
        )
        SELECT 
            COUNT(*) as total_active,
            SUM(CASE WHEN has_optin THEN 1 ELSE 0 END) as with_optin,
            SUM(CASE WHEN has_charge THEN 1 ELSE 0 END) as with_charge,
            SUM(CASE WHEN has_optin AND NOT has_charge THEN 1 ELSE 0 END) as optin_no_charge,
            SUM(CASE WHEN NOT has_optin AND NOT has_charge THEN 1 ELSE 0 END) as no_optin_no_charge,
            SUM(CASE WHEN last_charge_date < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END) as stale_charges
        FROM charge_analysis;
    "
    
    run_query "Charging Failure Analysis" "$query1" "$OUTPUT_DIR/02_charging_failure_analysis.txt"
    
    # Query 2: Detailed breakdown by charging status
    local query2="
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
        )
        SELECT 
            CASE
                WHEN last_charge IS NULL AND last_optin IS NULL THEN 'No Optin, No Charge'
                WHEN last_charge IS NULL AND last_optin IS NOT NULL THEN 'Optin, No Charge'
                WHEN last_charge < NOW() - INTERVAL '60 days' THEN '> 60 days ago'
                WHEN last_charge < NOW() - INTERVAL '30 days' THEN '30-60 days ago'
                WHEN last_charge < NOW() - INTERVAL '7 days' THEN '7-30 days ago'
                ELSE '< 7 days ago'
            END as last_charge_category,
            COUNT(*) as subscription_count,
            ROUND(COUNT(*)::numeric / (SELECT COUNT(*) FROM charge_analysis) * 100, 2) as percentage
        FROM charge_analysis
        GROUP BY 
            CASE
                WHEN last_charge IS NULL AND last_optin IS NULL THEN 'No Optin, No Charge'
                WHEN last_charge IS NULL AND last_optin IS NOT NULL THEN 'Optin, No Charge'
                WHEN last_charge < NOW() - INTERVAL '60 days' THEN '> 60 days ago'
                WHEN last_charge < NOW() - INTERVAL '30 days' THEN '30-60 days ago'
                WHEN last_charge < NOW() - INTERVAL '7 days' THEN '7-30 days ago'
                ELSE '< 7 days ago'
            END
        ORDER BY 
            CASE last_charge_category
                WHEN 'No Optin, No Charge' THEN 1
                WHEN 'Optin, No Charge' THEN 2
                WHEN '> 60 days ago' THEN 3
                WHEN '30-60 days ago' THEN 4
                WHEN '7-30 days ago' THEN 5
                ELSE 6
            END;
    "
    
    run_query "Charging Status Breakdown" "$query2" "$OUTPUT_DIR/03_charging_status_breakdown.txt"
    
    # Query 3: Get total count of failed charging
    local query3="
        SELECT COUNT(DISTINCT s.id) as total_failed_charging
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
        AND (ch.last_charge IS NULL OR ch.last_charge < NOW() - INTERVAL '30 days');
    "
    
    run_query "Total Failed Charging Count" "$query3" "$OUTPUT_DIR/04_total_failed_charging.txt"
}

# Function to analyze by product
analyze_by_product() {
    log_message "INFO" "Analyzing charging failures by product..."
    
    local query="
        WITH charge_analysis AS (
            SELECT 
                s.user_identifier as msisdn,
                s.product_id,
                p.name as product_name,
                (SELECT MAX(created_at) FROM notifications n 
                 WHERE n.msisdn = s.user_identifier 
                 AND n.product_id = s.product_id 
                 AND n.type IN ('CHARGE', 'USER_RENEWED')) as last_charge
            FROM subscriptions s
            JOIN products p ON s.product_id::text = p.product_id
            WHERE (s.status = 'active' OR s.status IS NULL)
            AND s.created_at < NOW() - INTERVAL '1 day'
        )
        SELECT 
            product_id,
            product_name,
            COUNT(*) as total_subscriptions,
            SUM(CASE WHEN last_charge IS NULL THEN 1 ELSE 0 END) as never_charged,
            SUM(CASE WHEN last_charge < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END) as stale_charges,
            ROUND(SUM(CASE WHEN last_charge IS NULL OR last_charge < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END)::numeric / COUNT(*) * 100, 2) as failure_percentage
        FROM charge_analysis
        GROUP BY product_id, product_name
        ORDER BY failure_percentage DESC, total_subscriptions DESC;
    "
    
    run_query "Product Analysis" "$query" "$OUTPUT_DIR/05_product_analysis.txt"
}

# Function to analyze by entry channel
analyze_by_entry_channel() {
    log_message "INFO" "Analyzing charging failures by entry channel..."
    
    local query="
        WITH charge_analysis AS (
            SELECT 
                s.user_identifier as msisdn,
                s.product_id,
                s.entry_channel,
                (SELECT MAX(created_at) FROM notifications n 
                 WHERE n.msisdn = s.user_identifier 
                 AND n.product_id = s.product_id 
                 AND n.type IN ('CHARGE', 'USER_RENEWED')) as last_charge
            FROM subscriptions s
            WHERE (s.status = 'active' OR s.status IS NULL)
            AND s.created_at < NOW() - INTERVAL '1 day'
        )
        SELECT 
            entry_channel,
            COUNT(*) as total_subscriptions,
            SUM(CASE WHEN last_charge IS NULL THEN 1 ELSE 0 END) as never_charged,
            SUM(CASE WHEN last_charge < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END) as stale_charges,
            ROUND(SUM(CASE WHEN last_charge IS NULL OR last_charge < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END)::numeric / COUNT(*) * 100, 2) as failure_percentage
        FROM charge_analysis
        GROUP BY entry_channel
        ORDER BY failure_percentage DESC, total_subscriptions DESC;
    "
    
    run_query "Entry Channel Analysis" "$query" "$OUTPUT_DIR/06_entry_channel_analysis.txt"
}

# Function to analyze by date ranges
analyze_by_date_ranges() {
    log_message "INFO" "Analyzing charging failures by date ranges..."
    
    local query="
        WITH charge_analysis AS (
            SELECT 
                s.user_identifier as msisdn,
                s.product_id,
                s.created_at as subscription_date,
                (SELECT MAX(created_at) FROM notifications n 
                 WHERE n.msisdn = s.user_identifier 
                 AND n.product_id = s.product_id 
                 AND n.type IN ('CHARGE', 'USER_RENEWED')) as last_charge
            FROM subscriptions s
            WHERE (s.status = 'active' OR s.status IS NULL)
            AND s.created_at < NOW() - INTERVAL '1 day'
        )
        SELECT 
            CASE
                WHEN subscription_date >= NOW() - INTERVAL '7 days' THEN 'Last 7 days'
                WHEN subscription_date >= NOW() - INTERVAL '30 days' THEN 'Last 30 days'
                WHEN subscription_date >= NOW() - INTERVAL '90 days' THEN 'Last 90 days'
                WHEN subscription_date >= NOW() - INTERVAL '180 days' THEN 'Last 180 days'
                WHEN subscription_date >= NOW() - INTERVAL '365 days' THEN 'Last 365 days'
                ELSE 'Older than 1 year'
            END as subscription_period,
            COUNT(*) as total_subscriptions,
            SUM(CASE WHEN last_charge IS NULL THEN 1 ELSE 0 END) as never_charged,
            SUM(CASE WHEN last_charge < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END) as stale_charges,
            ROUND(SUM(CASE WHEN last_charge IS NULL OR last_charge < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END)::numeric / COUNT(*) * 100, 2) as failure_percentage
        FROM charge_analysis
        GROUP BY 
            CASE
                WHEN subscription_date >= NOW() - INTERVAL '7 days' THEN 'Last 7 days'
                WHEN subscription_date >= NOW() - INTERVAL '30 days' THEN 'Last 30 days'
                WHEN subscription_date >= NOW() - INTERVAL '90 days' THEN 'Last 90 days'
                WHEN subscription_date >= NOW() - INTERVAL '180 days' THEN 'Last 180 days'
                WHEN subscription_date >= NOW() - INTERVAL '365 days' THEN 'Last 365 days'
                ELSE 'Older than 1 year'
            END
        ORDER BY 
            CASE subscription_period
                WHEN 'Last 7 days' THEN 1
                WHEN 'Last 30 days' THEN 2
                WHEN 'Last 90 days' THEN 3
                WHEN 'Last 180 days' THEN 4
                WHEN 'Last 365 days' THEN 5
                ELSE 6
            END;
    "
    
    run_query "Date Range Analysis" "$query" "$OUTPUT_DIR/07_date_range_analysis.txt"
}

# Function to generate summary report
generate_summary_report() {
    log_message "INFO" "Generating summary report..."
    
    local report_file="$OUTPUT_DIR/CHARGING_FAILURE_ANALYSIS_REPORT.md"
    
    cat > "$report_file" << EOF
# Charging Failure Analysis Report
## Using Notifications Table Approach

**Generated**: $(date)  
**Database**: $DB_NAME@$DB_HOST:$DB_PORT  
**Analysis Method**: Notifications-based charging status detection

---

## Executive Summary

This report analyzes charging failures using the notifications table approach, which provides more accurate and reliable data than error logs.

### Key Findings

$(cat "$OUTPUT_DIR/02_charging_failure_analysis.txt" | grep -E "total_active|with_optin|with_charge|optin_no_charge|no_optin_no_charge|stale_charges" | sed 's/^/- /')

---

## Detailed Analysis

### 1. Notification Types Distribution
\`\`\`
$(cat "$OUTPUT_DIR/01_notification_types.txt")
\`\`\`

### 2. Charging Failure Breakdown
\`\`\`
$(cat "$OUTPUT_DIR/03_charging_status_breakdown.txt")
\`\`\`

### 3. Product Analysis
\`\`\`
$(cat "$OUTPUT_DIR/05_product_analysis.txt")
\`\`\`

### 4. Entry Channel Analysis
\`\`\`
$(cat "$OUTPUT_DIR/06_entry_channel_analysis.txt")
\`\`\`

### 5. Date Range Analysis
\`\`\`
$(cat "$OUTPUT_DIR/07_date_range_analysis.txt")
\`\`\`

---

## Recommendations

### Immediate Actions
1. **Focus on 'Optin, No Charge' subscriptions** - These represent the highest priority for resubscription
2. **Address 'No Optin, No Charge' subscriptions** - These may indicate system issues
3. **Prioritize products with high failure rates** - Focus resources on problematic products

### Processing Strategy
1. **Start with recent failures** (Last 7-30 days) - Higher success probability
2. **Batch by product** - Process similar products together
3. **Monitor success rates** - Adjust strategy based on results

---

## Technical Notes

- **Analysis Method**: Uses notifications table with CHARGE, USER_RENEWED, and USER_OPTIN types
- **Time Threshold**: 30 days for charging staleness
- **Subscription Age**: Minimum 1 day to allow for initial charging processing
- **Status Filter**: Active or NULL status subscriptions only

---

## Files Generated

$(ls -1 "$OUTPUT_DIR"/*.txt | sed 's|.*/||' | sed 's/^/- /')

---

**Report generated by**: analyze_charging_failures_notifications.sh  
**Database**: $DB_NAME  
**Timestamp**: $(date)
EOF
    
    log_message "SUCCESS" "Summary report generated: $report_file"
}

# Function to display key metrics
display_key_metrics() {
    log_message "INFO" "Displaying key metrics..."
    
    echo ""
    echo "========================================="
    echo "  CHARGING FAILURE ANALYSIS RESULTS"
    echo "========================================="
    echo ""
    
    # Display total failed charging count
    if [ -f "$OUTPUT_DIR/04_total_failed_charging.txt" ]; then
        local total_failed=$(cat "$OUTPUT_DIR/04_total_failed_charging.txt" | grep -o '[0-9]*' | head -1)
        echo "🔴 Total Subscriptions with Charging Issues: $total_failed"
        echo ""
    fi
    
    # Display charging failure breakdown
    if [ -f "$OUTPUT_DIR/03_charging_status_breakdown.txt" ]; then
        echo "📊 Charging Failure Breakdown:"
        cat "$OUTPUT_DIR/03_charging_status_breakdown.txt" | grep -E "No Optin|Optin, No Charge|> 60 days|30-60 days|7-30 days" | head -5
        echo ""
    fi
    
    # Display product analysis summary
    if [ -f "$OUTPUT_DIR/05_product_analysis.txt" ]; then
        echo "🏷️  Top Products by Failure Rate:"
        cat "$OUTPUT_DIR/05_product_analysis.txt" | grep -v "^-" | grep -v "^$" | head -5
        echo ""
    fi
    
    echo "📋 Detailed reports available in: $OUTPUT_DIR"
    echo "📄 Summary report: $OUTPUT_DIR/CHARGING_FAILURE_ANALYSIS_REPORT.md"
}

# Main execution
main() {
    echo "========================================="
    echo "  CHARGING FAILURE ANALYSIS"
    echo "  Using Notifications Table Approach"
    echo "========================================="
    echo ""
    echo "Database: $DB_NAME@$DB_HOST:$DB_PORT"
    echo "User: $DB_USER"
    echo "Output: $OUTPUT_DIR"
    echo "Log: $LOG_FILE"
    echo ""
    
    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    
    # Initialize log file
    echo "Charging failure analysis started at $(date)" > "$LOG_FILE"
    
    # Execute analysis steps
    check_database_connectivity
    check_notifications_table
    analyze_notification_types
    analyze_charging_failures_notifications
    analyze_by_product
    analyze_by_entry_channel
    analyze_by_date_ranges
    generate_summary_report
    display_key_metrics
    
    log_message "SUCCESS" "Charging failure analysis completed successfully!"
    echo ""
    echo "✅ Analysis completed successfully!"
    echo "📋 Output directory: $OUTPUT_DIR"
    echo "📄 Summary report: $OUTPUT_DIR/CHARGING_FAILURE_ANALYSIS_REPORT.md"
    echo "📋 Log file: $LOG_FILE"
}

# Execute main function
main "$@" 