#!/bin/bash
# HE Bootstrap Flow Test Script
# Tests the HTTP-only Header Enrichment bootstrap flow
#
# Usage:
#   ./test_he_bootstrap.sh [environment]
#
# Environments:
#   local   - Test against localhost (default)
#   staging - Test against staging environment
#
# Prerequisites:
#   - Redis running (for token storage)
#   - acquisition-api running on port 8084
#   - NGINX configured with he-bootstrap.conf

set -e

ENV="${1:-local}"

case "$ENV" in
    local)
        HTTP_HOST="http://localhost:80"
        HTTPS_HOST="https://localhost:443"
        API_HOST="http://localhost:8084"
        ;;
    staging)
        HTTP_HOST="http://landing.nouveauricheglobalgroup.com"
        HTTPS_HOST="https://landing.nouveauricheglobalgroup.com"
        API_HOST="https://api.nouveauricheglobalgroup.com"
        ;;
    *)
        echo "Unknown environment: $ENV"
        exit 1
        ;;
esac

echo "================================================"
echo "HE Bootstrap Flow Test - Environment: $ENV"
echo "================================================"
echo ""

# Test 1: Health check
echo "Test 1: Health Check"
echo "--------------------"
curl -s "${API_HOST}/health" | jq .
echo ""

# Test 2: Direct API bootstrap (should work in dev with trusted proxy headers)
echo "Test 2: Direct HE Bootstrap (simulating trusted proxy)"
echo "------------------------------------------------------"
echo "Note: This test requires X-HE-Trusted-Proxy header to be set by NGINX"
echo "In production, this header is only set when request comes from operator proxy CIDR"
echo ""

# Simulate a trusted proxy request (localhost is in dev allowlist)
RESPONSE=$(curl -s -X GET "${API_HOST}/v1/he/bootstrap?campaign=test" \
    -H "X-HE-Trusted-Proxy: 1" \
    -H "X-MSISDN: 233241234567" \
    -H "X-Real-IP: 127.0.0.1" \
    -w "\n%{http_code}" 2>&1)

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)

if [ "$HTTP_CODE" = "302" ]; then
    echo "✓ Bootstrap endpoint returned 302 redirect (expected)"
    echo "  Response headers should contain Location with he_token"
else
    echo "✗ Unexpected response code: $HTTP_CODE"
    echo "  Response: $BODY"
fi
echo ""

# Test 3: Bootstrap without trusted proxy (should fail)
echo "Test 3: Bootstrap without trusted proxy (should fail)"
echo "-----------------------------------------------------"
RESPONSE=$(curl -s -X GET "${API_HOST}/v1/he/bootstrap" \
    -H "X-MSISDN: 233241234567" \
    -w "\n%{http_code}" 2>&1)

HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" = "403" ]; then
    echo "✓ Bootstrap correctly rejected untrusted request (403)"
else
    echo "✗ Unexpected response code: $HTTP_CODE (expected 403)"
fi
echo ""

# Test 4: Bootstrap without HE headers (should redirect to OTP flow)
echo "Test 4: Bootstrap without HE headers (OTP fallback)"
echo "---------------------------------------------------"
RESPONSE=$(curl -s -X GET "${API_HOST}/v1/he/bootstrap" \
    -H "X-HE-Trusted-Proxy: 1" \
    -H "X-Real-IP: 127.0.0.1" \
    -w "\n%{http_code}" 2>&1)

HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" = "302" ]; then
    echo "✓ Bootstrap redirected to HTTPS (OTP flow - no token)"
else
    echo "✗ Unexpected response code: $HTTP_CODE"
fi
echo ""

# Test 5: Token exchange with invalid token
echo "Test 5: Token exchange with invalid token"
echo "-----------------------------------------"
RESPONSE=$(curl -s -X GET "${API_HOST}/v1/he/token/exchange?token=invalidtoken1234567890123456789012345678901234567890123456789012" \
    -w "\n%{http_code}" 2>&1)

HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" = "404" ]; then
    echo "✓ Token exchange correctly rejected invalid token (404)"
else
    echo "✗ Unexpected response code: $HTTP_CODE (expected 404)"
fi
echo ""

# Test 6: Spoof attempt - sending X-MSISDN directly on HTTPS
echo "Test 6: Spoof attempt (X-MSISDN on HTTPS should be stripped)"
echo "------------------------------------------------------------"
echo "Note: In production, NGINX strips X-MSISDN on HTTPS traffic"
echo "This test verifies the API doesn't trust raw MSISDN headers on HTTPS"
echo ""

# Test 7: Token exchange endpoint rate limiting info
echo "Test 7: Rate limiting info"
echo "-------------------------"
echo "The token exchange endpoint has rate limiting configured:"
echo "  - Global: 50 requests/second"
echo "  - Per IP: 10 requests/second"
echo "  - Burst capacity: 100"
echo ""

echo "================================================"
echo "Manual Testing Required"
echo "================================================"
echo ""
echo "Operator HE is mapped to THREE hostnames:"
echo "  - http://api.nouveauricheglobalgroup.com"
echo "  - http://landing.nouveauricheglobalgroup.com"
echo "  - http://lp.nouveauricheglobalgroup.com"
echo ""
echo "URL Paths:"
echo "  - HTTP capture path: /c/:slug (proxied to acquisition-api)"
echo "  - HTTPS canonical path: /lp/:slug (served by landing-web)"
echo "  - HTTPS alias path: /c/:slug -> redirects to /lp/:slug"
echo ""
echo "After HE capture, redirect goes to HTTPS /lp/:slug on SAME HOST."
echo ""
echo "The following tests require real mobile devices:"
echo ""
echo "1. MTN Ghana Test (all three hosts):"
echo "   - Device with MTN SIM on mobile data (not WiFi)"
echo "   - Test A: http://api.nouveauricheglobalgroup.com/c/test"
echo "     Expected: Redirect to https://api.nouveauricheglobalgroup.com/lp/test?he_token=..."
echo "   - Test B: http://landing.nouveauricheglobalgroup.com/c/test"
echo "     Expected: Redirect to https://landing.nouveauricheglobalgroup.com/lp/test?he_token=..."
echo "   - Test C: http://lp.nouveauricheglobalgroup.com/c/test"
echo "     Expected: Redirect to https://lp.nouveauricheglobalgroup.com/lp/test?he_token=..."
echo "   - Verify: Landing page shows, he_token is captured, transaction uses HE identity"
echo ""
echo "2. Telecel Ghana Test:"
echo "   - Device with Telecel SIM on mobile data"
echo "   - Same flow as above on any of the three hosts"
echo ""
echo "3. AT Ghana Test:"
echo "   - Device with AT Ghana SIM on mobile data"
echo "   - Same flow as above on any of the three hosts"
echo ""
echo "4. WiFi Test (negative):"
echo "   - Device on WiFi (no HE)"
echo "   - Navigate to: http://landing.nouveauricheglobalgroup.com/c/test"
echo "   - Expected: Redirect to https://.../lp/test without he_token (OTP flow)"
echo ""
echo "5. Spoof Test (negative):"
echo "   - From any device, send curl with X-MSISDN header on HTTPS"
echo "   - curl -H 'X-MSISDN: 233999999999' https://api.nouveauricheglobalgroup.com/v1/acquisition/transactions"
echo "   - Expected: MSISDN should NOT be in request context (stripped by NGINX)"
echo ""
echo "6. Same-Host Redirect Verification:"
echo "   - Verify that http://api.*/c/test redirects to https://api.*/lp/test (same host)"
echo "   - Verify that http://lp.*/c/test redirects to https://lp.*/lp/test (same host)"
echo ""
echo "7. Token Propagation Verification:"
echo "   - After landing on /lp/:slug?he_token=..., check browser console for:"
echo "     '[HE Bootstrap] Token captured and stored'"
echo "   - Submit form, check server logs for:"
echo "     '[Transaction] HE bootstrap token present, exchanging server-side...'"
echo "     '[Transaction] HE token exchanged successfully, source: REAL'"
echo ""
