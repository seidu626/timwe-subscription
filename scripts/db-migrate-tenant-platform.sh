#!/usr/bin/env bash

set -euo pipefail

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
DB_PASSWORD="${DB_PASSWORD:-}"
BATCH_SIZE="${BATCH_SIZE:-500}"
CANONICAL_TENANT_KEY="${CANONICAL_TENANT_KEY:-nrg}"
CANONICAL_TENANT_NAME="${CANONICAL_TENANT_NAME:-NRG}"
CANONICAL_TENANT_COUNTRY="${CANONICAL_TENANT_COUNTRY:-GH}"
CANONICAL_TENANT_STATUS="${CANONICAL_TENANT_STATUS:-ACTIVE}"
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

REQUESTED_BACKFILL_TABLES="${MIGRATION_TABLES:-${TENANT_MIGRATION_TABLES:-}}"
if [[ -n "$REQUESTED_BACKFILL_TABLES" ]]; then
  declare -A allowed_tables=()
  for table in "${BACKFILL_TABLES[@]}"; do
    allowed_tables["$table"]=1
  done

  IFS=',' read -r -a requested_tables <<< "$REQUESTED_BACKFILL_TABLES"
  selected_tables=()
  for requested in "${requested_tables[@]}"; do
    table="${requested//[[:space:]]/}"
    if [[ -z "$table" ]]; then
      continue
    fi
    if [[ -z "${allowed_tables[$table]:-}" ]]; then
      echo "ERROR: unsupported migration table: ${table}" >&2
      echo "Allowed tables: ${BACKFILL_TABLES[*]}" >&2
      exit 1
    fi
    selected_tables+=("$table")
  done

  if [[ "${#selected_tables[@]}" -eq 0 ]]; then
    echo "ERROR: MIGRATION_TABLES did not include any supported table names" >&2
    exit 1
  fi

  BACKFILL_TABLES=("${selected_tables[@]}")
fi

eligible_predicate_for_table() {
  echo "tenant_id IS NULL"
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
  WHERE tenant_id IS NULL
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
  WHERE tenant_id IS NULL
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
  WHERE tenant_id IS NULL
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
  done

  return "$missing"
}

get_canonical_tenant_id() {
  query_text "
SELECT id
FROM tenants
WHERE tenant_key = '${CANONICAL_TENANT_KEY}'
LIMIT 1;
" | tail -n 1
}

upsert_canonical_tenant() {
  query_text "
INSERT INTO tenants (id, tenant_key, name, status, default_country, metadata_json)
VALUES (
  gen_random_uuid(),
  '${CANONICAL_TENANT_KEY}',
  '${CANONICAL_TENANT_NAME}',
  '${CANONICAL_TENANT_STATUS}',
  '${CANONICAL_TENANT_COUNTRY}',
  jsonb_build_object('migration', 'TMP-050', 'kind', 'canonical-default')
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

count_canonical_rows() {
  local table="$1"
  if [[ -z "${CANONICAL_TENANT_ID:-}" ]]; then
    echo 0
    return 0
  fi
  query_scalar "SELECT COUNT(*) FROM ${table} WHERE tenant_id = '${CANONICAL_TENANT_ID}'::uuid"
}

tenant_assignment_for_table() {
  local table="$1"
  case "$table" in
    admin_activity_logs)
      cat <<SQL
COALESCE(
  (
    SELECT t.id
    FROM tenants t
    WHERE target.entity_type = 'tenant'
      AND target.entity_id = t.id::text
    LIMIT 1
  ),
  '${CANONICAL_TENANT_ID}'::uuid
)
SQL
      ;;
    *)
      echo "'${CANONICAL_TENANT_ID}'::uuid"
      ;;
  esac
}

report_table_state() {
  local table="$1"
  local eligible_predicate eligible_rows canonical_rows conflict_groups batches status
  eligible_predicate="$(eligible_predicate_for_table "$table")"
  eligible_rows="$(count_rows "$table" "$eligible_predicate")"
  canonical_rows="$(count_canonical_rows "$table")"
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
  if [[ "$conflict_groups" -gt 0 ]]; then
    status="BLOCKED"
  fi
  printf "%-28s %14s %14s %14s %14s %s\n" "$table" "$eligible_rows" "$canonical_rows" "$conflict_groups" "$batches" "$status"
}

print_summary_header() {
  printf "%-28s %14s %14s %14s %14s %s\n" "table" "eligible_rows" "canonical_rows" "conflict_groups" "batches" "status"
  printf "%-28s %14s %14s %14s %14s %s\n" "-----" "-------------" "--------------" "---------------" "-------" "------"
}

run_dry_run() {
  local schema_ready=0
  if ! ensure_schema_prerequisites; then
    schema_ready=1
  fi
  CANONICAL_TENANT_ID="$(get_canonical_tenant_id)"
  echo "tenant-platform migration dry-run"
  echo "canonical_tenant_key=${CANONICAL_TENANT_KEY}"
  if [[ -n "$CANONICAL_TENANT_ID" ]]; then
    echo "canonical_tenant_id=${CANONICAL_TENANT_ID}"
    echo "canonical_tenant_present=yes"
  else
    echo "canonical_tenant_id=will_create_on_apply"
    echo "canonical_tenant_present=no"
  fi
  echo "batch_size=${BATCH_SIZE}"
  echo ""
  print_summary_header

  local any_blocked=0
  local total_eligible=0
  for table in "${BACKFILL_TABLES[@]}"; do
    local eligible_rows conflict_groups
    eligible_rows="$(count_rows "$table" "$(eligible_predicate_for_table "$table")")"
    conflict_groups="0"
    local conflict_query
    conflict_query="$(conflict_query_for_table "$table")"
    if [[ -n "$conflict_query" ]]; then
      conflict_groups="$(query_scalar "$conflict_query")"
    fi
    total_eligible=$((total_eligible + eligible_rows))
    if [[ "$conflict_groups" -gt 0 ]]; then
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
  local assignment
  assignment="$(tenant_assignment_for_table "$table")"
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
SET tenant_id = ${assignment}
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
  CANONICAL_TENANT_ID="$(upsert_canonical_tenant)"

  local blocked_tables=0
  for table in "${BACKFILL_TABLES[@]}"; do
    local conflict_groups
    conflict_groups="0"
    local conflict_query
    conflict_query="$(conflict_query_for_table "$table")"
    if [[ -n "$conflict_query" ]]; then
      conflict_groups="$(query_scalar "$conflict_query")"
    fi
    if [[ "$conflict_groups" -gt 0 ]]; then
      echo "BLOCKED ${table} conflict_groups=${conflict_groups}" >&2
      blocked_tables=1
    fi
  done

  if [[ "$blocked_tables" -ne 0 ]]; then
    echo "tenant-platform migration aborted before any data changes" >&2
    exit 1
  fi

  echo "applying tenant-platform migration"
  echo "canonical_tenant_id=${CANONICAL_TENANT_ID}"

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
    echo "tenant-platform migration left eligible tenantless rows behind" >&2
    exit 1
  fi

  echo "readiness=APPLIED"
}

case "$MODE" in
  --dry-run)
    run_dry_run
    ;;
  --apply)
    apply_migration
    ;;
  -h|--help|help)
    cat <<EOF
Usage: $0 [--dry-run|--apply]

Environment:
  DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD
  BATCH_SIZE (default 500)
  MIGRATION_TABLES (optional comma-separated subset, e.g. subscriptions,acquisition_transactions)
  CANONICAL_TENANT_KEY (default nrg)
  CANONICAL_TENANT_NAME (default NRG)
  CANONICAL_TENANT_COUNTRY (default GH)
  CANONICAL_TENANT_STATUS (default ACTIVE)
EOF
    ;;
  *)
    echo "unknown mode: $MODE" >&2
    exit 1
    ;;
esac
