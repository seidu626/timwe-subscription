#!/usr/bin/env bash
set -euo pipefail

ROOT_INPUT="${TIMWE_REPO_ROOT:-$(pwd)}"
ROOT="$(cd "$ROOT_INPUT" && pwd)"

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltnH 2>/dev/null | awk -v port="$port" '$4 ~ "(^|:)" port "$" { found = 1 } END { exit found ? 0 : 1 }'
    return $?
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -ltn 2>/dev/null | awk -v port="$port" '$4 ~ "(^|:)" port "$" { found = 1 } END { exit found ? 0 : 1 }'
    return $?
  fi
  return 1
}

port_owner() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp "sport = :${port}" 2>/dev/null | sed '1d'
    return 0
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -ltnp 2>/dev/null | awk -v port="$port" '$4 ~ "(^|:)" port "$"'
  fi
}

build_go() {
  local dir="$1" bin="$2" label="$3" package_path="$4"
  cd "${ROOT}/${dir}"
  if [ ! -x "$bin" ] || find . -path './vendor' -prune -o \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -newer "$bin" -print -quit | grep -q .; then
    echo "Building ${label}..."
    go build -o "$bin" "$package_path"
    echo "${label} built successfully"
  else
    echo "${label} binary current; skipping build."
  fi
}

run_dev_recipes() {
  local success_message="$1"
  shift

  local recipe failures=()
  for recipe in "$@"; do
    printf '\n==> just %s\n' "$recipe"
    if just "$recipe"; then
      continue
    fi
    failures+=("$recipe")
    printf 'WARN: just %s failed; continuing.\n' "$recipe"
  done

  printf '\n'
  if [ "${#failures[@]}" -eq 0 ]; then
    printf '%s\n' "$success_message"
    return 0
  fi

  printf 'Development services started with failures:\n'
  printf '  - %s\n' "${failures[@]}"
  if [ "${DEV_STRICT:-0}" = "1" ]; then
    return 1
  fi
  printf 'Set DEV_STRICT=1 to fail when any dev service cannot start.\n'
}

start_landing() {
  local dir="$1" pid_file="$2"
  local service_dir="${ROOT}/${dir}" pid_path="${ROOT}/${pid_file}"
  local existing_pid port

  if [ -f "$pid_path" ]; then
    existing_pid="$(cat "$pid_path" 2>/dev/null || true)"
    if printf '%s' "$existing_pid" | grep -Eq '^[0-9]+$' && kill -0 "$existing_pid" 2>/dev/null; then
      echo "Landing Web already running (pid ${existing_pid}); skipping start."
      return 0
    fi
    rm -f "$pid_path"
  fi

  if pgrep -f "${service_dir}/node_modules/.bin/next dev" >/dev/null 2>&1; then
    echo "Landing Web already running; skipping start."
    return 0
  fi

  cd "$service_dir"
  if [ ! -d node_modules ] || [ ! -f node_modules/.package-lock.json ] || [ package-lock.json -nt node_modules/.package-lock.json ]; then
    npm install --silent
  else
    echo "Landing Web dependencies current; skipping npm install."
  fi
  nohup setsid npm run dev > landing-web.log 2>&1 &
  echo "$!" > "$pid_path"
  sleep 4
  port="$(grep -oE 'localhost:[0-9]+' landing-web.log 2>/dev/null | head -1 | grep -oE '[0-9]+$' || true)"
  if [ -n "$port" ]; then
    echo "Landing Web started on port $port"
  else
    echo "Landing Web started; check ${dir}/landing-web.log for port"
  fi
}

start_binary() {
  local dir="$1" bin="$2" port="$3" env_name="$4" log_file="$5" pid_file="$6" label="$7" wait_seconds="${8:-5}"
  local resolved_port pid pid_path existing_pid existing_cmd
  pid_path="${ROOT}/${dir}/${pid_file}"
  if [ -f "$pid_path" ]; then
    existing_pid="$(cat "$pid_path" 2>/dev/null || true)"
    if printf '%s' "$existing_pid" | grep -Eq '^[0-9]+$' && kill -0 "$existing_pid" 2>/dev/null; then
      existing_cmd="$(ps -p "$existing_pid" -o args= 2>/dev/null || true)"
      if printf '%s' "$existing_cmd" | grep -Eq "(^|/| )${bin}( |$)"; then
        echo "${label} already running (pid ${existing_pid}); skipping start."
        return 0
      fi
    fi
    rm -f "$pid_path"
  fi

  if port_in_use "$port"; then
    echo "${label} port ${port} is already in use; refusing to start on a different port."
    port_owner "$port" || true
    exit 1
  fi

  resolved_port="$port"
  echo "Starting ${label} on port ${resolved_port}..."
  cd "${ROOT}/${dir}"
  if [ "$env_name" = "CADENCE_ADMIN_HTTP_ADDR" ]; then
    nohup setsid env "${env_name}=:${resolved_port}" "./${bin}" > "$log_file" 2>&1 &
  else
    nohup setsid env "${env_name}=${resolved_port}" "./${bin}" > "$log_file" 2>&1 &
  fi
  pid="$!"
  echo "$pid" > "$pid_file"
  for _ in $(seq 1 "$wait_seconds"); do
    port_in_use "$resolved_port" && break
    sleep 1
  done
  if port_in_use "$resolved_port"; then
    echo "${label} started on port ${resolved_port}"
  else
    echo "${label} failed to start; check ${dir}/${log_file}"
    tail -10 "$log_file" 2>/dev/null || true
    if kill -0 "$pid" 2>/dev/null; then
      pkill -TERM -P "$pid" 2>/dev/null || true
      kill -TERM "$pid" 2>/dev/null || true
      sleep 1
      if kill -0 "$pid" 2>/dev/null; then
        pkill -KILL -P "$pid" 2>/dev/null || true
        kill -KILL "$pid" 2>/dev/null || true
      fi
    fi
    rm -f "$pid_file"
    exit 1
  fi
}

stop_pid_file() {
  local pid_file="$1" pattern="$2"
  if [ ! -f "$pid_file" ]; then
    return 0
  fi

  local pid cmd child
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if printf '%s' "$pid" | grep -Eq '^[0-9]+$' && kill -0 "$pid" 2>/dev/null; then
    cmd="$(ps -p "$pid" -o args= 2>/dev/null || true)"
    if printf '%s' "$cmd" | grep -Eq "$pattern"; then
      for child in $(pgrep -P "$pid" 2>/dev/null || true); do
        pkill -TERM -P "$child" 2>/dev/null || true
        kill -TERM "$child" 2>/dev/null || true
      done
      pkill -TERM -P "$pid" 2>/dev/null || true
      kill -TERM "$pid" 2>/dev/null || true
      for _ in 1 2 3 4 5; do
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.2
      done
      if kill -0 "$pid" 2>/dev/null; then
        pkill -KILL -P "$pid" 2>/dev/null || true
        kill -KILL "$pid" 2>/dev/null || true
      fi
    else
      echo "Ignoring stale pid file ${pid_file} for pid ${pid} (${cmd})"
    fi
  fi
  rm -f "$pid_file"
}

stop_matching() {
  local pattern="$1"
  local pids pid
  pids="$(pgrep -f "$pattern" 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    kill -TERM $pids 2>/dev/null || true
    sleep 0.2
    for pid in $pids; do
      kill -0 "$pid" 2>/dev/null && kill -KILL "$pid" 2>/dev/null || true
    done
  fi
}

service_state() {
  local pid_file="$1" legacy_pid_file="$2" process_pattern="$3"
  if [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file" 2>/dev/null)" 2>/dev/null; then
    echo "Running"
  elif [ -n "$legacy_pid_file" ] && [ -f "$legacy_pid_file" ] && kill -0 "$(cat "$legacy_pid_file" 2>/dev/null)" 2>/dev/null; then
    echo "Running"
  elif pgrep -f "$process_pattern" >/dev/null 2>&1; then
    echo "Running"
  else
    echo "Stopped"
  fi
}

docker_login_check() {
  local registry="${DOCKER_PUSH_REGISTRY:-docker.io}"
  local logged_in_user
  logged_in_user="$(docker login --get-login "$registry" 2>/dev/null || true)"
  if [ -z "$logged_in_user" ]; then
    echo "Not logged into ${registry}. Run: docker login ${registry}"
    echo "If you need a different namespace, override DOCKER_USER."
    exit 1
  fi
  echo "Logged into ${registry} as ${logged_in_user}"
}

push_image() {
  local image="$1"
  local version="${VERSION:-latest}"
  local registry="${DOCKER_PUSH_REGISTRY:-docker.io}"
  local retries="${PUSH_RETRIES:-4}"
  local delay="${PUSH_RETRY_DELAY_SECONDS:-5}"
  local attempt=1

  docker tag "${image}:${version}" "${registry}/${image}:${version}"
  while [ "$attempt" -le "$retries" ]; do
    if docker push "${registry}/${image}:${version}"; then
      break
    fi
    if [ "$attempt" -eq "$retries" ]; then
      echo "Push failed after ${retries} attempts: ${registry}/${image}:${version}"
      exit 1
    fi
    echo "Push attempt ${attempt} failed for ${registry}/${image}:${version}; retrying in ${delay}s..."
    attempt=$((attempt + 1))
    sleep "$delay"
  done
}

deploy_service() {
  local service="$*"
  echo "Deploying ${service} to ${DEPLOY_SSH_HOST}..."
  ssh "$DEPLOY_SSH_HOST" "${DEPLOY_SCRIPT} ${service}"
  echo "${service} deployed successfully"
}

tenant_env() {
  local db_password db_host db_port db_user db_name
  db_password="${DB_PASSWORD:-$(sed -n 's/^APP_DATABASE_POSTGRESQL_PASSWORD=//p' "${ROOT}/.env" 2>/dev/null | tail -n 1)}"
  db_host="${DB_HOST:-$(sed -n 's/^APP_DATABASE_POSTGRESQL_HOST=//p' "${ROOT}/.env" 2>/dev/null | tail -n 1)}"
  db_port="${DB_PORT:-$(sed -n 's/^APP_DATABASE_POSTGRESQL_PORT=//p' "${ROOT}/.env" 2>/dev/null | tail -n 1)}"
  db_user="${DB_USER:-$(sed -n 's/^APP_DATABASE_POSTGRESQL_USER=//p' "${ROOT}/.env" 2>/dev/null | tail -n 1)}"
  db_name="${DB_NAME:-$(sed -n 's/^APP_DATABASE_POSTGRESQL_DB_NAME=//p' "${ROOT}/.env" 2>/dev/null | tail -n 1)}"
  if [ -z "$db_password" ]; then
    echo "ERROR: DB_PASSWORD or APP_DATABASE_POSTGRESQL_PASSWORD is required"
    exit 1
  fi
  export DB_PASSWORD="$db_password"
  export DB_HOST="${db_host:-139.59.135.253}"
  export DB_PORT="${db_port:-5432}"
  export DB_USER="${db_user:-sm_admin}"
  export DB_NAME="${db_name:-subscription_manager}"
}

psql_or_docker() {
  local sql_file="$1"
  if command -v psql >/dev/null 2>&1; then
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$sql_file"
  else
    docker run --rm \
      -e PGPASSWORD="$DB_PASSWORD" \
      -v "${ROOT}:/work" \
      -w /work \
      postgres:16 \
      psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$sql_file"
  fi
}

case "${1:-}" in
  run-dev-recipes)
    shift
    run_dev_recipes "$@"
    ;;
  start-landing)
    shift
    start_landing "$@"
    ;;
  build-go)
    shift
    build_go "$@"
    ;;
  start-binary)
    shift
    start_binary "$@"
    ;;
  stop-pid-file)
    shift
    stop_pid_file "$@"
    ;;
  stop-matching)
    shift
    stop_matching "$@"
    ;;
  status)
    echo "Service Status:"
    echo "Subscription External: $(service_state "$SUBSCRIPTION_EXTERNAL_PID_FILE" subscription-external.pid '[s]ervices/subscription-external/subscription-external$|^\./subscription-external$')"
    echo "Subscription: $(service_state "$SUBSCRIPTION_PID_FILE" subscription.pid '[s]ervices/subscription-partner/subscription$|^\./subscription$')"
    echo "Billing: $(service_state "$BILLING_PID_FILE" billing.pid '[s]ervices/billing/billing$|^\./billing$')"
    echo "Notification: $(service_state "$NOTIFICATION_PID_FILE" notification.pid '[s]ervices/notification/notification$|^\./notification$')"
    echo "Acquisition API: $(service_state "$ACQUISITION_API_PID_FILE" acquisition-api.pid '[s]ervices/acquisition-api/acquisition-api$|^\./acquisition-api$')"
    echo "Cadence Engine: $(service_state "$CADENCE_ENGINE_PID_FILE" cadence-engine.pid '[s]ervices/cadence-engine/cadence-engine$|^\./cadence-engine$')"
    echo "Landing Web: $(service_state "$LANDING_WEB_PID_FILE" landing-web.pid '[s]ervices/landing-web/node_modules/.bin/next dev')"
    echo "Admin Panel: $(service_state "$WEBSPA_ADMIN_PID_FILE" webspa-admin.pid '[n]g serve --port '"$WEBSPA_ADMIN_PORT"'|[n]px ng serve --port '"$WEBSPA_ADMIN_PORT"'|[n]ode ./node_modules/@angular/cli/bin/ng.js serve --port '"$WEBSPA_ADMIN_PORT"'|[f]rontend/webspa-admin/node_modules/@esbuild/')"
    ;;
  docker-login-check)
    docker_login_check
    ;;
  push-image)
    shift
    push_image "$@"
    ;;
  deploy-service)
    shift
    deploy_service "$@"
    ;;
  tenant-migration)
    shift
    tenant_env
    if [ "${1:-}" = "--tables" ]; then
      export MIGRATION_TABLES="$2"
      shift 2
    fi
    bash "${ROOT}/scripts/db-migrate-tenant-platform.sh" "$@"
    ;;
  db-exec-sql)
    shift
    if [ -z "${1:-}" ]; then
      echo "Error: SQL file path is required."
      exit 1
    fi
    tenant_env
    psql_or_docker "$1"
    ;;
  db-campaign-details)
    shift
    if [ -z "${1:-}" ]; then
      echo "Error: campaign slug is required."
      exit 1
    fi
    tenant_env
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT slug, country, operator, offer_product_id, partner_role_id, flow_type, price, billing_cycle, enabled, postback_rules, attribution_mapping, landing_page_urls FROM campaigns WHERE slug = '$1';"
    ;;
  db-generate-click-id)
    tenant_env
    click_id="$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c 'SELECT gen_random_uuid();')"
    printf '\n============================================================\n'
    printf 'New Click ID: %s\n' "$click_id"
    printf '============================================================\n\n'
    printf 'Use in landing page URL:\n  ?click_id=%s\n  ?txid=%s  (Mobplus format)\n\n' "$click_id" "$click_id"
    ;;
  *)
    echo "Unknown helper command: ${1:-}"
    exit 2
    ;;
esac
