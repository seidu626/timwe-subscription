#!/bin/bash
# Monitoring script for resubscription process
# File: scripts/monitor_resubscription.sh

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
BATCH_ID="${1:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to run PostgreSQL queries
run_query() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "$1"
}

# Function to display progress bar
show_progress_bar() {
    local progress=$1
    local total=50
    local filled=$((progress * total / 100))
    local empty=$((total - filled))
    
    printf "["
    printf "%${filled}s" | tr ' ' '='
    printf "%${empty}s" | tr ' ' '-'
    printf "] %d%%\n" "$progress"
}

# Main monitoring loop
monitor_batch() {
    clear
    echo "========================================="
    echo "   RESUBSCRIPTION PROCESS MONITOR"
    echo "========================================="
    echo ""
    
    # Get batch information
    if [ -z "$BATCH_ID" ]; then
        # Get the latest active batch
        BATCH_ID=$(run_query "SELECT batch_id FROM resubscription_checkpoints WHERE status = 'in_progress' ORDER BY started_at DESC LIMIT 1" | tr -d ' ')
        
        if [ -z "$BATCH_ID" ]; then
            echo -e "${RED}No active batch found${NC}"
            exit 1
        fi
    fi
    
    echo -e "Batch ID: ${GREEN}$BATCH_ID${NC}"
    echo ""
    
    # Get progress information
    PROGRESS_DATA=$(run_query "
        SELECT 
            total_count,
            processed_count,
            success_count,
            failure_count,
            ROUND(processed_count::numeric / NULLIF(total_count, 0) * 100, 2) as progress_pct,
            EXTRACT(EPOCH FROM (NOW() - started_at))/3600 as hours_elapsed,
            CASE 
                WHEN processed_count > 0 THEN 
                    ROUND((total_count - processed_count)::numeric / 
                          (processed_count / NULLIF(EXTRACT(EPOCH FROM (NOW() - started_at)), 0)) / 3600, 2)
                ELSE 0
            END as hours_remaining
        FROM resubscription_checkpoints
        WHERE batch_id = '$BATCH_ID'
    ")
    
    # Parse the data
    IFS='|' read -r TOTAL PROCESSED SUCCESS FAILED PROGRESS ELAPSED REMAINING <<< "$PROGRESS_DATA"
    
    # Trim whitespace
    TOTAL=$(echo "$TOTAL" | tr -d ' ')
    PROCESSED=$(echo "$PROCESSED" | tr -d ' ')
    SUCCESS=$(echo "$SUCCESS" | tr -d ' ')
    FAILED=$(echo "$FAILED" | tr -d ' ')
    PROGRESS=$(echo "$PROGRESS" | tr -d ' ')
    ELAPSED=$(echo "$ELAPSED" | tr -d ' ')
    REMAINING=$(echo "$REMAINING" | tr -d ' ')
    
    # Calculate rates
    if [ "$PROCESSED" -gt 0 ]; then
        ERROR_RATE=$(echo "scale=2; $FAILED * 100 / $PROCESSED" | bc)
        RATE_PER_SEC=$(echo "scale=2; $PROCESSED / ($ELAPSED * 3600)" | bc)
    else
        ERROR_RATE=0
        RATE_PER_SEC=0
    fi
    
    # Display statistics
    echo "📊 STATISTICS"
    echo "----------------------------------------"
    printf "Total Records:     %'d\n" "$TOTAL"
    printf "Processed:         %'d (%.2f%%)\n" "$PROCESSED" "$PROGRESS"
    printf "Successful:        %'d\n" "$SUCCESS"
    printf "Failed:            %'d (%.2f%%)\n" "$FAILED" "$ERROR_RATE"
    echo ""
    
    echo "⏱️  PERFORMANCE"
    echo "----------------------------------------"
    printf "Processing Rate:   %.2f records/sec\n" "$RATE_PER_SEC"
    printf "Time Elapsed:      %.2f hours\n" "$ELAPSED"
    printf "Time Remaining:    %.2f hours\n" "$REMAINING"
    echo ""
    
    echo "📈 PROGRESS"
    echo "----------------------------------------"
    show_progress_bar "${PROGRESS%.*}"
    echo ""
    
    # Check for errors
    if [ "$ERROR_RATE" != "0" ] && (( $(echo "$ERROR_RATE > 5" | bc -l) )); then
        echo -e "${YELLOW}⚠️  WARNING: Error rate is above 5%${NC}"
        
        # Get top errors
        echo ""
        echo "Top Error Messages:"
        run_query "
            SELECT error_message, COUNT(*) as count
            FROM resubscription_tracking
            WHERE process_batch_id = '$BATCH_ID'
            AND resubscribe_status = 'failed'
            AND error_message IS NOT NULL
            GROUP BY error_message
            ORDER BY count DESC
            LIMIT 5
        " | while IFS='|' read -r ERROR COUNT; do
            printf "  - %s (%d occurrences)\n" "$ERROR" "$COUNT"
        done
    fi
}

# Function to show real-time logs
show_logs() {
    echo ""
    echo "📝 RECENT ACTIVITY"
    echo "----------------------------------------"
    
    run_query "
        SELECT 
            TO_CHAR(created_at, 'HH24:MI:SS') as time,
            msisdn,
            product_id,
            resubscribe_status,
            CASE 
                WHEN error_message IS NOT NULL THEN 
                    SUBSTRING(error_message, 1, 50)
                ELSE ''
            END as error
        FROM resubscription_tracking
        WHERE process_batch_id = '$BATCH_ID'
        ORDER BY created_at DESC
        LIMIT 10
    " | while IFS='|' read -r TIME MSISDN PRODUCT STATUS ERROR; do
        if [ "$STATUS" = "success" ]; then
            echo -e "$TIME | $MSISDN | Product: $PRODUCT | ${GREEN}✓ SUCCESS${NC}"
        else
            echo -e "$TIME | $MSISDN | Product: $PRODUCT | ${RED}✗ FAILED${NC} | $ERROR"
        fi
    done
}

# Function to check system health
check_system_health() {
    echo ""
    echo "💻 SYSTEM HEALTH"
    echo "----------------------------------------"
    
    # Database connections
    DB_CONNECTIONS=$(run_query "SELECT COUNT(*) FROM pg_stat_activity WHERE datname = '$DB_NAME'" | tr -d ' ')
    MAX_CONNECTIONS=$(run_query "SHOW max_connections" | tr -d ' ')
    CONNECTION_PCT=$(echo "scale=2; $DB_CONNECTIONS * 100 / $MAX_CONNECTIONS" | bc)
    
    printf "DB Connections:    %d/%d (%.2f%%)\n" "$DB_CONNECTIONS" "$MAX_CONNECTIONS" "$CONNECTION_PCT"
    
    # Check process status
    IS_RUNNING=$(run_query "SELECT CASE WHEN status = 'in_progress' THEN 'YES' ELSE 'NO' END FROM resubscription_checkpoints WHERE batch_id = '$BATCH_ID'" | tr -d ' ')
    
    if [ "$IS_RUNNING" = "YES" ]; then
        echo -e "Process Status:    ${GREEN}RUNNING${NC}"
    else
        echo -e "Process Status:    ${RED}STOPPED${NC}"
    fi
    
    # Get last checkpoint time
    LAST_CHECKPOINT=$(run_query "SELECT EXTRACT(EPOCH FROM (NOW() - updated_at))/60 FROM resubscription_checkpoints WHERE batch_id = '$BATCH_ID'" | tr -d ' ')
    printf "Last Checkpoint:   %.0f minutes ago\n" "$LAST_CHECKPOINT"
    
    if (( $(echo "$LAST_CHECKPOINT > 10" | bc -l) )); then
        echo -e "${YELLOW}⚠️  WARNING: No checkpoint in last 10 minutes${NC}"
    fi
}

# Function to generate summary report
generate_report() {
    echo ""
    echo "📋 GENERATING SUMMARY REPORT..."
    echo "----------------------------------------"
    
    REPORT_FILE="resubscription_report_${BATCH_ID}_$(date +%Y%m%d_%H%M%S).txt"
    
    {
        echo "RESUBSCRIPTION BATCH REPORT"
        echo "=========================="
        echo "Batch ID: $BATCH_ID"
        echo "Generated: $(date)"
        echo ""
        
        echo "SUMMARY STATISTICS"
        echo "------------------"
        run_query "
            SELECT 
                'Total Records: ' || total_count,
                'Processed: ' || processed_count || ' (' || ROUND(processed_count::numeric / NULLIF(total_count, 0) * 100, 2) || '%)',
                'Successful: ' || success_count,
                'Failed: ' || failure_count,
                'Success Rate: ' || ROUND(success_count::numeric / NULLIF(processed_count, 0) * 100, 2) || '%',
                'Started: ' || TO_CHAR(started_at, 'YYYY-MM-DD HH24:MI:SS'),
                'Updated: ' || TO_CHAR(updated_at, 'YYYY-MM-DD HH24:MI:SS'),
                'Duration: ' || ROUND(EXTRACT(EPOCH FROM (COALESCE(completed_at, NOW()) - started_at))/3600, 2) || ' hours'
            FROM resubscription_checkpoints
            WHERE batch_id = '$BATCH_ID'
        "
        
        echo ""
        echo "ERROR ANALYSIS"
        echo "--------------"
        run_query "
            SELECT error_message, COUNT(*) as count
            FROM resubscription_tracking
            WHERE process_batch_id = '$BATCH_ID'
            AND resubscribe_status = 'failed'
            AND error_message IS NOT NULL
            GROUP BY error_message
            ORDER BY count DESC
        "
        
        echo ""
        echo "PRODUCT BREAKDOWN"
        echo "-----------------"
        run_query "
            SELECT 
                rt.product_id,
                p.name,
                COUNT(*) as total,
                SUM(CASE WHEN rt.resubscribe_status = 'success' THEN 1 ELSE 0 END) as success,
                SUM(CASE WHEN rt.resubscribe_status = 'failed' THEN 1 ELSE 0 END) as failed
            FROM resubscription_tracking rt
            LEFT JOIN products p ON rt.product_id::text = p.product_id
            WHERE rt.process_batch_id = '$BATCH_ID'
            GROUP BY rt.product_id, p.name
            ORDER BY total DESC
        "
    } > "$REPORT_FILE"
    
    echo -e "${GREEN}Report saved to: $REPORT_FILE${NC}"
}

# Main menu
show_menu() {
    echo ""
    echo "========================================="
    echo "         MONITORING OPTIONS"
    echo "========================================="
    echo "1) View current progress"
    echo "2) Show recent activity logs"
    echo "3) Generate summary report"
    echo "4) Check system health"
    echo "5) Auto-refresh (every 10 seconds)"
    echo "6) Exit"
    echo ""
    read -p "Select option: " choice
    
    case $choice in
        1)
            monitor_batch
            ;;
        2)
            monitor_batch
            show_logs
            ;;
        3)
            generate_report
            ;;
        4)
            monitor_batch
            check_system_health
            ;;
        5)
            while true; do
                clear
                monitor_batch
                show_logs
                check_system_health
                echo ""
                echo "Auto-refreshing in 10 seconds... (Press Ctrl+C to stop)"
                sleep 10
            done
            ;;
        6)
            exit 0
            ;;
        *)
            echo "Invalid option"
            ;;
    esac
}

# Main execution
if [ "$1" = "--auto" ]; then
    # Auto-refresh mode
    while true; do
        clear
        monitor_batch
        show_logs
        check_system_health
        echo ""
        echo "Auto-refreshing in 10 seconds... (Press Ctrl+C to stop)"
        sleep 10
    done
elif [ "$1" = "--report" ]; then
    # Generate report only
    generate_report
elif [ "$1" = "--health" ]; then
    # System health check only
    monitor_batch
    check_system_health
else
    # Interactive mode
    while true; do
        show_menu
        echo ""
        read -p "Press Enter to continue..."
    done
fi
