#!/bin/bash

# Performance Monitoring Script for MSISDN Generator
# Run this script to monitor system performance

echo "=== MSISDN Generator Performance Monitor ==="
echo "Timestamp: $(date)"
echo ""

# Check if the application is running
if pgrep -f "subscription-external" > /dev/null; then
    echo "✅ Application is running"
    
    # Get process info
    PID=$(pgrep -f "subscription-external")
    echo "Process ID: $PID"
    
    # Memory usage
    MEM_USAGE=$(ps -p $PID -o %mem --no-headers)
    echo "Memory usage: ${MEM_USAGE}%"
    
    # CPU usage
    CPU_USAGE=$(ps -p $PID -o %cpu --no-headers)
    echo "CPU usage: ${CPU_USAGE}%"
    
    # Check health endpoint if available
    if command -v curl >/dev/null 2>&1; then
        echo ""
        echo "=== Health Check ==="
        if curl -s http://localhost:8083/health >/dev/null 2>&1; then
            echo "✅ Health endpoint accessible"
            # Get detailed metrics
            curl -s http://localhost:8083/health | jq '.msisdn_generator' 2>/dev/null || echo "Raw response: $(curl -s http://localhost:8083/health)"
        else
            echo "⚠️  Health endpoint not accessible"
        fi
    fi
else
    echo "❌ Application is not running"
fi

echo ""
echo "=== System Resources ==="
echo "CPU load: $(uptime | awk -F'load average:' '{print $2}')"
echo "Memory usage: $(free -h | grep Mem | awk '{print $3"/"$2" ("$3/$2*100.0"%)"}')"
echo "Disk usage: $(df -h . | tail -1 | awk '{print $5" of "$2}')"
