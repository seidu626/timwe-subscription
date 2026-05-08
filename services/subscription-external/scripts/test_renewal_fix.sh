#!/bin/bash

# Test script to verify renewal cycle ID fix
# This script tests the renewal service logic to ensure cycles get proper IDs

echo "Testing Renewal Cycle ID Fix"
echo "============================"

# Check if the service is running
echo "1. Checking if subscription-external service is running..."
if pgrep -f "subscription-external" > /dev/null; then
    echo "   ✓ Service is running"
else
    echo "   ✗ Service is not running"
    echo "   Please start the service first"
    exit 1
fi

# Check recent logs for the specific error
echo ""
echo "2. Checking recent logs for 'no renewal cycle found with id 0' errors..."
if journalctl -u subscription-external --since "1 hour ago" 2>/dev/null | grep -q "no renewal cycle found with id 0"; then
    echo "   ⚠️  Found recent errors - this indicates the issue still exists"
else
    echo "   ✓ No recent 'id 0' errors found"
fi

# Check for successful cycle creation logs
echo ""
echo "3. Checking for successful renewal cycle creation logs..."
if journalctl -u subscription-external --since "1 hour ago" 2>/dev/null | grep -q "Successfully created renewal cycle"; then
    echo "   ✓ Found successful cycle creation logs"
else
    echo "   ⚠️  No recent successful cycle creation logs found"
fi

# Check for cycle ID validation
echo ""
echo "4. Checking for cycle ID validation logs..."
if journalctl -u subscription-external --since "1 hour ago" 2>/dev/null | grep -q "Cycle has no ID, creating new cycle"; then
    echo "   ✓ Found cycle ID validation logs"
else
    echo "   ⚠️  No cycle ID validation logs found"
fi

# Check charging failure rate
echo ""
echo "5. Checking charging failure rate..."
if journalctl -u subscription-external --since "1 hour ago" 2>/dev/null | grep -q "Charging failure rate is 100.00%"; then
    echo "   ⚠️  Still seeing 100% charging failure rate"
else
    echo "   ✓ No recent 100% failure rate alerts"
fi

echo ""
echo "Test Summary:"
echo "============="
echo "The renewal cycle ID fix should resolve the 'no renewal cycle found with id 0' error."
echo "This should also help reduce the charging failure rate from 100%."
echo ""
echo "To monitor the fix:"
echo "1. Watch the logs for successful cycle creation"
echo "2. Monitor charging failure rates"
echo "3. Check for proper cycle ID assignment in logs"
echo ""
echo "If you still see issues, check:"
echo "- Database connectivity"
echo "- Renewal cycle table structure"
echo "- Service configuration" 