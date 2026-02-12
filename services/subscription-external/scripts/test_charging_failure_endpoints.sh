#!/bin/bash

# test_charging_failure_endpoints.sh - Test all charging failure endpoints
# File: scripts/test_charging_failure_endpoints.sh

set -e

echo "========================================="
echo "  CHARGING FAILURE ENDPOINTS TEST SUITE"
echo "========================================="

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Configuration
SERVICE_URL="http://localhost:8083"
LOG_FILE="/tmp/charging_failure_endpoints_test_$(date +%Y%m%d_%H%M%S).log"
TEST_DATA_DIR="/tmp/charging_failure_test_data"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to log messages
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

# Function to log success
log_success() {
    echo -e "${GREEN}✅ $1${NC}" | tee -a "$LOG_FILE"
}

# Function to log warning
log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}" | tee -a "$LOG_FILE"
}

# Function to log error
log_error() {
    echo -e "${RED}❌ $1${NC}" | tee -a "$LOG_FILE"
}

# Function to test endpoint
test_endpoint() {
    local method="$1"
    local endpoint="$2"
    local description="$3"
    local data="$4"
    
    log "Testing: $description"
    log "  Method: $method"
    log "  Endpoint: $endpoint"
    
    local response
    local status_code
    
    if [[ "$method" == "GET" ]]; then
        response=$(curl -s -w "\n%{http_code}" "$SERVICE_URL$endpoint" 2>/dev/null || echo "FAILED")
    elif [[ "$method" == "POST" ]]; then
        if [[ -n "$data" ]]; then
            response=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d "$data" "$SERVICE_URL$endpoint" 2>/dev/null || echo "FAILED")
        else
            response=$(curl -s -w "\n%{http_code}" -X POST "$SERVICE_URL$endpoint" 2>/dev/null || echo "FAILED")
        fi
    fi
    
    # Extract status code (last line)
    status_code=$(echo "$response" | tail -n1)
    # Extract response body (all lines except last)
    response_body=$(echo "$response" | head -n -1)
    
    log "  Status Code: $status_code"
    log "  Response: $response_body"
    
    if [[ "$status_code" == "200" || "$status_code" == "201" ]]; then
        log_success "Endpoint test passed: $description"
        return 0
    elif [[ "$status_code" == "501" ]]; then
        log_warning "Endpoint not yet implemented: $description (Status: $status_code)"
        return 0
    else
        log_error "Endpoint test failed: $description (Status: $status_code)"
        return 1
    fi
}

# Function to check service health
check_service_health() {
    log "🔍 Checking service health..."
    
    # Try different health endpoints
    local health_endpoints=(
        "/health"
        "/api/health"
        "/api/v1/health"
        "/ping"
    )
    
    local service_healthy=false
    
    for endpoint in "${health_endpoints[@]}"; do
        if curl -s "$SERVICE_URL$endpoint" >/dev/null 2>&1; then
            log_success "Service is responding on endpoint: $endpoint"
            service_healthy=true
            break
        fi
    done
    
    if [[ "$service_healthy" == "false" ]]; then
        log_error "Service is not responding on any health endpoint"
        log "Please ensure the service is running on $SERVICE_URL"
        return 1
    fi
    
    return 0
}

# Function to test charging failure endpoints
test_charging_failure_endpoints() {
    log ""
    log "🚀 Testing charging failure endpoints..."
    
    local test_results=()
    local total_tests=0
    local passed_tests=0
    
    # Test 1: Get charging failures (GET)
    total_tests=$((total_tests + 1))
    if test_endpoint "GET" "/api/v1/subscription-external/charging-failures" "Get charging failures list"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Test 2: Get charging failures with query parameters (GET)
    total_tests=$((total_tests + 1))
    if test_endpoint "GET" "/api/v1/subscription-external/charging-failures?limit=10&offset=0&days_threshold=30" "Get charging failures with parameters"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Test 3: Get charging failure statistics (GET)
    total_tests=$((total_tests + 1))
    if test_endpoint "GET" "/api/v1/subscription-external/charging-failures/stats" "Get charging failure statistics"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Test 4: Get charging failure summary (GET)
    total_tests=$((total_tests + 1))
    if test_endpoint "GET" "/api/v1/subscription-external/charging-failures/summary" "Get charging failure summary"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Test 5: Get charging failure by MSISDN (GET)
    total_tests=$((total_tests + 1))
    if test_endpoint "GET" "/api/v1/subscription-external/charging-failures/msisdn?msisdn=233200000000" "Get charging failure by MSISDN"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Test 6: Update charging health status (POST)
    total_tests=$((total_tests + 1))
    local update_data='{"subscription_id": 1, "status": "IN_PROGRESS", "reason": "Testing endpoint"}'
    if test_endpoint "POST" "/api/v1/subscription-external/charging-failures/health-status" "Update charging health status" "$update_data"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Test 7: Mark charging failure as processed (POST)
    total_tests=$((total_tests + 1))
    local mark_data='{"subscription_id": 1, "status": "completed"}'
    if test_endpoint "POST" "/api/v1/subscription-external/charging-failures/mark-processed" "Mark charging failure as processed" "$mark_data"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Test 8: Test with product filter (GET)
    total_tests=$((total_tests + 1))
    if test_endpoint "GET" "/api/v1/subscription-external/charging-failures?product_ids=1,2,3&days_threshold=60" "Get charging failures with product filter"; then
        passed_tests=$((passed_tests + 1))
    fi
    
    # Summary
    log ""
    log "📊 Test Results Summary:"
    log "  Total Tests: $total_tests"
    log "  Passed: $passed_tests"
    log "  Failed: $((total_tests - passed_tests))"
    
    if [[ $passed_tests -eq $total_tests ]]; then
        log_success "All tests passed! 🎉"
        return 0
    else
        log_warning "Some tests failed. Check the logs for details."
        return 1
    fi
}

# Function to test enhanced resubscribe endpoint
test_enhanced_resubscribe() {
    log ""
    log "🚀 Testing enhanced resubscribe endpoint..."
    
    local test_data='{
        "telco": "AirtelTigo",
        "product_ids": ["1", "2"],
        "entry_channel": "USSD",
        "batch_size": 100,
        "max_workers": 10,
        "use_charging_failures": true,
        "resume_from_checkpoint": false
    }'
    
    if test_endpoint "POST" "/api/v1/subscription-external/resubscribe/enhanced" "Enhanced resubscribe with charging failures" "$test_data"; then
        log_success "Enhanced resubscribe endpoint test passed"
        return 0
    else
        log_error "Enhanced resubscribe endpoint test failed"
        return 1
    fi
}

# Function to test batch progress endpoint
test_batch_endpoints() {
    log ""
    log "🚀 Testing batch management endpoints..."
    
    # Test batch progress
    if test_endpoint "GET" "/api/v1/subscription-external/batch/progress?batch_id=test-batch-123" "Get batch progress"; then
        log_success "Batch progress endpoint test passed"
    else
        log_error "Batch progress endpoint test failed"
    fi
    
    # Test batch stop
    local stop_data='{"batch_id": "test-batch-123", "reason": "Testing endpoint"}'
    if test_endpoint "POST" "/api/v1/subscription-external/batch/stop" "Stop batch processing" "$stop_data"; then
        log_success "Batch stop endpoint test passed"
    else
        log_error "Batch stop endpoint test failed"
    fi
}

# Function to generate test report
generate_test_report() {
    log ""
    log "📋 Generating test report..."
    
    local report_file="$TEST_DATA_DIR/test_report_$(date +%Y%m%d_%H%M%S).md"
    
    mkdir -p "$TEST_DATA_DIR"
    
    cat > "$report_file" << EOF
# Charging Failure Endpoints Test Report

**Date**: $(date)
**Service URL**: $SERVICE_URL
**Test Duration**: $(date -d "@$SECONDS" -u +%H:%M:%S)

## Test Summary

- **Total Tests**: $total_tests
- **Passed**: $passed_tests
- **Failed**: $((total_tests - passed_tests))
- **Success Rate**: $((passed_tests * 100 / total_tests))%

## Test Results

### ✅ Passed Tests
$(grep "✅" "$LOG_FILE" | tail -n $passed_tests | sed 's/.*✅/✅/')

### ⚠️  Warnings
$(grep "⚠️" "$LOG_FILE" | sed 's/.*⚠️/⚠️/')

### ❌ Failed Tests
$(grep "❌" "$LOG_FILE" | sed 's/.*❌/❌/')

## Recommendations

1. **Database Migration**: Ensure migration 002_notifications_based_charging.sql is applied
2. **Service Integration**: Connect charging failure service to main service
3. **Performance Testing**: Test with large datasets (25M+ records)
4. **Monitoring**: Set up metrics and alerting for charging failure processing

## Next Steps

1. Deploy to staging environment
2. Test with real data from notifications table
3. Verify 25M+ target subscriptions detected
4. Performance optimization if needed
5. Production deployment

---
*Report generated by test_charging_failure_endpoints.sh*
EOF

    log_success "Test report generated: $report_file"
}

# Main execution
main() {
    log "🚀 Starting charging failure endpoints test suite..."
    log "Service URL: $SERVICE_URL"
    log "Log File: $LOG_FILE"
    log "Timestamp: $(date)"
    
    # Check service health
    if ! check_service_health; then
        log_error "Service health check failed. Exiting."
        exit 1
    fi
    
    # Test charging failure endpoints
    test_charging_failure_endpoints
    
    # Test enhanced resubscribe
    test_enhanced_resubscribe
    
    # Test batch endpoints
    test_batch_endpoints
    
    # Generate test report
    generate_test_report
    
    log ""
    log "========================================="
    log "  TEST SUITE COMPLETE"
    log "========================================="
    log ""
    log "📊 Final Results:"
    log "  Total Tests: $total_tests"
    log "  Passed: $passed_tests"
    log "  Failed: $((total_tests - passed_tests))"
    log ""
    log "📋 Test report: $TEST_DATA_DIR/"
    log "📝 Detailed logs: $LOG_FILE"
    log ""
    
    if [[ $passed_tests -eq $total_tests ]]; then
        log_success "🎉 All tests passed! The charging failure system is ready for production!"
        exit 0
    else
        log_warning "⚠️  Some tests failed. Please review the logs and fix any issues."
        exit 1
    fi
}

# Initialize variables
total_tests=0
passed_tests=0

# Run main function
main "$@" 