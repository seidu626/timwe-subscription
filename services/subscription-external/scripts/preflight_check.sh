#!/bin/bash
# Pre-flight checks before starting resubscription process
# File: scripts/preflight_check.sh

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
SERVICE_URL="${SERVICE_URL:-http://localhost:8083}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================="
echo "    PRE-FLIGHT CHECKS FOR RESUBSCRIPTION"
echo "========================================="
echo ""

CHECKS_PASSED=true

# Function to check status
check_status() {
    local check_name=$1
    local status=$2
    local message=$3
    
    if [ "$status" = "PASS" ]; then
        echo -e "✅ ${GREEN}[PASS]${NC} $check_name"
        if [ -n "$message" ]; then
            echo "         $message"
        fi
    elif [ "$status" = "WARN" ]; then
        echo -e "⚠️  ${YELLOW}[WARN]${NC} $check_name"
        echo "         $message"
    else
        echo -e "❌ ${RED}[FAIL]${NC} $check_name"
        echo "         $message"
        CHECKS_PASSED=false
    fi
}

# 1. Check database connectivity
echo -e "${BLUE}1. DATABASE CONNECTIVITY${NC}"
echo "----------------------------------------"
if pg_isready -h "$DB_HOST" -p "$DB_PORT" > /dev/null 2>&1; then
    check_status "PostgreSQL Connection" "PASS" "Connected to $DB_HOST:$DB_PORT"
else
    check_status "PostgreSQL Connection" "FAIL" "Cannot connect to $DB_HOST:$DB_PORT"
fi

# Check if we can query the database
if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" > /dev/null 2>&1; then
    check_status "Database Access" "PASS" "Can execute queries on $DB_NAME"
else
    check_status "Database Access" "FAIL" "Cannot execute queries on $DB_NAME"
fi

echo ""

# 2. Check required tables exist
echo -e "${BLUE}2. DATABASE SCHEMA${NC}"
echo "----------------------------------------"

REQUIRED_TABLES=("subscriptions" "products" "invalid_msisdn_logs" "resubscription_tracking" "resubscription_checkpoints")

for table in "${REQUIRED_TABLES[@]}"; do
    EXISTS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT EXISTS (
            SELECT 1 FROM information_schema.tables 
            WHERE table_schema = 'public' 
            AND table_name = '$table'
        )" | tr -d ' ')
    
    if [ "$EXISTS" = "t" ]; then
        check_status "Table: $table" "PASS"
    else
        check_status "Table: $table" "FAIL" "Table does not exist"
    fi
done

echo ""

# 3. Check service health
echo -e "${BLUE}3. SERVICE HEALTH${NC}"
echo "----------------------------------------"

if curl -f -s "$SERVICE_URL/health" > /dev/null 2>&1; then
    check_status "Subscription Service" "PASS" "Service is healthy at $SERVICE_URL"
else
    check_status "Subscription Service" "FAIL" "Service not responding at $SERVICE_URL"
fi

echo ""

# 4. Check system resources
echo -e "${BLUE}4. SYSTEM RESOURCES${NC}"
echo "----------------------------------------"

# Check disk space
DISK_USAGE=$(df -h / | awk 'NR==2 {print $5}' | sed 's/%//')
DISK_AVAILABLE=$(df -BG / | awk 'NR==2 {print $4}' | sed 's/G//')

if [ "$DISK_USAGE" -lt 80 ]; then
    check_status "Disk Space" "PASS" "${DISK_AVAILABLE}GB available (${DISK_USAGE}% used)"
elif [ "$DISK_USAGE" -lt 90 ]; then
    check_status "Disk Space" "WARN" "${DISK_AVAILABLE}GB available (${DISK_USAGE}% used)"
else
    check_status "Disk Space" "FAIL" "Only ${DISK_AVAILABLE}GB available (${DISK_USAGE}% used)"
fi

# Check memory
MEM_AVAILABLE=$(free -g | awk 'NR==2 {print $7}')
MEM_TOTAL=$(free -g | awk 'NR==2 {print $2}')
MEM_PERCENT=$((100 - (MEM_AVAILABLE * 100 / MEM_TOTAL)))

if [ "$MEM_PERCENT" -lt 80 ]; then
    check_status "Memory" "PASS" "${MEM_AVAILABLE}GB available of ${MEM_TOTAL}GB"
elif [ "$MEM_PERCENT" -lt 90 ]; then
    check_status "Memory" "WARN" "${MEM_AVAILABLE}GB available of ${MEM_TOTAL}GB"
else
    check_status "Memory" "FAIL" "Only ${MEM_AVAILABLE}GB available of ${MEM_TOTAL}GB"
fi

echo ""

# 5. Check database statistics
echo -e "${BLUE}5. DATABASE STATISTICS${NC}"
echo "----------------------------------------"

# Get total subscriptions with charging issues
CHARGING_FAILED_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
    SELECT COUNT(DISTINCT s.id)
    FROM subscriptions s
    LEFT JOIN invalid_msisdn_logs iml ON s.user_identifier = iml.msisdn
    WHERE (
        iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE', 'CHARGING_ERROR')
        OR iml.response_message LIKE '%charging%'
    )" 2>/dev/null | tr -d ' ') || CHARGING_FAILED_COUNT=0

echo "Total subscriptions with charging issues: $CHARGING_FAILED_COUNT"

# Get already processed count
ALREADY_PROCESSED=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
    SELECT COUNT(DISTINCT msisdn)
    FROM resubscription_tracking
    WHERE resubscribe_status = 'success'
    AND created_at > NOW() - INTERVAL '7 days'
" 2>/dev/null | tr -d ' ') || ALREADY_PROCESSED=0

echo "Already processed (last 7 days): $ALREADY_PROCESSED"

# Get pending count
PENDING=$((CHARGING_FAILED_COUNT - ALREADY_PROCESSED))
if [ "$PENDING" -lt 0 ]; then
    PENDING=0
fi
echo "Pending to process: $PENDING"

echo ""

# 6. Check for active batches
echo -e "${BLUE}6. ACTIVE BATCHES${NC}"
echo "----------------------------------------"

ACTIVE_BATCHES=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
    SELECT COUNT(*)
    FROM resubscription_checkpoints
    WHERE status = 'in_progress'
" | tr -d ' ')

if [ "$ACTIVE_BATCHES" -eq 0 ]; then
    check_status "Active Batches" "PASS" "No active batches running"
else
    check_status "Active Batches" "WARN" "$ACTIVE_BATCHES active batch(es) found"
    
    # Show active batch details
    echo ""
    echo "  Active batch details:"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
        SELECT 
            batch_id,
            total_count,
            processed_count,
            ROUND(processed_count::numeric / total_count * 100, 2) as progress_pct,
            started_at
        FROM resubscription_checkpoints
        WHERE status = 'in_progress'
    "
fi

echo ""

# 7. Database connection pool
echo -e "${BLUE}7. DATABASE CONNECTIONS${NC}"
echo "----------------------------------------"

CURRENT_CONNECTIONS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
    SELECT COUNT(*) FROM pg_stat_activity WHERE datname = '$DB_NAME'
" | tr -d ' ')

MAX_CONNECTIONS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
    SHOW max_connections
" | tr -d ' ')

CONNECTION_PERCENT=$((CURRENT_CONNECTIONS * 100 / MAX_CONNECTIONS))

if [ "$CONNECTION_PERCENT" -lt 50 ]; then
    check_status "Database Connections" "PASS" "$CURRENT_CONNECTIONS/$MAX_CONNECTIONS connections used"
elif [ "$CONNECTION_PERCENT" -lt 80 ]; then
    check_status "Database Connections" "WARN" "$CURRENT_CONNECTIONS/$MAX_CONNECTIONS connections used"
else
    check_status "Database Connections" "FAIL" "$CURRENT_CONNECTIONS/$MAX_CONNECTIONS connections used"
fi

echo ""

# 8. Final summary
echo "========================================="
echo "              SUMMARY"
echo "========================================="

if [ "$CHECKS_PASSED" = true ]; then
    echo -e "${GREEN}✅ All critical checks passed!${NC}"
    echo ""
    echo "System is ready for resubscription processing."
    echo ""
    echo "Recommendations:"
    echo "1. Start with a small batch (1000-10000 records) for testing"
    echo "2. Monitor the process using: ./scripts/monitor_resubscription.sh"
    echo "3. Check logs regularly for any errors"
    echo "4. Have rollback procedure ready"
    exit 0
else
    echo -e "${RED}❌ Some critical checks failed!${NC}"
    echo ""
    echo "Please resolve the issues above before starting the resubscription process."
    exit 1
fi
