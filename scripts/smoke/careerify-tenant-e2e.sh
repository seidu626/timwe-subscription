#!/usr/bin/env bash
# careerify-tenant-e2e.sh
# Smoke test: end-to-end happy-path matrix for the careerify/web-gh-airteltigo tenant.
# Exercises all 10 inbound URLs (6 notification + 4 subscription) through nginx -> KrakenD
# -> backend, confirming tenant scoping end-to-end.
#
# Usage:
#   ./scripts/smoke/careerify-tenant-e2e.sh
#   HOST=http://staging.example.com PARTNER_ROLE=2117 ./scripts/smoke/careerify-tenant-e2e.sh
#
# Returns exit 0 only if all 10 endpoints respond with 2xx.
# Slice TMP-070 — targets commits: TMP-066=77f9359 TMP-067=7e10692 TMP-068=3027c86 TMP-069=3897e89

set -euo pipefail

HOST="${HOST:-http://127.0.0.1:8080}"
TENANT_KEY="${TENANT_KEY:-careerify}"
CHANNEL_KEY="${CHANNEL_KEY:-web-gh-airteltigo}"
PARTNER_ROLE="${PARTNER_ROLE:-airtelgh}"
MSISDN="${MSISDN:-233572503330}"
EXTERNAL_TX_ID="${EXTERNAL_TX_ID:-smoke-$(date +%s)}"

QUERY="tenant_key=${TENANT_KEY}&channel_key=${CHANNEL_KEY}"
BASE_SUB="${HOST}/api/external/v1/${TENANT_KEY}/${CHANNEL_KEY}/subscriptions"

# ---------------------------------------------------------------------------
# Request bodies
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# Endpoint list: label|url|body
# 6 notification endpoints + 4 subscription endpoints = 10 total
# ---------------------------------------------------------------------------
declare -a TESTS=(
  "notification/mo|${HOST}/api/v1/notification/mo/${PARTNER_ROLE}?${QUERY}|${MO_BODY}"
  "notification/mt-dn|${HOST}/api/v1/notification/mt/dn/${PARTNER_ROLE}?${QUERY}|${MT_DN_BODY}"
  "notification/user-optin|${HOST}/api/v1/notification/user-optin/${PARTNER_ROLE}?${QUERY}|${USER_OPTIN_BODY}"
  "notification/user-renewed|${HOST}/api/v1/notification/user-renewed/${PARTNER_ROLE}?${QUERY}|${USER_RENEWED_BODY}"
  "notification/user-optout|${HOST}/api/v1/notification/user-optout/${PARTNER_ROLE}?${QUERY}|${USER_OPTOUT_BODY}"
  "notification/charge|${HOST}/api/v1/notification/charge/${PARTNER_ROLE}?${QUERY}|${CHARGE_BODY}"
  "subscription/optin|${BASE_SUB}/optin|${OPTIN_BODY}"
  "subscription/confirm|${BASE_SUB}/confirm|${CONFIRM_BODY}"
  "subscription/optout|${BASE_SUB}/optout|${OPTOUT_BODY}"
  "subscription/status|${BASE_SUB}/status|${STATUS_BODY}"
)

PASS=0
FAIL=0
FAIL_LIST=()

echo "=== Careerify tenant e2e smoke test (10 happy-path URLs) ==="
echo "  Gateway    : ${HOST}"
echo "  Tenant     : ${TENANT_KEY} / ${CHANNEL_KEY}"
echo "  Partner    : ${PARTNER_ROLE}"
echo "  MSISDN     : ${MSISDN}"
echo "  Tx-ID      : ${EXTERNAL_TX_ID}"
echo ""
echo "  Commits    : TMP-066=77f9359 TMP-067=7e10692 TMP-068=3027c86 TMP-069=3897e89"
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
echo "=== Results: ${PASS}/10 PASS  ${FAIL}/10 FAIL ==="
echo ""
printf '  %-32s %s\n' "ENDPOINT" "RESULT"
printf '  %-32s %s\n' "--------" "------"
for entry in "${TESTS[@]}"; do
  IFS='|' read -r label url _body <<< "${entry}"
  if printf '%s\n' "${FAIL_LIST[@]+"${FAIL_LIST[@]}"}" | grep -qx "${label}"; then
    printf '  %-32s FAIL\n' "${label}"
  else
    printf '  %-32s PASS\n' "${label}"
  fi
done

echo ""
if [[ ${FAIL} -eq 0 ]]; then
  echo "All 10 careerify tenant endpoints returned 2xx. Tenant scoping end-to-end VERIFIED."
  exit 0
else
  echo "FAILED endpoints: ${FAIL_LIST[*]}"
  echo ""
  echo "Gap ownership:"
  echo "  notification/* failures -> TMP-066 (seed) or TMP-067 (KrakenD propagation)"
  echo "  subscription/*  failures -> TMP-066 (seed) or TMP-068 (subscription tenant path)"
  echo "  Any tenant-context error  -> TMP-069 (header/query precedence resolver)"
  exit 1
fi
