#!/bin/bash

# Test script for the renewal system
# This script tests the renewal API endpoints

set -e

# Configuration
BASE_URL="http://localhost:8083"
API_BASE="/api/v1/renewal"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    if [ "$status" = "SUCCESS" ]; then
        echo -e "${GREEN}[SUCCESS]${NC} $message"
    elif [ "$status" = "FAILED" ]; then
        echo -e "${RED}[FAILED]${NC} $message"
    else
        echo -e "${YELLOW}[INFO]${NC} $message"
    fi
}

# Function to test an endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local expected_status=$4
    
    local url="$BASE_URL$endpoint"
    local response
    local status_code
    
    print_status "INFO" "Testing $method $endpoint"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "%{http_code}" "$url" -o /tmp/response.json)
    elif [ "$method" = "POST" ]; then
        if [ -n "$data" ]; then
            response=$(curl -s -w "%{http_code}" -X POST -H "Content-Type: application/json" -d "$data" "$url" -o /tmp/response.json)
        else
            response=$(curl -s -w "%{http_code}" -X POST "$url" -o /tmp/response.json)
        fi
    fi
    
    # Extract status code (last line of response)
    status_code=$(echo "$response" | tail -n1)
    
    if [ "$status_code" = "$expected_status" ]; then
        print_status "SUCCESS" "$method $endpoint returned $status_code"
        if [ -f /tmp/response.json ]; then
            echo "Response: $(cat /tmp/response.json)"
        fi
    else
        print_status "FAILED" "$method $endpoint returned $status_code, expected $expected_status"
        if [ -f /tmp/response.json ]; then
            echo "Response: $(cat /tmp/response.json)"
        fi
    fi
    
    echo ""
}

# Check if service is running
print_status "INFO" "Checking if service is running..."
if ! curl -s "$BASE_URL/health" > /dev/null; then
    print_status "FAILED" "Service is not running at $BASE_URL"
    print_status "INFO" "Please start the service first"
    exit 1
fi

print_status "SUCCESS" "Service is running at $BASE_URL"
echo ""

# Test renewal endpoints
print_status "INFO" "Testing renewal system endpoints..."
echo ""

# Test worker status
test_endpoint "GET" "$API_BASE/worker/status" "" "200"

# Test renewal statistics
test_endpoint "GET" "$API_BASE/statistics" "" "200"

# Test churn candidates
test_endpoint "GET" "$API_BASE/churn-candidates" "" "200"

# Test renewal health
test_endpoint "GET" "$API_BASE/health" "" "200"

# Test manual renewal (this should fail without proper data)
test_endpoint "POST" "$API_BASE/manual" '{"msisdn":"1234567890","product_id":"test_product"}' "500"

# Test priority retry queue processing
test_endpoint "POST" "$API_BASE/priority-retry/process" "" "200"

# Test force churn evaluation
test_endpoint "POST" "$API_BASE/force-churn-evaluation" "" "200"

# Test worker start (this might fail if already running)
test_endpoint "POST" "$API_BASE/worker/start" "" "200"

# Test worker stop (this might fail if not running)
test_endpoint "POST" "$API_BASE/worker/stop" "" "200"

print_status "INFO" "Renewal system test completed!"
echo ""

# Cleanup
rm -f /tmp/response.json

print_status "SUCCESS" "All tests completed. Check the output above for any failures." 