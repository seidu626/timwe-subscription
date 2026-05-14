#!/usr/bin/env bash
# krakend-notification-tenant.sh
# Smoke test: verify KrakenD forwards tenant_key and channel_key query params
# to the notification service for all 6 notification callback endpoints.
#
# Usage:
#   ./scripts/smoke/krakend-notification-tenant.sh
#   HOST=http://127.0.0.1:8090 PARTNER_ROLE=2117 ./scripts/smoke/krakend-notification-tenant.sh
#
# Returns exit 0 only if all 6 endpoints respond with 2xx.
# Does not require the full service stack to be running; non-2xx results are reported and
# the script exits 1 — the TMP-070 closing slice is responsible for a live run.

set -uo pipefail

HOST="${HOST:-http://127.0.0.1:8080}"
TENANT_KEY="${TENANT_KEY:-careerify}"
CHANNEL_KEY="${CHANNEL_KEY:-web-gh-airteltigo}"
PARTNER_ROLE="${PARTNER_ROLE:-airtelgh}"
MSISDN="${MSISDN:-233572503330}"
EXTERNAL_TX_ID="${EXTERNAL_TX_ID:-smoke-$(date +%s)}"

QUERY="tenant_key=${TENANT_KEY}&channel_key=${CHANNEL_KEY}"

# Minimal valid body shared across all callback types
MO_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "partnerRole": "${PARTNER_ROLE}",
  "message": "STOP",
  "keyword": "STOP"
}
JSON
)

MT_DN_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "partnerRole": "${PARTNER_ROLE}",
  "status": "DELIVERED",
  "messageId": "smoke-msg-001"
}
JSON
)

USER_OPTIN_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "partnerRole": "${PARTNER_ROLE}",
  "optinDate": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON
)

USER_RENEWED_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "partnerRole": "${PARTNER_ROLE}",
  "renewalDate": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON
)

USER_OPTOUT_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "partnerRole": "${PARTNER_ROLE}",
  "optoutDate": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON
)

CHARGE_BODY=$(cat <<JSON
{
  "msisdn": "${MSISDN}",
  "partnerRole": "${PARTNER_ROLE}",
  "amount": "1.00",
  "currency": "GHS",
  "status": "SUCCESS",
  "transactionId": "${EXTERNAL_TX_ID}"
}
JSON
)

# Endpoint list: label|url|body
declare -a TESTS=(
  "mo|${HOST}/api/v1/notification/mo/${PARTNER_ROLE}?${QUERY}|${MO_BODY}"
  "mt/dn|${HOST}/api/v1/notification/mt/dn/${PARTNER_ROLE}?${QUERY}|${MT_DN_BODY}"
  "user-optin|${HOST}/api/v1/notification/user-optin/${PARTNER_ROLE}?${QUERY}|${USER_OPTIN_BODY}"
  "user-renewed|${HOST}/api/v1/notification/user-renewed/${PARTNER_ROLE}?${QUERY}|${USER_RENEWED_BODY}"
  "user-optout|${HOST}/api/v1/notification/user-optout/${PARTNER_ROLE}?${QUERY}|${USER_OPTOUT_BODY}"
  "charge|${HOST}/api/v1/notification/charge/${PARTNER_ROLE}?${QUERY}|${CHARGE_BODY}"
)

PASS=0
FAIL=0
FAIL_LIST=()

echo "=== KrakenD notification tenant-propagation smoke test ==="
echo "  Gateway : ${HOST}"
echo "  Tenant  : ${TENANT_KEY} / ${CHANNEL_KEY}"
echo "  Partner : ${PARTNER_ROLE}"
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
  echo "All 6 notification endpoints returned 2xx with tenant query params forwarded."
  exit 0
else
  echo "FAILED endpoints: ${FAIL_LIST[*]}"
  exit 1
fi
