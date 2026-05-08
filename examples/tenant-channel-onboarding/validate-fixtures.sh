#!/usr/bin/env bash
set -euo pipefail

fixture_path="${1:-examples/tenant-channel-onboarding/contract-fixtures.json}"

jq -e '
  .contract_version == "tenant-channel-v1.0.0"
  and (.tenant_key | length > 0)
  and (.channel_key | length > 0)
  and (.partner_key | length > 0)
  and ([.fixtures[] | select(.expect == "accepted")] | length >= 3)
  and ([.fixtures[] | select(.expect == "rejected" and .error_code == "SIGNATURE_REQUIRED")] | length == 1)
  and ([.fixtures[] | select(.expect == "rejected" and .error_code == "CAPABILITY_NOT_ENABLED")] | length == 1)
  and ([.fixtures[] | select(.kind == "postback" and .body.postback_id and .body.tenant_key and .body.channel_key)] | length == 1)
  and all(.fixtures[]; .body.tenant_key and .body.channel_key and .body.partner_key)
  and all(.fixtures[] | select(.kind == "request" and .expect == "accepted"); .headers["Idempotency-Key"])
  and all(.fixtures[] | select(.kind == "callback" and .expect == "accepted"); .headers["X-Timwe-Signature"] and .headers["X-Timwe-Timestamp"] and .headers["X-Timwe-Event-Id"])
' "$fixture_path" >/dev/null

echo "tenant-channel contract fixtures: PASS"
