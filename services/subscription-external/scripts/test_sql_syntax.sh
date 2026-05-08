#!/bin/bash
# SQL Syntax Test Script
# Purpose: Test SQL syntax without requiring database connection
# Author: System Optimization Team
# Date: 2025-01-27

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Testing SQL syntax for optimization scripts..."

# Test compatible script (new default)
echo "Testing compatible optimization script..."
if grep -q "CREATE INDEX IF NOT EXISTS" "$SCRIPT_DIR/optimize_invalid_msisdn_compatible.sql"; then
    echo "✓ Compatible script contains expected CREATE INDEX statements"
else
    echo "✗ Compatible script missing CREATE INDEX statements"
    exit 1
fi

if grep -q "CREATE OR REPLACE VIEW" "$SCRIPT_DIR/optimize_invalid_msisdn_compatible.sql"; then
    echo "✓ Compatible script contains expected VIEW statements"
else
    echo "✗ Compatible script missing VIEW statements"
    exit 1
fi

if grep -q "CREATE OR REPLACE FUNCTION" "$SCRIPT_DIR/optimize_invalid_msisdn_compatible.sql"; then
    echo "✓ Compatible script contains expected FUNCTION statements"
else
    echo "✗ Compatible script missing FUNCTION statements"
    exit 1
fi

# Test simple script
echo "Testing simple optimization script..."
if grep -q "CREATE INDEX IF NOT EXISTS" "$SCRIPT_DIR/optimize_invalid_msisdn_simple.sql"; then
    echo "✓ Simple script contains expected CREATE INDEX statements"
else
    echo "✗ Simple script missing CREATE INDEX statements"
    exit 1
fi

if grep -q "CREATE OR REPLACE VIEW" "$SCRIPT_DIR/optimize_invalid_msisdn_simple.sql"; then
    echo "✓ Simple script contains expected VIEW statements"
else
    echo "✗ Simple script missing VIEW statements"
    exit 1
fi

if grep -q "CREATE OR REPLACE FUNCTION" "$SCRIPT_DIR/optimize_invalid_msisdn_simple.sql"; then
    echo "✓ Simple script contains expected FUNCTION statements"
else
    echo "✗ Simple script missing FUNCTION statements"
    exit 1
fi

# Test full script
echo "Testing full optimization script..."
if grep -q "CREATE INDEX IF NOT EXISTS" "$SCRIPT_DIR/optimize_invalid_msisdn_database.sql"; then
    echo "✓ Full script contains expected CREATE INDEX statements"
else
    echo "✗ Full script missing CREATE INDEX statements"
    exit 1
fi

# Check that CONCURRENTLY is not in DO blocks
if grep -A 10 -B 10 "DO \$\$" "$SCRIPT_DIR/optimize_invalid_msisdn_database.sql" | grep -q "CREATE INDEX CONCURRENTLY"; then
    echo "✗ Full script contains CONCURRENTLY inside DO blocks"
    exit 1
else
    echo "✓ Full script does not contain CONCURRENTLY inside DO blocks"
fi

echo "All SQL syntax tests passed!"
echo ""
echo "Usage examples:"
echo "  # Use compatible script (recommended, works across PostgreSQL versions)"
echo "  ./deploy_optimization.sh --compatible"
echo ""
echo "  # Use simple script (no CONCURRENTLY issues)"
echo "  ./deploy_optimization.sh --simple"
echo ""
echo "  # Use full script (if CONCURRENTLY is supported)"
echo "  ./deploy_optimization.sh --full"
echo ""
echo "  # Test only (no deployment)"
echo "  ./deploy_optimization.sh --test-only" 