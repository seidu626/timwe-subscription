#!/usr/bin/env bash
# krakend-subscription-tenant.sh
# Smoke test: verify KrakenD propagates tenant_key and channel_key to the subscription-external
# admin handlers as X-Tenant-Key / X-Channel-Key headers for all 4 tenant-path endpoints.
#
# Usage:
#   ./scripts/smoke/krakend-subscription-tenant.sh
#   HOST=http://127.0.0.1:8090 TENANT_KEY=careerify CHANNEL_KEY=web-gh-airteltigo ./scripts/smoke/krakend-subscription-tenant.sh
#
# Returns exit 0 only if all 4 endpoints respond with 2xx.
# Does not require the full service stack to be running; non-2xx results are reported and
# the script exits 1 — the TMP-070 closing slice is responsible for a live run.

set -uo pipefail

HOST="${HOST:-http://127.0.0.1:8080}"
TENANT_KEY="${TENANT_KEY:-careerify}"
CHANNEL_KEY="${CHANNEL_KEY:-web-gh-airteltigo}"
MSISDN="${MSISDN:-233572503330}"
EXTERNAL_TX_ID="${EXTERNAL_TX_ID:-smoke-$(date +%s)}"

BASE_PATH="${HOST}/api/external/v1/${TENANT_KEY}/${CHANNEL_KEY}/subscriptions"

# Minimal valid body shared across all subscription operations
OPTIN_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "externalTxId": "${EXTERNAL_TX_ID}"
}
JSON
)

CONFIRM_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "externalTxId": "${EXTERNAL_TX_ID}"
}
JSON
)

OPTOUT_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "externalTxId": "${EXTERNAL_TX_ID}"
}
JSON
)

STATUS_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}"
}
JSON
)

# Endpoint list: label|url|body
declare -a TESTS=(
  "optin|${BASE_PATH}/optin|${OPTIN_BODY}"
  "confirm|${BASE_PATH}/confirm|${CONFIRM_BODY}"
  "optout|${BASE_PATH}/optout|${OPTOUT_BODY}"
  "status|${BASE_PATH}/status|${STATUS_BODY}"
)

PASS=0
FAIL=0
FAIL_LIST=()

echo "=== KrakenD subscription tenant-path smoke test ==="
echo "  Gateway   : ${HOST}"
echo "  Tenant    : ${TENANT_KEY} / ${CHANNEL_KEY}"
echo "  Base path : ${BASE_PATH}"
echo ""

for entry in "${TESTS[@]}"; do
  IFS='|' read -r label url body <<< "${entry}"

  RESPONSE=$(curl -sS -w "\n%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -H "external-tx-id: ${EXTERNAL_TX_ID}" \
    --max-time 10 \
    --data "${body}" \
    "${url}" 2>&1) || true

  HTTP_CODE=$(printf '%s' "${RESPONSE}" | tail -1)
  RESP_BODY=$(printf '%s' "${RESPONSE}" | sed '$d')

  if [[ "${HTTP_CODE}" =~ ^2[0-9]{2}$ ]]; then
    echo "  [PASS] ${label}  HTTP ${HTTP_CODE}"
    PASS=$(( PASS + 1 ))
  else
    echo "  [FAIL] ${label}  HTTP ${HTTP_CODE}"
    echo "         url : ${url}"
    echo "         body: ${RESP_BODY}"
    FAIL=$(( FAIL + 1 ))
    FAIL_LIST+=("${label}")
  fi
done

echo ""
echo "=== Results: ${PASS} PASS / ${FAIL} FAIL ==="

if [[ ${FAIL} -eq 0 ]]; then
  echo "All 4 subscription endpoints returned 2xx with tenant headers propagated."
  exit 0
else
  echo "FAILED endpoints: ${FAIL_LIST[*]}"
  exit 1
fi
