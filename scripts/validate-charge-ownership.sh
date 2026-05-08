#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$repo_root/docker-compose.yml"
adr_file="$repo_root/slices/decisions/TMP-017-charge-ownership.md"
billing_repo="$repo_root/services/billing/internal/repository/postgres.go"
krakend_endpoint_template="$repo_root/krakend/config/templates/Endpoint.tmpl"
krakend_static_config="$repo_root/krakend/krakend.json"

if [[ ! -f "$adr_file" ]]; then
  echo "missing charge ownership ADR: $adr_file" >&2
  exit 1
fi

if ! grep -q "subscription-external.*tenant-platform owner" "$adr_file"; then
  echo "charge ownership ADR must name subscription-external as tenant-platform owner" >&2
  exit 1
fi

if grep -Eq '^[[:space:]]{2}billing:' "$compose_file"; then
  if ! grep -q "tenant_id" "$billing_repo" || ! grep -q "channel_id" "$billing_repo"; then
    echo "billing service is enabled but billing repository is not tenant/channel aware" >&2
    exit 1
  fi
fi

if ! grep -q "billing service disabled" "$compose_file"; then
  echo "docker-compose must document disabled billing service ownership posture" >&2
  exit 1
fi

if grep -q "billing_api_url" "$krakend_endpoint_template"; then
  echo "KrakenD endpoints must not route charge/MT traffic to disabled billing owner" >&2
  exit 1
fi

if grep -q '"url_pattern": "/api/v1/{realm}/charge/dob/{partnerRole}"' "$krakend_static_config"; then
  echo "static KrakenD config still routes to legacy billing charge endpoint" >&2
  exit 1
fi

if grep -q '"url_pattern": "/api/v1/{realm}/{channel}/mt/{partnerRole}"' "$krakend_static_config"; then
  echo "static KrakenD config still routes to legacy billing MT endpoint" >&2
  exit 1
fi

echo "charge ownership validation passed"
