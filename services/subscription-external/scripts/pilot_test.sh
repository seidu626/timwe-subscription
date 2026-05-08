#!/bin/bash
# Pilot test script for resubscription process
# File: scripts/pilot_test.sh

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
SERVICE_URL="${SERVICE_URL:-http://localhost:8083}"
PILOT_SIZE="${1:-1000}"
BATCH_ID="pilot-$(date +%Y%m%d-%H%M%S)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================="
echo "    RESUBSCRIPTION PILOT TEST"
echo "========================================="
echo ""
echo "Batch ID: $BATCH_ID"
echo "Pilot Size: $PILOT_SIZE records"
echo "Service URL: $SERVICE_URL"
echo ""

# Function to run query
run_query() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "$1"
}

# Function to check service health
check_service() {
    if curl -f -s "$SERVICE_URL/health" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 1. Pre-flight checks
echo -e "${BLUE}1. Running pre-flight checks...${NC}"

# Check database
if pg_isready -h "$DB_HOST" -p "$DB_PORT" > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓${NC} Database connected"
else
    echo -e "  ${RED}✗${NC} Database not accessible"
    exit 1
fi

# Check service
if check_service; then
    echo -e "  ${GREEN}✓${NC} Service healthy"
else
    echo -e "  ${RED}✗${NC} Service not responding"
    exit 1
fi

# Check tables exist
TABLES_EXIST=$(run_query "
    SELECT COUNT(*) 
    FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name IN ('resubscription_tracking', 'resubscription_checkpoints')
" | tr -d ' ')

if [ "$TABLES_EXIST" -eq "2" ]; then
    echo -e "  ${GREEN}✓${NC} Required tables exist"
else
    echo -e "  ${RED}✗${NC} Required tables missing"
    echo "    Run migration first: ./scripts/apply_migration.sh"
    exit 1
fi

echo ""

# 2. Select pilot candidates
echo -e "${BLUE}2. Selecting pilot candidates...${NC}"

# Get charging failed subscriptions
CANDIDATES=$(run_query "
    WITH candidates AS (
        SELECT DISTINCT 
            s.id,
            s.user_identifier as msisdn,
            s.product_id,
            s.entry_channel
        FROM subscriptions s
        LEFT JOIN invalid_msisdn_logs iml ON 
            s.user_identifier = iml.msisdn 
            AND s.product_id = iml.product_id
        WHERE (
            iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE')
            OR iml.response_message LIKE '%charging%'
        )
        AND (s.resubscribe_status IS NULL OR s.resubscribe_status != 'completed')
        AND (s.last_resubscribe_attempt_at IS NULL 
             OR s.last_resubscribe_attempt_at < NOW() - INTERVAL '24 hours')
        ORDER BY s.id
        LIMIT $PILOT_SIZE
    )
    SELECT COUNT(*) FROM candidates
" | tr -d ' ')

echo -e "  Found ${GREEN}$CANDIDATES${NC} candidates for pilot"

if [ "$CANDIDATES" -eq "0" ]; then
    echo -e "  ${RED}✗${NC} No candidates found"
    echo "    Check if there are subscriptions with charging failures"
    exit 1
fi

echo ""

# 3. Export pilot data
echo -e "${BLUE}3. Exporting pilot data...${NC}"

PILOT_FILE="/tmp/pilot_${BATCH_ID}.json"

run_query "
    WITH pilot_data AS (
        SELECT 
            s.id,
            s.user_identifier as msisdn,
            s.product_id,
            s.entry_channel,
            s.status,
            s.charging_failure_count,
            s.last_charging_failure_at
        FROM subscriptions s
        LEFT JOIN invalid_msisdn_logs iml ON 
            s.user_identifier = iml.msisdn 
            AND s.product_id = iml.product_id
        WHERE (
            iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE')
            OR iml.response_message LIKE '%charging%'
        )
        AND (s.resubscribe_status IS NULL OR s.resubscribe_status != 'completed')
        ORDER BY s.id
        LIMIT $PILOT_SIZE
    )
    SELECT json_agg(pilot_data) FROM pilot_data
" > "$PILOT_FILE"

echo -e "  ${GREEN}✓${NC} Exported to $PILOT_FILE"

# Get unique product IDs
PRODUCT_IDS=$(run_query "
    SELECT string_agg(DISTINCT product_id::text, ',')
    FROM (
        SELECT s.product_id
        FROM subscriptions s
        LEFT JOIN invalid_msisdn_logs iml ON 
            s.user_identifier = iml.msisdn 
            AND s.product_id = iml.product_id
        WHERE (
            iml.response_code IN ('CHARGING_FAILED', 'INSUFFICIENT_BALANCE')
            OR iml.response_message LIKE '%charging%'
        )
        AND (s.resubscribe_status IS NULL OR s.resubscribe_status != 'completed')
        ORDER BY s.id
        LIMIT $PILOT_SIZE
    ) t
" | tr -d ' ')

echo -e "  Product IDs: ${GREEN}$PRODUCT_IDS${NC}"

echo ""

# 4. Create pilot request
echo -e "${BLUE}4. Creating pilot request...${NC}"

REQUEST_FILE="/tmp/pilot_request_${BATCH_ID}.json"

cat > "$REQUEST_FILE" << EOF
{
    "batch_id": "$BATCH_ID",
    "telco": "AirtelTigo",
    "entry_channels": ["USSD", "SMS", "WEB"],
    "product_ids": [$(echo $PRODUCT_IDS | sed 's/,/","/g' | sed 's/^/"/;s/$/"/') ],
    "use_charging_failures": true,
    "batch_size": $PILOT_SIZE,
    "max_workers": 10,
    "rate_limit_per_second": 10,
    "checkpoint_interval": 100,
    "force_reprocess": false,
    "dry_run": false
}
EOF

echo -e "  ${GREEN}✓${NC} Request created"
cat "$REQUEST_FILE" | jq '.'

echo ""

# 5. Confirm execution
echo -e "${YELLOW}⚠️  CONFIRMATION REQUIRED${NC}"
echo ""
echo "This will process $PILOT_SIZE subscriptions in PRODUCTION"
echo "Batch ID: $BATCH_ID"
echo ""
read -p "Do you want to proceed? (yes/NO): " -r
echo ""

if [[ ! "$REPLY" == "yes" ]]; then
    echo "Pilot test cancelled"
    exit 0
fi

# 6. Execute pilot
echo -e "${BLUE}5. Executing pilot test...${NC}"

RESPONSE=$(curl -X POST \
    -H "Content-Type: application/json" \
    -d @"$REQUEST_FILE" \
    "$SERVICE_URL/api/v1/subscription-external/resubscribe/enhanced" \
    2>/dev/null)

if [ $? -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} Request submitted"
    echo "  Response: $RESPONSE"
    
    JOB_ID=$(echo "$RESPONSE" | jq -r '.jobId')
    echo "  Job ID: $JOB_ID"
else
    echo -e "  ${RED}✗${NC} Request failed"
    exit 1
fi

echo ""

# 7. Monitor progress
echo -e "${BLUE}6. Monitoring progress...${NC}"
echo "  Starting monitor in 5 seconds..."
sleep 5

# Launch monitor in background
./scripts/monitor_resubscription.sh "$BATCH_ID" &
MONITOR_PID=$!

echo "  Monitor PID: $MONITOR_PID"
echo ""
echo "Press Ctrl+C to stop monitoring (pilot will continue running)"

# Wait for monitor or user interrupt
wait $MONITOR_PID

echo ""
echo -e "${GREEN}Pilot test initiated successfully${NC}"
echo ""
echo "Next steps:"
echo "1. Monitor progress: ./scripts/monitor_resubscription.sh $BATCH_ID"
echo "2. Generate report: ./scripts/monitor_resubscription.sh $BATCH_ID --report"
echo "3. Check errors in database"
echo "4. Analyze results before proceeding with full batch"
