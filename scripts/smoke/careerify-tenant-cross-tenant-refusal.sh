#!/usr/bin/env bash
# careerify-tenant-cross-tenant-refusal.sh
# Adversarial smoke test: verifies cross-tenant injection attempts are rejected.
# PASS means the server REJECTED the request with the expected 4xx/409.
# A 2xx response is a FAIL — it indicates a tenant-scoping regression.
#
# Three cases:
#   A) Conflict: X-Tenant-Key=careerify header + ?tenant_key=other-tenant query -> 409
#   B) Foreign tenant: tenant_key=evil-tenant (unknown) on notification endpoint -> 4xx
#   C) Missing channel: tenant_key=careerify, NO channel_key on notification endpoint -> 4xx
#
# Usage:
#   ./scripts/smoke/careerify-tenant-cross-tenant-refusal.sh
#   HOST=http://staging.example.com ./scripts/smoke/careerify-tenant-cross-tenant-refusal.sh
#
# Returns exit 0 only if all 3 adversarial cases are correctly refused.
# Slice TMP-070 — targets commits: TMP-066=77f9359 TMP-067=7e10692 TMP-068=3027c86 TMP-069=3897e89

set -euo pipefail

HOST="${HOST:-http://127.0.0.1:8080}"
TENANT_KEY="${TENANT_KEY:-careerify}"
CHANNEL_KEY="${CHANNEL_KEY:-web-gh-airteltigo}"
PARTNER_ROLE="${PARTNER_ROLE:-2117}"
MSISDN="${MSISDN:-233572503330}"
EXTERNAL_TX_ID="${EXTERNAL_TX_ID:-smoke-adv-$(date +%s)}"

# Minimal notification body reused across adversarial cases
NOTIFY_BODY="{\"msisdn\":\"${MSISDN}\",\"partnerRole\":${PARTNER_ROLE},\"message\":\"probe\",\"keyword\":\"probe\"}"

# Minimal subscription body for Case A (conflict on subscription endpoint)
SUB_BODY="{\"msisdn\":\"${MSISDN}\",\"externalTxId\":\"${EXTERNAL_TX_ID}\"}"

PASS=0
FAIL=0

echo "=== Careerify cross-tenant refusal smoke test (3 adversarial cases) ==="
echo "  Gateway : ${HOST}"
echo "  Tenant  : ${TENANT_KEY} / ${CHANNEL_KEY}"
echo ""
echo "  PASS = server correctly REJECTED the request with the expected error status."
echo "  FAIL = server accepted (2xx) or returned an unexpected status — tenant-scoping gap."
echo ""
echo "  Commits : TMP-066=77f9359 TMP-067=7e10692 TMP-068=3027c86 TMP-069=3897e89"
echo ""

# ---------------------------------------------------------------------------
# Case A: Conflict — X-Tenant-Key header disagrees with ?tenant_key query param
# Precedence rule 3: header and query disagree -> 409 TENANT_KEY_CONFLICT
# Route: subscription endpoint so the header is inspected by the resolver
# ---------------------------------------------------------------------------
CASE_A_NAME="A) header/query conflict (tenant key mismatch)"
CASE_A_URL="${HOST}/api/external/v1/${TENANT_KEY}/${CHANNEL_KEY}/subscriptions/optin?tenant_key=other-tenant&channel_key=${CHANNEL_KEY}"
CASE_A_EXPECTED=409

RESPONSE_A=$(curl -sS -w "\n%{http_code}" \
  -X POST \
  -H "Content-Type: application/json" \
  -H "X-Tenant-Key: ${TENANT_KEY}" \
  -H "external-tx-id: ${EXTERNAL_TX_ID}" \
  --max-time 10 \
  --data "${SUB_BODY}" \
  "${CASE_A_URL}" 2>&1) || true

HTTP_A=$(printf '%s' "${RESPONSE_A}" | tail -1)
BODY_A=$(printf '%s' "${RESPONSE_A}" | sed '$d')

echo "  Case  : ${CASE_A_NAME}"
echo "  URL   : ${CASE_A_URL}"
echo "  Header: X-Tenant-Key: ${TENANT_KEY}"
echo "  Query : tenant_key=other-tenant (conflict)"
echo "  Expect: HTTP ${CASE_A_EXPECTED} (TENANT_CONTEXT_REQUIRED)"
echo "  Actual: HTTP ${HTTP_A}"

if [[ "${HTTP_A}" == "${CASE_A_EXPECTED}" ]]; then
  echo "  Result: [PASS] server refused with 409 as expected"
  PASS=$(( PASS + 1 ))
else
  echo "  Result: [FAIL] expected ${CASE_A_EXPECTED}, got ${HTTP_A}"
  echo "  Body  : ${BODY_A}"
  echo "  Owner : TMP-069 (header/query precedence resolver)"
  FAIL=$(( FAIL + 1 ))
fi
echo ""

# ---------------------------------------------------------------------------
# Case B: Foreign tenant — unknown tenant_key not present in the tenants table
# Expect: 4xx (400 or 404) — tenant resolution failure, no rows in tenants table
# ---------------------------------------------------------------------------
CASE_B_NAME="B) foreign tenant key (unknown tenant)"
CASE_B_URL="${HOST}/api/v1/notification/mo/${PARTNER_ROLE}?tenant_key=evil-tenant&channel_key=${CHANNEL_KEY}"
CASE_B_EXPECTED="4xx"

RESPONSE_B=$(curl -sS -w "\n%{http_code}" \
  -X POST \
  -H "Content-Type: application/json" \
  -H "external-tx-id: ${EXTERNAL_TX_ID}" \
  --max-time 10 \
  --data "${NOTIFY_BODY}" \
  "${CASE_B_URL}" 2>&1) || true

HTTP_B=$(printf '%s' "${RESPONSE_B}" | tail -1)
BODY_B=$(printf '%s' "${RESPONSE_B}" | sed '$d')

echo "  Case  : ${CASE_B_NAME}"
echo "  URL   : ${CASE_B_URL}"
echo "  Query : tenant_key=evil-tenant (not in tenants table)"
echo "  Expect: HTTP 4xx (tenant resolution failure)"
echo "  Actual: HTTP ${HTTP_B}"

if [[ "${HTTP_B}" =~ ^4[0-9]{2}$ ]]; then
  echo "  Result: [PASS] server refused with 4xx (${HTTP_B}) as expected"
  PASS=$(( PASS + 1 ))
else
  echo "  Result: [FAIL] expected 4xx, got ${HTTP_B}"
  echo "  Body  : ${BODY_B}"
  echo "  Owner : TMP-066 (careerify tenant seed / tenants table) or TMP-069 (resolver)"
  FAIL=$(( FAIL + 1 ))
fi
echo ""

# ---------------------------------------------------------------------------
# Case C: Missing channel — tenant_key=careerify but NO channel_key supplied
# Expect: 4xx (400) — tenant context required, channel missing
# ---------------------------------------------------------------------------
CASE_C_NAME="C) missing channel_key (tenant only, no channel)"
CASE_C_URL="${HOST}/api/v1/notification/mo/${PARTNER_ROLE}?tenant_key=${TENANT_KEY}"
CASE_C_EXPECTED="4xx"

RESPONSE_C=$(curl -sS -w "\n%{http_code}" \
  -X POST \
  -H "Content-Type: application/json" \
  -H "external-tx-id: ${EXTERNAL_TX_ID}" \
  --max-time 10 \
  --data "${NOTIFY_BODY}" \
  "${CASE_C_URL}" 2>&1) || true

HTTP_C=$(printf '%s' "${RESPONSE_C}" | tail -1)
BODY_C=$(printf '%s' "${RESPONSE_C}" | sed '$d')

echo "  Case  : ${CASE_C_NAME}"
echo "  URL   : ${CASE_C_URL}"
echo "  Query : tenant_key=${TENANT_KEY} only (channel_key absent)"
echo "  Expect: HTTP 4xx (TENANT_CHANNEL_REQUIRED)"
echo "  Actual: HTTP ${HTTP_C}"

if [[ "${HTTP_C}" =~ ^4[0-9]{2}$ ]]; then
  echo "  Result: [PASS] server refused with 4xx (${HTTP_C}) as expected"
  PASS=$(( PASS + 1 ))
else
  echo "  Result: [FAIL] expected 4xx, got ${HTTP_C}"
  echo "  Body  : ${BODY_C}"
  echo "  Owner : TMP-067 (KrakenD notification tenant propagation) or TMP-069 (resolver)"
  FAIL=$(( FAIL + 1 ))
fi
echo ""

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo "=== Results: ${PASS}/3 PASS  ${FAIL}/3 FAIL ==="
echo ""

if [[ ${FAIL} -eq 0 ]]; then
  echo "All 3 adversarial cross-tenant injection attempts were correctly refused."
  echo "Tenant scoping boundary VERIFIED."
  exit 0
else
  echo "FAILED cases indicate a tenant-scoping gap. Do NOT promote to production."
  echo "Review the Owner hints above and the value-gate-report for per-case gap ownership."
  exit 1
fi
