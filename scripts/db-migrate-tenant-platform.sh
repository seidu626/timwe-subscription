#!/usr/bin/env bash

set -euo pipefail

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
DB_PASSWORD="${DB_PASSWORD:-}"
BATCH_SIZE="${BATCH_SIZE:-500}"
LEGACY_TENANT_KEY="${LEGACY_TENANT_KEY:-legacy-default}"
LEGACY_TENANT_NAME="${LEGACY_TENANT_NAME:-Legacy Default Tenant}"
LEGACY_TENANT_COUNTRY="${LEGACY_TENANT_COUNTRY:-GH}"
LEGACY_TENANT_STATUS="${LEGACY_TENANT_STATUS:-ACTIVE}"
MODE="${1:-}"

if [[ -z "$MODE" ]]; then
  MODE="--dry-run"
fi

if ! [[ "$BATCH_SIZE" =~ ^[1-9][0-9]*$ ]]; then
  echo "ERROR: BATCH_SIZE must be a positive integer" >&2
  exit 1
fi

if [[ -n "$DB_PASSWORD" ]]; then
  export PGPASSWORD="$DB_PASSWORD"
fi

PSQL=(psql -X -v ON_ERROR_STOP=1 -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME")

BACKFILL_TABLES=(
  "campaigns"
  "acquisition_transactions"
  "postback_outbox"
  "products"
  "userbase"
  "userbase_import_jobs"
  "userbase_import_errors"
  "admin_activity_logs"
  "subscriptions"
  "notifications"
  "admin_subscription_action_logs"
  "product_message_series"
  "message_content_items"
  "subscription_message_state"
  "message_outbox"
)

channel_scoped_table() {
  case "$1" in
    campaigns|postback_outbox|subscriptions|notifications|admin_subscription_action_logs|product_message_series|message_content_items|subscription_message_state|message_outbox)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

eligible_predicate_for_table() {
  local table="$1"
  if channel_scoped_table "$table"; then
    echo "tenant_id IS NULL AND channel_id IS NULL"
  else
    echo "tenant_id IS NULL"
  fi
}

blocked_predicate_for_table() {
  local table="$1"
  if channel_scoped_table "$table"; then
    echo "tenant_id IS NULL AND channel_id IS NOT NULL"
  else
    echo ""
  fi
}

conflict_query_for_table() {
  local table="$1"
  case "$table" in
    campaigns)
      cat <<'SQL'
SELECT COUNT(*)
FROM (
  SELECT slug
  FROM campaigns
  WHERE tenant_id IS NULL AND channel_id IS NULL
  GROUP BY slug
  HAVING COUNT(*) > 1
) AS duplicate_groups
SQL
      ;;
    products)
      cat <<'SQL'
SELECT COUNT(*)
FROM (
  SELECT product_id
  FROM products
  WHERE tenant_id IS NULL
  GROUP BY product_id
  HAVING COUNT(*) > 1
) AS duplicate_groups
SQL
      ;;
    subscriptions)
      cat <<'SQL'
SELECT COUNT(*)
FROM (
  SELECT partner_role_id, user_identifier, product_id
  FROM subscriptions
  WHERE tenant_id IS NULL AND channel_id IS NULL
  GROUP BY partner_role_id, user_identifier, product_id
  HAVING COUNT(*) > 1
) AS duplicate_groups
SQL
      ;;
    product_message_series)
      cat <<'SQL'
SELECT COUNT(*)
FROM (
  SELECT partner_role_id, product_id, name
  FROM product_message_series
  WHERE tenant_id IS NULL AND channel_id IS NULL
  GROUP BY partner_role_id, product_id, name
  HAVING COUNT(*) > 1
) AS duplicate_groups
SQL
      ;;
    *)
      echo ""
      ;;
  esac
}

query_scalar() {
  "${PSQL[@]}" -qAt -c "$1" | tr -d '[:space:]'
}

query_text() {
  "${PSQL[@]}" -qAt -c "$1"
}

table_exists() {
  local table="$1"
  [[ "$(query_scalar "SELECT to_regclass('public.${table}') IS NOT NULL")" == "t" ]]
}

tenant_column_exists() {
  local table="$1"
  [[ "$(query_scalar "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '${table}' AND column_name = 'tenant_id')")" == "t" ]]
}

channel_column_exists() {
  local table="$1"
  [[ "$(query_scalar "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '${table}' AND column_name = 'channel_id')")" == "t" ]]
}

ensure_schema_prerequisites() {
  local missing=0
  for table in "${BACKFILL_TABLES[@]}"; do
    if ! table_exists "$table"; then
      echo "MISSING_TABLE ${table}"
      missing=1
      continue
    fi

    if ! tenant_column_exists "$table"; then
      echo "MISSING_COLUMN ${table}.tenant_id"
      missing=1
    fi

    if channel_scoped_table "$table" && ! channel_column_exists "$table"; then
      echo "MISSING_COLUMN ${table}.channel_id"
      missing=1
    fi
  done

  return "$missing"
}

get_default_tenant_id() {
  query_text "
SELECT id
FROM tenants
WHERE tenant_key = '${LEGACY_TENANT_KEY}'
LIMIT 1;
" | tail -n 1
}

upsert_default_tenant() {
  query_text "
INSERT INTO tenants (id, tenant_key, name, status, default_country, metadata_json)
VALUES (
  gen_random_uuid(),
  '${LEGACY_TENANT_KEY}',
  '${LEGACY_TENANT_NAME}',
  '${LEGACY_TENANT_STATUS}',
  '${LEGACY_TENANT_COUNTRY}',
  jsonb_build_object('migration', 'TMP-011', 'kind', 'legacy-default')
)
ON CONFLICT (tenant_key) DO UPDATE SET
  name = EXCLUDED.name,
  status = EXCLUDED.status,
  default_country = EXCLUDED.default_country,
  metadata_json = EXCLUDED.metadata_json,
  updated_at = NOW()
RETURNING id;
" | tail -n 1
}

count_rows() {
  local table="$1"
  local predicate="$2"
  query_scalar "SELECT COUNT(*) FROM ${table} WHERE ${predicate}"
}

count_default_rows() {
  local table="$1"
  if [[ -z "${DEFAULT_TENANT_ID:-}" ]]; then
    echo 0
    return 0
  fi
  query_scalar "SELECT COUNT(*) FROM ${table} WHERE tenant_id = '${DEFAULT_TENANT_ID}'::uuid"
}

report_table_state() {
  local table="$1"
  local eligible_predicate blocked_predicate eligible_rows blocked_rows default_rows conflict_groups batches status
  eligible_predicate="$(eligible_predicate_for_table "$table")"
  blocked_predicate="$(blocked_predicate_for_table "$table")"
  eligible_rows="$(count_rows "$table" "$eligible_predicate")"
  default_rows="$(count_default_rows "$table")"
  blocked_rows="0"
  if [[ -n "$blocked_predicate" ]]; then
    blocked_rows="$(count_rows "$table" "$blocked_predicate")"
  fi
  conflict_groups="0"
  local conflict_query
  conflict_query="$(conflict_query_for_table "$table")"
  if [[ -n "$conflict_query" ]]; then
    conflict_groups="$(query_scalar "$conflict_query")"
  fi
  batches=0
  if [[ "$eligible_rows" -gt 0 ]]; then
    batches=$(( (eligible_rows + BATCH_SIZE - 1) / BATCH_SIZE ))
  fi
  status="READY"
  if [[ "$blocked_rows" -gt 0 || "$conflict_groups" -gt 0 ]]; then
    status="BLOCKED"
  fi
  printf "%-28s %14s %14s %14s %14s %14s %s\n" "$table" "$eligible_rows" "$default_rows" "$blocked_rows" "$conflict_groups" "$batches" "$status"
}

print_summary_header() {
  printf "%-28s %14s %14s %14s %14s %14s %s\n" "table" "eligible_rows" "default_rows" "blocked_rows" "conflict_groups" "batches" "status"
  printf "%-28s %14s %14s %14s %14s %14s %s\n" "-----" "-------------" "------------" "------------" "---------------" "-------" "------"
}

run_dry_run() {
  local schema_ready=0
  if ! ensure_schema_prerequisites; then
    schema_ready=1
  fi
  DEFAULT_TENANT_ID="$(get_default_tenant_id)"
  echo "tenant-platform migration dry-run"
  echo "default_tenant_key=${LEGACY_TENANT_KEY}"
  if [[ -n "$DEFAULT_TENANT_ID" ]]; then
    echo "default_tenant_id=${DEFAULT_TENANT_ID}"
    echo "default_tenant_present=yes"
  else
    echo "default_tenant_id=missing"
    echo "default_tenant_present=no"
    schema_ready=1
  fi
  echo "batch_size=${BATCH_SIZE}"
  echo ""
  print_summary_header

  local any_blocked=0
  local total_eligible=0
  for table in "${BACKFILL_TABLES[@]}"; do
    local eligible_rows blocked_rows conflict_groups
    eligible_rows="$(count_rows "$table" "$(eligible_predicate_for_table "$table")")"
    blocked_rows="0"
    if channel_scoped_table "$table"; then
      blocked_rows="$(count_rows "$table" "$(blocked_predicate_for_table "$table")")"
    fi
    conflict_groups="0"
    local conflict_query
    conflict_query="$(conflict_query_for_table "$table")"
    if [[ -n "$conflict_query" ]]; then
      conflict_groups="$(query_scalar "$conflict_query")"
    fi
    total_eligible=$((total_eligible + eligible_rows))
    if [[ "$blocked_rows" -gt 0 || "$conflict_groups" -gt 0 ]]; then
      any_blocked=1
    fi
    report_table_state "$table"
  done

  echo ""
  echo "summary"
  echo "eligible_rows_total=${total_eligible}"
  if [[ "$schema_ready" -ne 0 || "$any_blocked" -ne 0 ]]; then
    echo "readiness=BLOCKED"
  else
    echo "readiness=READY_FOR_APPLY"
  fi
}

backfill_table() {
  local table="$1"
  local predicate="$2"
  local total=0
  while true; do
    local batch_sql batch_rows
    batch_sql="
WITH batch AS (
  SELECT ctid
  FROM ${table}
  WHERE ${predicate}
  ORDER BY ctid
  LIMIT ${BATCH_SIZE}
)
UPDATE ${table} AS target
SET tenant_id = '${DEFAULT_TENANT_ID}'::uuid
FROM batch
WHERE target.ctid = batch.ctid
RETURNING 1;
"
    batch_rows="$(query_text "$batch_sql" | sed '/^$/d' | wc -l | tr -d ' ')"
    if [[ "$batch_rows" -eq 0 ]]; then
      break
    fi
    total=$((total + batch_rows))
  done
  echo "$total"
}

restore_table() {
  local table="$1"
  local total=0
  while true; do
    local batch_sql batch_rows
    batch_sql="
WITH batch AS (
  SELECT ctid
  FROM ${table}
  WHERE tenant_id = '${DEFAULT_TENANT_ID}'::uuid
  ORDER BY ctid
  LIMIT ${BATCH_SIZE}
)
UPDATE ${table} AS target
SET tenant_id = NULL
FROM batch
WHERE target.ctid = batch.ctid
RETURNING 1;
"
    batch_rows="$(query_text "$batch_sql" | sed '/^$/d' | wc -l | tr -d ' ')"
    if [[ "$batch_rows" -eq 0 ]]; then
      break
    fi
    total=$((total + batch_rows))
  done
  echo "$total"
}

apply_migration() {
  if ! ensure_schema_prerequisites; then
    echo "tenant-platform migration aborted because schema prerequisites are missing" >&2
    exit 1
  fi
  DEFAULT_TENANT_ID="$(upsert_default_tenant)"

  local blocked_tables=0
  for table in "${BACKFILL_TABLES[@]}"; do
    local blocked_rows conflict_groups
    blocked_rows="0"
    if channel_scoped_table "$table"; then
      blocked_rows="$(count_rows "$table" "$(blocked_predicate_for_table "$table")")"
    fi
    conflict_groups="0"
    local conflict_query
    conflict_query="$(conflict_query_for_table "$table")"
    if [[ -n "$conflict_query" ]]; then
      conflict_groups="$(query_scalar "$conflict_query")"
    fi
    if [[ "$blocked_rows" -gt 0 || "$conflict_groups" -gt 0 ]]; then
      echo "BLOCKED ${table} blocked_rows=${blocked_rows} conflict_groups=${conflict_groups}" >&2
      blocked_tables=1
    fi
  done

  if [[ "$blocked_tables" -ne 0 ]]; then
    echo "tenant-platform migration aborted before any data changes" >&2
    exit 1
  fi

  echo "applying tenant-platform migration"
  echo "default_tenant_id=${DEFAULT_TENANT_ID}"

  for table in "${BACKFILL_TABLES[@]}"; do
    local predicate moved_rows
    predicate="$(eligible_predicate_for_table "$table")"
    moved_rows="$(backfill_table "$table" "$predicate")"
    printf "%-28s %14s\n" "$table" "$moved_rows"
  done

  echo ""
  echo "verification"
  local remaining=0
  for table in "${BACKFILL_TABLES[@]}"; do
    local eligible_rows
    eligible_rows="$(count_rows "$table" "$(eligible_predicate_for_table "$table")")"
    if [[ "$eligible_rows" -ne 0 ]]; then
      echo "REMAINING ${table} eligible_rows=${eligible_rows}" >&2
      remaining=1
    fi
  done

  if [[ "$remaining" -ne 0 ]]; then
    echo "tenant-platform migration left eligible legacy rows behind" >&2
    exit 1
  fi

  echo "readiness=APPLIED"
}

rollback_migration() {
  if ! ensure_schema_prerequisites; then
    echo "tenant-platform rollback aborted because schema prerequisites are missing" >&2
    exit 1
  fi
  DEFAULT_TENANT_ID="$(upsert_default_tenant)"

  echo "rolling back tenant-platform migration"
  echo "default_tenant_id=${DEFAULT_TENANT_ID}"

  for table in "${BACKFILL_TABLES[@]}"; do
    local restored_rows
    restored_rows="$(restore_table "$table")"
    printf "%-28s %14s\n" "$table" "$restored_rows"
  done

  local references=0
  for table in "${BACKFILL_TABLES[@]}"; do
    references=$((references + $(count_rows "$table" "tenant_id = '${DEFAULT_TENANT_ID}'::uuid")))
  done

  if [[ "$references" -eq 0 ]]; then
    query_text "DELETE FROM tenants WHERE tenant_key = '${LEGACY_TENANT_KEY}';" >/dev/null
  fi

  echo "readiness=ROLLED_BACK"
}

case "$MODE" in
  --dry-run)
    run_dry_run
    ;;
  --apply)
    apply_migration
    ;;
  --rollback)
    rollback_migration
    ;;
  -h|--help|help)
    cat <<EOF
Usage: $0 [--dry-run|--apply|--rollback]

Environment:
  DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD
  BATCH_SIZE (default 500)
  LEGACY_TENANT_KEY (default legacy-default)
  LEGACY_TENANT_NAME (default Legacy Default Tenant)
  LEGACY_TENANT_COUNTRY (default GH)
  LEGACY_TENANT_STATUS (default ACTIVE)
EOF
    ;;
  *)
    echo "unknown mode: $MODE" >&2
    exit 1
    ;;
esac
