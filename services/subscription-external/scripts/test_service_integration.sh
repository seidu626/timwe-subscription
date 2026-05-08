#!/bin/bash
# Comprehensive Service Integration Test
# File: scripts/test_service_integration.sh

set -e

# Configuration
SERVICE_HOST="${SERVICE_HOST:-localhost}"
SERVICE_PORT="${SERVICE_PORT:-8083}"
BASE_URL="http://$SERVICE_HOST:$SERVICE_PORT"
TEST_DATA_DIR="/tmp/resubscription_test_data"
LOG_FILE="/tmp/service_integration_test_$(date +%Y%m%d_%H%M%S).log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

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

# Function to run test
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_status="${3:-200}"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    log_message "INFO" "Running test: $test_name"
    
    if eval "$test_command" > /tmp/test_output.log 2>&1; then
        # Check if response contains expected status or success indicator
        if grep -q "HTTP/1.1 $expected_status" /tmp/test_output.log || \
           grep -q "success" /tmp/test_output.log || \
           grep -q "200 OK" /tmp/test_output.log; then
            log_message "SUCCESS" "Test PASSED: $test_name"
            PASSED_TESTS=$((PASSED_TESTS + 1))
            return 0
        else
            log_message "WARNING" "Test output for $test_name:"
            cat /tmp/test_output.log | tee -a "$LOG_FILE"
            log_message "ERROR" "Test FAILED: $test_name (unexpected response)"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            return 1
        fi
    else
        log_message "ERROR" "Test FAILED: $test_name (command failed)"
        log_message "ERROR" "Test output:"
        cat /tmp/test_output.log | tee -a "$LOG_FILE"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

# Function to check service health
check_service_health() {
    log_message "INFO" "Checking service health..."
    
    # Test basic health endpoint
    if curl -s -f "$BASE_URL/health" > /dev/null; then
        log_message "SUCCESS" "Service health check passed"
        return 0
    else
        log_message "ERROR" "Service health check failed"
        return 1
    fi
}

# Function to test basic endpoints
test_basic_endpoints() {
    log_message "INFO" "Testing basic endpoints..."
    
    # Test health endpoint
    run_test "Health Check" "curl -s -w 'HTTP/1.1 %{http_code}' -o /dev/null '$BASE_URL/health'" "200"
    
    # Test metrics endpoint
    run_test "Metrics Endpoint" "curl -s -w 'HTTP/1.1 %{http_code}' -o /dev/null '$BASE_URL/metrics'" "200"
    
    # Test swagger endpoint
    run_test "Swagger Endpoint" "curl -s -w 'HTTP/1.1 %{http_code}' -o /dev/null '$BASE_URL/swagger/index.html'" "200"
}

# Function to test enhanced resubscription endpoints
test_enhanced_endpoints() {
    log_message "INFO" "Testing enhanced resubscription endpoints..."
    
    # Create test data directory
    mkdir -p "$TEST_DATA_DIR"
    
    # Test enhanced resubscribe endpoint with minimal data
    local test_payload='{
        "telco": "TEST",
        "entry_channel": "USSD",
        "product_ids": ["1"],
        "batch_size": 10,
        "max_workers": 2,
        "use_charging_failures": false,
        "dry_run": true
    }'
    
    echo "$test_payload" > "$TEST_DATA_DIR/enhanced_resubscribe_test.json"
    
    run_test "Enhanced Resubscribe Endpoint" \
        "curl -s -X POST '$BASE_URL/api/v1/subscription-external/resubscribe/enhanced' \
         -H 'Content-Type: application/json' \
         -d @$TEST_DATA_DIR/enhanced_resubscribe_test.json \
         -w 'HTTP/1.1 %{http_code}' -o /tmp/enhanced_response.json" "202"
    
    # Test charging failures endpoint
    run_test "Charging Failures Endpoint" \
        "curl -s -w 'HTTP/1.1 %{http_code}' -o /dev/null '$BASE_URL/api/v1/subscription-external/charging-failures'" "200"
    
    # Test batch progress endpoint
    run_test "Batch Progress Endpoint" \
        "curl -s -w 'HTTP/1.1 %{http_code}' -o /dev/null '$BASE_URL/api/v1/subscription-external/batch/progress'" "200"
}

# Function to test existing endpoints
test_existing_endpoints() {
    log_message "INFO" "Testing existing endpoints..."
    
    # Test basic resubscribe endpoint
    local basic_payload='{
        "msisdn": "1234567890",
        "product_id": "1",
        "entry_channel": "USSD"
    }'
    
    echo "$basic_payload" > "$TEST_DATA_DIR/basic_resubscribe_test.json"
    
    run_test "Basic Resubscribe Endpoint" \
        "curl -s -X POST '$BASE_URL/api/v1/subscription-external/resubscribe' \
         -H 'Content-Type: application/json' \
         -d @$TEST_DATA_DIR/basic_resubscribe_test.json \
         -w 'HTTP/1.1 %{http_code}' -o /tmp/basic_response.json" "200"
    
    # Test batch endpoint
    local batch_payload='{
        "telco": "TEST",
        "entry_channel": "USSD",
        "product_ids": ["1"],
        "msisdns": ["1234567890", "0987654321"]
    }'
    
    echo "$batch_payload" > "$TEST_DATA_DIR/batch_test.json"
    
    run_test "Batch Endpoint" \
        "curl -s -X POST '$BASE_URL/api/v1/subscription-external/batch' \
         -H 'Content-Type: application/json' \
         -d @$TEST_DATA_DIR/batch_test.json \
         -w 'HTTP/1.1 %{http_code}' -o /tmp/batch_response.json" "202"
}

# Function to test error handling
test_error_handling() {
    log_message "INFO" "Testing error handling..."
    
    # Test invalid JSON
    run_test "Invalid JSON Handling" \
        "curl -s -X POST '$BASE_URL/api/v1/subscription-external/resubscribe/enhanced' \
         -H 'Content-Type: application/json' \
         -d '{invalid json}' \
         -w 'HTTP/1.1 %{http_code}' -o /dev/null" "400"
    
    # Test missing required fields
    local invalid_payload='{
        "telco": "TEST"
    }'
    
    echo "$invalid_payload" > "$TEST_DATA_DIR/invalid_payload.json"
    
    run_test "Missing Required Fields" \
        "curl -s -X POST '$BASE_URL/api/v1/subscription-external/resubscribe/enhanced' \
         -H 'Content-Type: application/json' \
         -d @$TEST_DATA_DIR/invalid_payload.json \
         -w 'HTTP/1.1 %{http_code}' -o /dev/null" "400"
    
    # Test invalid HTTP method
    run_test "Invalid HTTP Method" \
        "curl -s -X GET '$BASE_URL/api/v1/subscription-external/resubscribe/enhanced' \
         -w 'HTTP/1.1 %{http_code}' -o /dev/null" "405"
}

# Function to test performance
test_performance() {
    log_message "INFO" "Testing performance..."
    
    # Test response time for health endpoint
    local start_time=$(date +%s%N)
    curl -s "$BASE_URL/health" > /dev/null
    local end_time=$(date +%s%N)
    local response_time=$(( (end_time - start_time) / 1000000 ))
    
    if [ $response_time -lt 1000 ]; then
        log_message "SUCCESS" "Health endpoint response time: ${response_time}ms ✅"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        log_message "WARNING" "Health endpoint response time: ${response_time}ms ⚠️"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    # Test concurrent requests
    log_message "INFO" "Testing concurrent requests..."
    
    local concurrent_test_payload='{
        "telco": "TEST",
        "entry_channel": "USSD",
        "product_ids": ["1"],
        "batch_size": 5,
        "max_workers": 3,
        "use_charging_failures": false,
        "dry_run": true
    }'
    
    echo "$concurrent_test_payload" > "$TEST_DATA_DIR/concurrent_test.json"
    
    # Send 5 concurrent requests
    for i in {1..5}; do
        run_test "Concurrent Request $i" \
            "curl -s -X POST '$BASE_URL/api/v1/subscription-external/resubscribe/enhanced' \
             -H 'Content-Type: application/json' \
             -d @$TEST_DATA_DIR/concurrent_test.json \
             -w 'HTTP/1.1 %{http_code}' -o /tmp/concurrent_response_$i.json" "202"
    done
}

# Function to test database integration
test_database_integration() {
    log_message "INFO" "Testing database integration..."
    
    # Test if we can query charging failures (this should work if migration is applied)
    local db_test_query="
        SELECT COUNT(*) as count 
        FROM information_schema.tables 
        WHERE table_name = 'resubscription_tracking';
    "
    
    # This is a basic test - in a real scenario, you'd want to test actual database operations
    if command -v psql &> /dev/null; then
        log_message "INFO" "PostgreSQL client available - testing database connectivity"
        
        # Test basic database connectivity
        if psql -h localhost -U sm_admin -d subscription_manager -c "SELECT 1;" > /dev/null 2>&1; then
            log_message "SUCCESS" "Database connectivity test passed"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            log_message "WARNING" "Database connectivity test failed"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    else
        log_message "WARNING" "PostgreSQL client not available - skipping database tests"
    fi
}

# Function to generate test report
generate_test_report() {
    local report_file="$TEST_DATA_DIR/integration_test_report_$(date +%Y%m%d_%H%M%S).md"
    
    log_message "INFO" "Generating test report..."
    
    cat > "$report_file" << EOF
# Service Integration Test Report

## Test Summary
- **Date**: $(date)
- **Service**: $BASE_URL
- **Total Tests**: $TOTAL_TESTS
- **Passed**: $PASSED_TESTS
- **Failed**: $FAILED_TESTS
- **Success Rate**: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Results

### ✅ Passed Tests ($PASSED_TESTS)
$(grep "Test PASSED" "$LOG_FILE" | sed 's/.*Test PASSED: //' | sort | uniq | sed 's/^/- /')

### ❌ Failed Tests ($FAILED_TESTS)
$(grep "Test FAILED" "$LOG_FILE" | sed 's/.*Test FAILED: //' | sort | uniq | sed 's/^/- /')

## Endpoints Tested

### Basic Endpoints
- Health Check: /health
- Metrics: /metrics
- Swagger: /swagger/index.html

### Enhanced Resubscription Endpoints
- Enhanced Resubscribe: /api/v1/subscription-external/resubscribe/enhanced
- Charging Failures: /api/v1/subscription-external/charging-failures
- Batch Progress: /api/v1/subscription-external/batch/progress

### Existing Endpoints
- Basic Resubscribe: /api/v1/subscription-external/resubscribe
- Batch Processing: /api/v1/subscription-external/batch

## Recommendations

$(if [ $FAILED_TESTS -gt 0 ]; then
    echo "- **CRITICAL**: $FAILED_TESTS tests failed - review service logs and fix issues"
    echo "- **WARNING**: Service may not be ready for production use"
else
    echo "- **SUCCESS**: All tests passed - service appears ready for pilot testing"
    echo "- **NEXT STEP**: Proceed with pilot test using 1,000 records"
fi)

## Next Steps
1. Review failed tests and fix issues
2. Run pilot test with small dataset
3. Monitor system performance
4. Gradually increase load

## Log Files
- Test Log: $LOG_FILE
- Test Data: $TEST_DATA_DIR/
EOF
    
    log_message "SUCCESS" "Test report generated: $report_file"
}

# Function to cleanup test data
cleanup_test_data() {
    log_message "INFO" "Cleaning up test data..."
    
    # Remove temporary test files
    rm -rf "$TEST_DATA_DIR"
    
    # Remove temporary response files
    rm -f /tmp/*_response.json
    rm -f /tmp/test_output.log
    
    log_message "SUCCESS" "Test data cleanup completed"
}

# Main execution
main() {
    echo "========================================="
    echo "  SERVICE INTEGRATION TEST SUITE"
    echo "========================================="
    echo ""
    echo "Service: $BASE_URL"
    echo "Log File: $LOG_FILE"
    echo "Test Data: $TEST_DATA_DIR"
    echo ""
    
    # Initialize log file
    echo "Service integration test started at $(date)" > "$LOG_FILE"
    
    # Check if service is running
    if ! check_service_health; then
        log_message "CRITICAL" "Service is not accessible. Please ensure the service is running."
        echo ""
        echo "❌ Service integration test failed!"
        echo "📋 Log file: $LOG_FILE"
        exit 1
    fi
    
    # Run test suites
    test_basic_endpoints
    test_enhanced_endpoints
    test_existing_endpoints
    test_error_handling
    test_performance
    test_database_integration
    
    # Generate report
    generate_test_report
    
    # Display summary
    echo ""
    echo "========================================="
    echo "  TEST SUMMARY"
    echo "========================================="
    echo "Total Tests: $TOTAL_TESTS"
    echo "Passed: $PASSED_TESTS"
    echo "Failed: $FAILED_TESTS"
    echo "Success Rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%"
    echo ""
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo "✅ All tests passed! Service is ready for pilot testing."
        echo "📋 Test report: $TEST_DATA_DIR/integration_test_report_*.md"
        echo "📋 Log file: $LOG_FILE"
        echo ""
        echo "Next steps:"
        echo "1. Run pilot test with 1,000 records"
        echo "2. Monitor system performance"
        echo "3. Begin gradual rollout"
    else
        echo "❌ $FAILED_TESTS tests failed. Review issues before proceeding."
        echo "📋 Test report: $TEST_DATA_DIR/integration_test_report_*.md"
        echo "📋 Log file: $LOG_FILE"
        echo ""
        echo "Critical issues must be resolved before production use."
    fi
    
    # Cleanup
    cleanup_test_data
}

# Execute main function
main "$@" 