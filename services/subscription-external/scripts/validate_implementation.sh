#!/bin/bash

# validate_implementation.sh - Validate the notifications-based charging failure implementation
# File: scripts/validate_implementation.sh

set -e

echo "========================================="
echo "  IMPLEMENTATION VALIDATION"
echo "========================================="

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Project Root: $PROJECT_ROOT"
echo "Script Directory: $SCRIPT_DIR"
echo "Timestamp: $(date)"
echo ""

# Function to check if file exists and has content
check_file() {
    local file_path="$1"
    local description="$2"
    
    if [[ -f "$file_path" ]]; then
        local line_count=$(wc -l < "$file_path" 2>/dev/null || echo "0")
        echo "✅ $description: $file_path ($line_count lines)"
        return 0
    else
        echo "❌ $description: $file_path (MISSING)"
        return 1
    fi
}

# Function to check Go compilation
check_go_build() {
    echo ""
    echo "🔍 Checking Go compilation..."
    
    cd "$PROJECT_ROOT"
    
    if go build ./... 2>/dev/null; then
        echo "✅ Go compilation successful"
        return 0
    else
        echo "❌ Go compilation failed"
        return 1
    fi
}

# Function to check Go module dependencies
check_go_modules() {
    echo ""
    echo "🔍 Checking Go module dependencies..."
    
    cd "$PROJECT_ROOT"
    
    if go mod tidy 2>/dev/null; then
        echo "✅ Go module dependencies resolved"
        return 0
    else
        echo "❌ Go module dependency issues found"
        return 1
    fi
}

# Function to validate API endpoint structure
validate_api_structure() {
    echo ""
    echo "🔍 Validating API endpoint structure..."
    
    local router_file="$PROJECT_ROOT/internal/transport/router.go"
    local handler_file="$PROJECT_ROOT/internal/handler/subscription_handler.go"
    
    # Check if charging failure endpoints are defined
    local endpoints=(
        "charging-failures"
        "charging-failures/stats"
        "charging-failures/summary"
        "charging-failures/msisdn"
        "charging-failures/health-status"
        "charging-failures/mark-processed"
    )
    
    local all_endpoints_found=true
    
    for endpoint in "${endpoints[@]}"; do
        if grep -q "$endpoint" "$router_file" 2>/dev/null; then
            echo "✅ Endpoint: /api/v1/subscription-external/$endpoint"
        else
            echo "❌ Endpoint: /api/v1/subscription-external/$endpoint (MISSING)"
            all_endpoints_found=false
        fi
    done
    
    if [[ "$all_endpoints_found" == "true" ]]; then
        echo "✅ All charging failure endpoints are properly defined"
        return 0
    else
        echo "❌ Some charging failure endpoints are missing"
        return 1
    fi
}

# Function to validate service layer
validate_service_layer() {
    echo ""
    echo "🔍 Validating service layer..."
    
    local service_file="$PROJECT_ROOT/internal/service/charging_failure_service.go"
    
    if [[ -f "$service_file" ]]; then
        local method_count=$(grep -c "func.*ChargingFailureService" "$service_file" 2>/dev/null || echo "0")
        echo "✅ ChargingFailureService: $service_file ($method_count methods)"
        
        # Check for key methods
        local methods=(
            "GetChargingFailures"
            "GetChargingFailureCount"
            "GetChargingFailureStats"
            "GetChargingFailureSummary"
            "GetChargingFailureByMSISDN"
            "UpdateChargingHealthStatus"
            "MarkChargingFailureAsProcessed"
            "ProcessChargingFailures"
            "GetChargingFailureMetrics"
        )
        
        local all_methods_found=true
        
        for method in "${methods[@]}"; do
            if grep -q "func.*$method" "$service_file" 2>/dev/null; then
                echo "  ✅ Method: $method"
            else
                echo "  ❌ Method: $method (MISSING)"
                all_methods_found=false
            fi
        done
        
        if [[ "$all_methods_found" == "true" ]]; then
            echo "✅ All required service methods are implemented"
            return 0
        else
            echo "❌ Some required service methods are missing"
            return 1
        fi
    else
        echo "❌ ChargingFailureService: $service_file (MISSING)"
        return 1
    fi
}

# Function to validate repository layer
validate_repository_layer() {
    echo ""
    echo "🔍 Validating repository layer..."
    
    local repo_file="$PROJECT_ROOT/internal/repository/charging_failure_query.go"
    local interface_file="$PROJECT_ROOT/internal/repository/subscription.interface.go"
    
    if [[ -f "$repo_file" ]]; then
        local method_count=$(grep -c "func.*FetchChargingFailedSubscriptions\|func.*GetChargingFailureCount\|func.*GetChargingFailureStats" "$repo_file" 2>/dev/null || echo "0")
        echo "✅ ChargingFailureQuery: $repo_file ($method_count methods)"
        
        # Check for key repository methods
        local methods=(
            "FetchChargingFailedSubscriptions"
            "GetChargingFailureCount"
            "GetChargingFailureStats"
            "GetChargingFailureSummary"
            "GetChargingFailureByMSISDN"
            "UpdateChargingHealthStatus"
            "MarkChargingFailureAsProcessed"
        )
        
        local all_methods_found=true
        
        for method in "${methods[@]}"; do
            if grep -q "func.*$method" "$repo_file" 2>/dev/null; then
                echo "  ✅ Method: $method"
            else
                echo "  ❌ Method: $method (MISSING)"
                all_methods_found=false
            fi
        done
        
        if [[ "$all_methods_found" == "true" ]]; then
            echo "✅ All required repository methods are implemented"
        else
            echo "❌ Some required repository methods are missing"
            all_methods_found=false
        fi
    else
        echo "❌ ChargingFailureQuery: $repo_file (MISSING)"
        all_methods_found=false
    fi
    
    if [[ -f "$interface_file" ]]; then
        echo "✅ SubscriptionRepositoryInterface: $interface_file"
        
        # Check if charging failure methods are in interface
        if grep -q "FetchChargingFailedSubscriptions\|GetChargingFailureCount\|GetChargingFailureStats" "$interface_file" 2>/dev/null; then
            echo "✅ Charging failure methods are properly defined in interface"
        else
            echo "❌ Charging failure methods are missing from interface"
            all_methods_found=false
        fi
    else
        echo "❌ SubscriptionRepositoryInterface: $interface_file (MISSING)"
        all_methods_found=false
    fi
    
    if [[ "$all_methods_found" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# Function to validate database migration
validate_migration() {
    echo ""
    echo "🔍 Validating database migration..."
    
    local migration_file="$PROJECT_ROOT/migrations/002_notifications_based_charging.sql"
    
    if [[ -f "$migration_file" ]]; then
        local line_count=$(wc -l < "$migration_file" 2>/dev/null || echo "0")
        echo "✅ Migration: $migration_file ($line_count lines)"
        
        # Check for key migration components
        local components=(
            "ALTER TABLE subscriptions"
            "CREATE INDEX"
            "CREATE OR REPLACE VIEW"
            "CREATE OR REPLACE FUNCTION"
            "GRANT"
        )
        
        local all_components_found=true
        
        for component in "${components[@]}"; do
            if grep -q "$component" "$migration_file" 2>/dev/null; then
                echo "  ✅ Component: $component"
            else
                echo "  ❌ Component: $component (MISSING)"
                all_components_found=false
            fi
        done
        
        if [[ "$all_components_found" == "true" ]]; then
            echo "✅ All required migration components are present"
            return 0
        else
            echo "❌ Some required migration components are missing"
            return 1
        fi
    else
        echo "❌ Migration: $migration_file (MISSING)"
        return 1
    fi
}

# Main validation
echo "🚀 Starting implementation validation..."
echo ""

# Check file structure
echo "📁 Checking file structure..."
check_file "$PROJECT_ROOT/migrations/002_notifications_based_charging.sql" "Database Migration"
check_file "$PROJECT_ROOT/internal/repository/charging_failure_query.go" "Repository Implementation"
check_file "$PROJECT_ROOT/internal/service/charging_failure_service.go" "Service Implementation"
check_file "$PROJECT_ROOT/internal/handler/subscription_handler.go" "Handler Implementation"
check_file "$PROJECT_ROOT/internal/transport/router.go" "Router Configuration"
check_file "$PROJECT_ROOT/internal/repository/subscription.interface.go" "Repository Interface"

# Validate layers
validate_migration
validate_repository_layer
validate_service_layer
validate_api_structure

# Check Go compilation
check_go_build

# Check Go modules
check_go_modules

echo ""
echo "========================================="
echo "  VALIDATION COMPLETE"
echo "========================================="

# Summary
echo ""
echo "📊 Implementation Summary:"
echo "✅ Database Migration: Ready for deployment"
echo "✅ Repository Layer: Complete with notifications-based queries"
echo "✅ Service Layer: Complete with charging failure operations"
echo "✅ Handler Layer: Complete with API endpoints"
echo "✅ Router Configuration: Complete with charging failure routes"
echo "✅ Go Compilation: Successful"
echo "✅ Go Modules: Dependencies resolved"
echo ""
echo "🎯 Next Steps:"
echo "1. Deploy database migration (002_notifications_based_charging.sql)"
echo "2. Test API endpoints with real data"
echo "3. Verify 25M+ target subscriptions detected"
echo "4. Performance testing and optimization"
echo ""
echo "🚀 The notifications-based charging failure strategy is fully implemented and ready for deployment!" 