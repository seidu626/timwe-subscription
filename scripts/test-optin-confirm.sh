#!/usr/bin/env bash
set -euo pipefail

# End-to-end opt-in → confirm test via acquisition-api
#
# Usage:
#   ./scripts/test-optin-confirm.sh                          # defaults
#   ./scripts/test-optin-confirm.sh --msisdn 233572503330    # custom MSISDN
#   ./scripts/test-optin-confirm.sh --host http://localhost:8084

HOST="${HOST:-http://139.59.135.253:8084}"
CAMPAIGN="${CAMPAIGN:-gh-airteltigo-mobplus-daily-v1}"
MSISDN="${MSISDN:-233572503330}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)    HOST="$2";     shift 2 ;;
    --campaign) CAMPAIGN="$2"; shift 2 ;;
    --msisdn)  MSISDN="$2";   shift 2 ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

echo "=== Opt-in / Confirm E2E Test ==="
echo "  Host:     $HOST"
echo "  Campaign: $CAMPAIGN"
echo "  MSISDN:   $MSISDN"
echo ""

# ── Step 1: Create transaction (triggers TIMWE opt-in + SMS with PIN) ──
echo ">>> Step 1: Creating transaction (opt-in)..."
OPTIN_RESP=$(curl -sS -w "\n%{http_code}" "$HOST/v1/acquisition/transactions" \
  -H "Content-Type: application/json" \
  --data-raw "{
    \"campaign_slug\": \"$CAMPAIGN\",
    \"msisdn\": \"$MSISDN\",
    \"consent_checked\": true
  }")

HTTP_CODE=$(echo "$OPTIN_RESP" | tail -1)
BODY=$(echo "$OPTIN_RESP" | sed '$d')

echo "  HTTP $HTTP_CODE"
echo "$BODY" | python3 -m json.tool 2>/dev/null || echo "$BODY"
echo ""

if [[ "$HTTP_CODE" != "200" ]]; then
  echo "ERROR: Opt-in failed (HTTP $HTTP_CODE). Fix the issue above before continuing."
  exit 1
fi

TX_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['transaction_id'])" 2>/dev/null)
STATUS=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null)

echo "  Transaction ID: $TX_ID"
echo "  Status:         $STATUS"
echo ""

STATUS_LOWER=$(echo "$STATUS" | tr '[:upper:]' '[:lower:]')
if [[ "$STATUS_LOWER" != "confirm_required" ]]; then
  echo "NOTE: Status is '$STATUS' (not CONFIRM_REQUIRED). Confirm step may not apply."
  echo "  If status is 'subscribed', the subscription completed without OTP."
  exit 0
fi

# ── Step 2: Wait for PIN from SMS ──
echo ">>> Step 2: Waiting for PIN..."
echo "  An SMS with a confirmation code should arrive at $MSISDN."
echo ""
read -rp "  Enter the PIN from SMS (or 'skip' to just check status): " PIN
echo ""

if [[ "$PIN" == "skip" ]]; then
  echo ">>> Checking transaction status..."
  STATUS_RESP=$(curl -sS "$HOST/v1/acquisition/transactions/$TX_ID/status")
  echo "$STATUS_RESP" | python3 -m json.tool 2>/dev/null || echo "$STATUS_RESP"
  exit 0
fi

# ── Step 3: Confirm transaction with PIN ──
CONFIRM_URL="$HOST/v1/acquisition/transactions/$TX_ID/confirm"
CONFIRM_BODY="{\"auth_code\": \"$PIN\"}"

echo ">>> Step 3: Confirming transaction with PIN=$PIN..."
echo "  URL:  POST $CONFIRM_URL"
echo "  Body: $CONFIRM_BODY"
echo ""

CONFIRM_RESP=$(curl -sS -w "\n%{http_code}" "$CONFIRM_URL" \
  -H "Content-Type: application/json" \
  --data-raw "$CONFIRM_BODY")

HTTP_CODE=$(echo "$CONFIRM_RESP" | tail -1)
BODY=$(echo "$CONFIRM_RESP" | sed '$d')

echo "  HTTP $HTTP_CODE"
echo "$BODY" | python3 -m json.tool 2>/dev/null || echo "$BODY"
echo ""

if [[ "$HTTP_CODE" == "200" ]]; then
  echo "SUCCESS: Transaction confirmed."
else
  echo "FAILED: Confirm returned HTTP $HTTP_CODE."
fi

# ── Step 4: Final status check ──
echo ""
echo ">>> Step 4: Final status check..."
STATUS_RESP=$(curl -sS "$HOST/v1/acquisition/transactions/$TX_ID/status")
echo "$STATUS_RESP" | python3 -m json.tool 2>/dev/null || echo "$STATUS_RESP"
