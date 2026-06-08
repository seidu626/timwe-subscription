set dotenv-load
set export
set shell := ["bash", "-euo", "pipefail", "-c"]

TIMWE_REPO_ROOT := "."
TIMWE_JUST_HELPER := "./scripts/just/timwe.sh"

DOCKER_USER := env_var_or_default("DOCKER_USER", "xper626")
VERSION := env_var_or_default("VERSION", "latest")
DOCKER_PUSH_REGISTRY := env_var_or_default("DOCKER_PUSH_REGISTRY", "docker.io")
PUSH_RETRIES := env_var_or_default("PUSH_RETRIES", "4")
PUSH_RETRY_DELAY_SECONDS := env_var_or_default("PUSH_RETRY_DELAY_SECONDS", "5")
DEV_JOBS := env_var_or_default("DEV_JOBS", "8")

SUBSCRIPTION_DIR := "services/subscription-partner"
SUBSCRIPTION_EXTERNAL_DIR := "services/subscription-external"
BILLING_DIR := "services/billing"
NOTIFICATION_DIR := "services/notification"
ACQUISITION_API_DIR := "services/acquisition-api"
POSTBACK_DISPATCHER_DIR := "services/postback-dispatcher"
LANDING_WEB_DIR := "services/landing-web"
WEBSPA_ADMIN_DIR := "frontend/webspa-admin"
KRAKEND_DIR := "krakend"
CADENCE_ENGINE_DIR := "services/cadence-engine"

SUBSCRIPTION_EXTERNAL_PORT := env_var_or_default("SUBSCRIPTION_EXTERNAL_PORT", "8083")
SUBSCRIPTION_PORT := env_var_or_default("SUBSCRIPTION_PORT", "8087")
BILLING_PORT := env_var_or_default("BILLING_PORT", "8085")
NOTIFICATION_PORT := env_var_or_default("NOTIFICATION_PORT", "8082")
ACQUISITION_API_PORT := env_var_or_default("ACQUISITION_API_PORT", "8084")
WEBSPA_ADMIN_PORT := env_var_or_default("WEBSPA_ADMIN_PORT", "4200")

SUBSCRIPTION_EXTERNAL_PID_FILE := "services/subscription-external/subscription-external.pid"
SUBSCRIPTION_PID_FILE := "services/subscription-partner/subscription.pid"
BILLING_PID_FILE := "services/billing/billing.pid"
NOTIFICATION_PID_FILE := "services/notification/notification.pid"
ACQUISITION_API_PID_FILE := "services/acquisition-api/acquisition-api.pid"
CADENCE_ENGINE_PID_FILE := "services/cadence-engine/cadence-engine.pid"
LANDING_WEB_PID_FILE := "services/landing-web/landing-web.pid"
WEBSPA_ADMIN_PID_FILE := "frontend/webspa-admin/webspa-admin.pid"

SUBSCRIPTION_PARTNER_IMAGE := DOCKER_USER + "/subscription-partner"
SUBSCRIPTION_EXTERNAL_IMAGE := DOCKER_USER + "/subscription-external"
BILLING_IMAGE := DOCKER_USER + "/billing-service"
NOTIFICATION_IMAGE := DOCKER_USER + "/notification-service"
ACQUISITION_API_IMAGE := DOCKER_USER + "/acquisition-api"
POSTBACK_DISPATCHER_IMAGE := DOCKER_USER + "/postback-dispatcher"
LANDING_WEB_IMAGE := DOCKER_USER + "/landing-web"
WEBSPA_ADMIN_IMAGE := DOCKER_USER + "/nr-subscription-webspa-admin"
KRAKEND_IMAGE := DOCKER_USER + "/krakend-timwe-ma"
CADENCE_ENGINE_IMAGE := DOCKER_USER + "/cadence-engine"

DB_HOST := env_var_or_default("DB_HOST", "139.59.135.253")
DB_PORT := env_var_or_default("DB_PORT", "5432")
DB_USER := env_var_or_default("DB_USER", "sm_admin")
DB_NAME := env_var_or_default("DB_NAME", "subscription_manager")

DEPLOY_SSH_HOST := env_var_or_default("DEPLOY_SSH_HOST", "do-sa-user")
DEPLOY_SCRIPT := env_var_or_default("DEPLOY_SCRIPT", "~/services/nouveauricheglobalgroup/deploy.sh")
KRAKEND_REMOTE_CONFIG := env_var_or_default("KRAKEND_REMOTE_CONFIG", "/etc/krakend/config")
KRAKEND_LOCAL_CONFIG := "krakend/config"

default: help

[group("dev")]
dev:
    just -j "$DEV_JOBS" dev-subscription-external dev-subscription dev-billing dev-notification dev-acquisition-api dev-cadence-engine dev-landing dev-admin
    echo ""
    echo "All development services started."

[group("dev")]
dev-all:
    just -j "$DEV_JOBS" dev-subscription-external dev-subscription dev-billing dev-notification dev-acquisition-api
    echo ""
    echo "Core development services started."

[group("dev")]
dev-subscription-external: build-local-subscription-external
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ SUBSCRIPTION_EXTERNAL_DIR }}" subscription-external "$SUBSCRIPTION_EXTERNAL_PORT" APP_APPLICATION_PORT subscription-external.log subscription-external.pid "Subscription External Service" 5

[group("dev")]
dev-subscription: build-local-subscription
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ SUBSCRIPTION_DIR }}" subscription "$SUBSCRIPTION_PORT" APP_APPLICATION_PORT subscription.log subscription.pid "Subscription Service" 5

[group("dev")]
dev-billing: build-local-billing
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ BILLING_DIR }}" billing "$BILLING_PORT" APPLICATION_PORT billing.log billing.pid "Billing Service" 5

[group("dev")]
dev-notification: build-local-notification
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ NOTIFICATION_DIR }}" notification "$NOTIFICATION_PORT" APPLICATION_PORT notification.log notification.pid "Notification Service" 5

[group("dev")]
dev-acquisition-api: build-local-acquisition-api
    if [ -z "${APP_DATABASE_POSTGRESQL_PASSWORD:-}" ] && ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' "{{ ACQUISITION_API_DIR }}/.env" 2>/dev/null && ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' .env 2>/dev/null; then echo "ERROR: APP_DATABASE_POSTGRESQL_PASSWORD is not set"; exit 1; fi
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ ACQUISITION_API_DIR }}" acquisition-api "$ACQUISITION_API_PORT" APP_APPLICATION_PORT acquisition-api.log acquisition-api.pid "Acquisition API" 45

[group("dev")]
dev-cadence-engine: build-local-cadence-engine
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ CADENCE_ENGINE_DIR }}" cadence-engine 8091 CADENCE_ADMIN_HTTP_ADDR cadence-engine.log cadence-engine.pid "Cadence Engine" 10

[group("dev")]
dev-landing:
    cd "{{ LANDING_WEB_DIR }}" && if [ ! -d node_modules ] || [ ! -f node_modules/.package-lock.json ] || [ package-lock.json -nt node_modules/.package-lock.json ]; then npm install --silent; else echo "Landing Web dependencies current; skipping npm install."; fi
    cd "{{ LANDING_WEB_DIR }}" && nohup setsid npm run dev > landing-web.log 2>&1 & echo $! > landing-web.pid
    sleep 4
    port="$(grep -oE 'localhost:[0-9]+' "{{ LANDING_WEB_DIR }}/landing-web.log" 2>/dev/null | head -1 | grep -oE '[0-9]+$' || true)"; if [ -n "$port" ]; then echo "Landing Web started on port $port"; else echo "Landing Web started; check {{ LANDING_WEB_DIR }}/landing-web.log for port"; fi

[group("dev")]
dev-admin:
    cd "{{ WEBSPA_ADMIN_DIR }}" && if [ ! -d node_modules ] || [ ! -f node_modules/.package-lock.json ] || [ package-lock.json -nt node_modules/.package-lock.json ]; then npm install; else echo "Admin Panel dependencies current; skipping npm install."; fi
    port="$WEBSPA_ADMIN_PORT"; if ss -ltn 2>/dev/null | grep -q ":$port " || netstat -ltn 2>/dev/null | grep -q ":$port "; then echo "Admin Panel port $port is already in use; skipping start."; exit 0; fi; cd "{{ WEBSPA_ADMIN_DIR }}" && nohup setsid npm run start -- --port "$port" > webspa-admin.log 2>&1 & echo $! > webspa-admin.pid
    for _ in 1 2 3 4 5 6 7 8 9 10; do sleep 2; if ss -ltn 2>/dev/null | grep -q ":$WEBSPA_ADMIN_PORT " || netstat -ltn 2>/dev/null | grep -q ":$WEBSPA_ADMIN_PORT "; then break; fi; if grep -q "Compiled successfully\|Application bundle generation complete\|Watch mode enabled\|listening on\|Local:\|open your browser" "{{ WEBSPA_ADMIN_DIR }}/webspa-admin.log" 2>/dev/null; then break; fi; done
    if ss -ltn 2>/dev/null | grep -q ":$WEBSPA_ADMIN_PORT " || netstat -ltn 2>/dev/null | grep -q ":$WEBSPA_ADMIN_PORT "; then echo "Admin Panel started on port $WEBSPA_ADMIN_PORT"; else echo "Admin Panel did not bind to expected port $WEBSPA_ADMIN_PORT"; tail -20 "{{ WEBSPA_ADMIN_DIR }}/webspa-admin.log" 2>/dev/null || true; exit 1; fi

[group("service")]
start: start-subscription start-notification start-acquisition-api
    echo ""
    echo "All services started."

[group("service")]
start-all: start-subscription-external start-subscription start-billing start-notification start-acquisition-api
    echo ""
    echo "All services started."

[group("service")]
start-subscription-external:
    cd "{{ SUBSCRIPTION_EXTERNAL_DIR }}" && nohup ./subscription-external > subscription-external.log 2>&1 & echo $! > subscription-external.pid
    echo "Subscription External Service started on port {{ SUBSCRIPTION_EXTERNAL_PORT }}"

[group("service")]
start-subscription:
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ SUBSCRIPTION_DIR }}" subscription "$SUBSCRIPTION_PORT" APP_APPLICATION_PORT subscription.log subscription.pid "Subscription Service" 5

[group("service")]
start-billing:
    cd "{{ BILLING_DIR }}" && nohup ./billing > billing.log 2>&1 & echo $! > billing.pid
    echo "Billing Service started on port {{ BILLING_PORT }}"

[group("service")]
start-notification:
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ NOTIFICATION_DIR }}" notification "$NOTIFICATION_PORT" APPLICATION_PORT notification.log notification.pid "Notification Service" 5

[group("service")]
start-acquisition-api:
    if [ -z "${APP_DATABASE_POSTGRESQL_PASSWORD:-}" ] && ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' "{{ ACQUISITION_API_DIR }}/.env" 2>/dev/null && ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' .env 2>/dev/null; then echo "ERROR: APP_DATABASE_POSTGRESQL_PASSWORD is not set"; exit 1; fi
    "{{ TIMWE_JUST_HELPER }}" start-binary "{{ ACQUISITION_API_DIR }}" acquisition-api "$ACQUISITION_API_PORT" APP_APPLICATION_PORT acquisition-api.log acquisition-api.pid "Acquisition API" 45

[group("service")]
start-cadence-engine:
    cd "{{ CADENCE_ENGINE_DIR }}" && nohup ./cadence-engine > cadence-engine.log 2>&1 & echo $! > cadence-engine.pid
    echo "Cadence Engine started on port 8091"

[group("service")]
stop:
    just -j "$DEV_JOBS" stop-subscription-external stop-subscription stop-billing stop-notification stop-acquisition-api stop-cadence-engine stop-landing stop-admin
    echo "All services stopped."

[group("service")]
stop-all: stop

[group("service")]
stop-subscription-external:
    echo "Stopping Subscription External Service..."
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ SUBSCRIPTION_EXTERNAL_PID_FILE }}" '(^|/)(subscription-external|subscription-ex)( |$)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[s]ervices/subscription-external/subscription-external$'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '\./subscription-external$'

[group("service")]
stop-subscription:
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ SUBSCRIPTION_PID_FILE }}" '(^|/)subscription( |$)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[s]ervices/subscription-partner/subscription$'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '\./subscription$'

[group("service")]
stop-billing:
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ BILLING_PID_FILE }}" '(^|/)billing( |$)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[s]ervices/billing/billing$'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '\./billing$'

[group("service")]
stop-notification:
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ NOTIFICATION_PID_FILE }}" '(^|/)notification( |$)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[s]ervices/notification/notification$'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '\./notification$'

[group("service")]
stop-acquisition-api:
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ ACQUISITION_API_PID_FILE }}" '(^|/)acquisition-api( |$)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[s]ervices/acquisition-api/acquisition-api$'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '\./acquisition-api$'

[group("service")]
stop-cadence-engine:
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ CADENCE_ENGINE_PID_FILE }}" '(^|/)cadence-engine( |$)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[s]ervices/cadence-engine/cadence-engine$'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '\./cadence-engine$'

[group("service")]
stop-landing:
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ LANDING_WEB_PID_FILE }}" '(npm run dev|next dev)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[s]ervices/landing-web/node_modules/.bin/next dev'

[group("service")]
stop-admin:
    "{{ TIMWE_JUST_HELPER }}" stop-pid-file "{{ WEBSPA_ADMIN_PID_FILE }}" '(npm run start|ng serve)'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[n]g serve --port {{ WEBSPA_ADMIN_PORT }}'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[n]px ng serve --port {{ WEBSPA_ADMIN_PORT }}'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[n]ode ./node_modules/@angular/cli/bin/ng.js serve --port {{ WEBSPA_ADMIN_PORT }}'
    "{{ TIMWE_JUST_HELPER }}" stop-matching '[f]rontend/webspa-admin/node_modules/@esbuild/'

[group("service")]
restart: stop start
    echo "All services restarted."

[group("service")]
restart-subscription-external: stop-subscription-external start-subscription-external

[group("service")]
restart-subscription: stop-subscription start-subscription

[group("service")]
restart-billing: stop-billing start-billing

[group("service")]
restart-notification: stop-notification start-notification

[group("build")]
build: build-local-subscription-external build-local-subscription build-local-notification build-local-notification-worker build-local-acquisition-api build-local-cadence-engine
    echo "All services built successfully."

[group("build")]
build-all-local: build-local-subscription-external build-local-subscription build-local-billing build-local-notification build-local-notification-worker build-local-acquisition-api build-local-cadence-engine
    echo "All services built successfully."

[group("build")]
build-local-subscription-external:
    "{{ TIMWE_JUST_HELPER }}" build-go "{{ SUBSCRIPTION_EXTERNAL_DIR }}" subscription-external "Subscription External Service" cmd/main.go

[group("build")]
build-local-subscription:
    "{{ TIMWE_JUST_HELPER }}" build-go "{{ SUBSCRIPTION_DIR }}" subscription "Subscription Service" cmd/main.go

[group("build")]
build-local-billing:
    "{{ TIMWE_JUST_HELPER }}" build-go "{{ BILLING_DIR }}" billing "Billing Service" cmd/main.go

[group("build")]
build-local-notification:
    "{{ TIMWE_JUST_HELPER }}" build-go "{{ NOTIFICATION_DIR }}" notification "Notification Service" cmd/main.go

[group("build")]
build-local-notification-worker:
    "{{ TIMWE_JUST_HELPER }}" build-go "{{ NOTIFICATION_DIR }}" notification-worker "Notification Worker" cmd/notification-worker/main.go

[group("build")]
build-local-acquisition-api:
    "{{ TIMWE_JUST_HELPER }}" build-go "{{ ACQUISITION_API_DIR }}" acquisition-api "Acquisition API" cmd/main.go

[group("build")]
build-local-postback-dispatcher:
    cd "{{ POSTBACK_DISPATCHER_DIR }}" && go build -o postback-dispatcher cmd/main.go

[group("build")]
build-local-cadence-engine:
    "{{ TIMWE_JUST_HELPER }}" build-go "{{ CADENCE_ENGINE_DIR }}" cadence-engine "Cadence Engine" cmd/cadence-engine/main.go

[group("test")]
test: test-subscription-external test-subscription test-billing test-notification
    echo "All tests completed."

[group("test")]
test-subscription-external:
    cd "{{ SUBSCRIPTION_EXTERNAL_DIR }}" && go test -v ./... -cover

[group("test")]
test-subscription:
    cd "{{ SUBSCRIPTION_DIR }}" && go test -v ./... -cover

[group("test")]
test-billing:
    cd "{{ BILLING_DIR }}" && go test -v ./... -cover

[group("test")]
test-notification:
    cd "{{ NOTIFICATION_DIR }}" && go test -v ./... -cover

[group("test")]
test-legacy:
    go test -v ./... -cover

[group("ops")]
health: health-subscription-external health-subscription health-billing health-notification

[group("ops")]
health-subscription-external:
    curl -s "http://localhost:{{ SUBSCRIPTION_EXTERNAL_PORT }}/api/v1/subscription-external/monitoring/health" | jq '.health.overall_status' || echo "Service not responding"

[group("ops")]
health-subscription:
    curl -s "http://localhost:{{ SUBSCRIPTION_PORT }}/health" | jq '.status' || echo "Service not responding"

[group("ops")]
health-billing:
    curl -s "http://localhost:{{ BILLING_PORT }}/health" | jq '.status' || echo "Service not responding"

[group("ops")]
health-notification:
    curl -s "http://localhost:{{ NOTIFICATION_PORT }}/health" | jq '.status' || echo "Service not responding"

[group("ops")]
logs: logs-subscription-external logs-subscription logs-billing logs-notification

[group("ops")]
logs-subscription-external:
    tail -f "{{ SUBSCRIPTION_EXTERNAL_DIR }}"/logs/*.log || echo "No log files found"

[group("ops")]
logs-subscription:
    tail -f "{{ SUBSCRIPTION_DIR }}"/logs/*.log || echo "No log files found"

[group("ops")]
logs-billing:
    tail -f "{{ BILLING_DIR }}"/logs/*.log || echo "No log files found"

[group("ops")]
logs-notification:
    tail -f "{{ NOTIFICATION_DIR }}"/logs/*.log || echo "No log files found"

[group("ops")]
status:
    "{{ TIMWE_JUST_HELPER }}" status

[group("ops")]
quick-start: build-local-subscription-external start-subscription-external

[group("ops")]
clean: clean-subscription-external clean-subscription clean-billing clean-notification
    echo "All services cleaned."

[group("ops")]
clean-subscription-external:
    cd "{{ SUBSCRIPTION_EXTERNAL_DIR }}" && rm -f subscription-external main

[group("ops")]
clean-subscription:
    cd "{{ SUBSCRIPTION_DIR }}" && rm -f subscription

[group("ops")]
clean-billing:
    cd "{{ BILLING_DIR }}" && rm -f billing

[group("ops")]
clean-notification:
    cd "{{ NOTIFICATION_DIR }}" && rm -f notification notification-worker

[group("tools")]
tools: init update-deps

[group("tools")]
init:
    if ! [ -f "$(go env GOPATH)/bin/protoc-gen-go" ]; then go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; else echo "protoc-gen-go already installed"; fi
    if ! [ -f "$(go env GOPATH)/bin/protoc-gen-go-grpc" ]; then go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; else echo "protoc-gen-go-grpc already installed"; fi
    if ! [ -f "$(go env GOPATH)/bin/protoc-gen-micro" ]; then go install github.com/micro/micro/v3/cmd/protoc-gen-micro@latest; else echo "protoc-gen-micro already installed"; fi
    if ! [ -f "$(go env GOPATH)/bin/protoc-gen-openapi" ]; then go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest; else echo "protoc-gen-openapi already installed"; fi

[group("tools")]
update-deps:
    go mod verify
    go mod tidy

update_deps: update-deps

[group("tools")]
proto:
    protoc --openapi_out=. --proto_path=. --micro_out=. --go_out=. --go-grpc_out=. services/**/proto/*.proto

[group("tools")]
docs:
    protoc --openapi_out=. --proto_path=. --micro_out=. --go_out=. proto/tenant.proto
    redoc-cli bundle api-sm.json --options.theme.colors.primary.main=orange

[group("tools")]
generate-docs:
    swag init --generalInfo ./cmd/main.go --output ./docs

[group("docker")]
docker-build-krakend:
    docker build -t "{{ KRAKEND_IMAGE }}:{{ VERSION }}" "{{ KRAKEND_DIR }}"

[group("docker")]
krakend-query-forwarding-check:
    python3 scripts/check-krakend-query-forwarding.py

[group("docker")]
krakend-check: krakend-query-forwarding-check
    docker run --rm -v "$PWD/krakend:/etc/krakend" -e FC_ENABLE=1 -e FC_SETTINGS="/etc/krakend/config/settings" -e FC_PARTIALS="/etc/krakend/config/partials" -e FC_TEMPLATES="/etc/krakend/config/templates" docker.io/library/krakend:latest krakend check -t -c "/etc/krakend/config/krakend.tmpl"

[group("docker")]
krakend-check-do: krakend-query-forwarding-check
    docker run --rm -v "$PWD/krakend:/etc/krakend" -e FC_ENABLE=1 -e FC_SETTINGS="/etc/krakend/config/settings/do" -e FC_PARTIALS="/etc/krakend/config/partials" -e FC_TEMPLATES="/etc/krakend/config/templates" docker.io/library/krakend:latest krakend check -t -c "/etc/krakend/config/krakend.tmpl"

[group("docker")]
krakend-debug-do:
    docker run --rm -v "$PWD/krakend:/etc/krakend" -e FC_ENABLE=1 -e FC_SETTINGS="/etc/krakend/config/settings/do" -e FC_PARTIALS="/etc/krakend/config/partials" -e FC_TEMPLATES="/etc/krakend/config/templates" docker.io/library/krakend:latest krakend check -d -c "/etc/krakend/config/krakend.tmpl"

[group("docker")]
docker-build-subscription-partner:
    docker build -t "{{ SUBSCRIPTION_PARTNER_IMAGE }}:{{ VERSION }}" "{{ SUBSCRIPTION_DIR }}"

[group("docker")]
docker-build-subscription-external:
    cd "{{ SUBSCRIPTION_EXTERNAL_DIR }}" && go mod vendor
    docker build -t "{{ SUBSCRIPTION_EXTERNAL_IMAGE }}:{{ VERSION }}" "{{ SUBSCRIPTION_EXTERNAL_DIR }}"

[group("docker")]
vendor-check:
    ./scripts/check-vendor-sync.sh

[group("docker")]
docker-build-billing:
    docker build -t "{{ BILLING_IMAGE }}:{{ VERSION }}" "{{ BILLING_DIR }}"

[group("docker")]
docker-build-notification:
    docker build -t "{{ NOTIFICATION_IMAGE }}:{{ VERSION }}" "{{ NOTIFICATION_DIR }}"

[group("docker")]
docker-build-acquisition-api:
    docker build -f "{{ ACQUISITION_API_DIR }}/Dockerfile" -t "{{ ACQUISITION_API_IMAGE }}:{{ VERSION }}" .

[group("docker")]
docker-build-postback-dispatcher:
    docker build -t "{{ POSTBACK_DISPATCHER_IMAGE }}:{{ VERSION }}" "{{ POSTBACK_DISPATCHER_DIR }}"

[group("docker")]
docker-build-landing-web:
    docker build -t "{{ LANDING_WEB_IMAGE }}:{{ VERSION }}" "{{ LANDING_WEB_DIR }}"

[group("docker")]
docker-build-webspa-admin:
    docker build -t "{{ WEBSPA_ADMIN_IMAGE }}:{{ VERSION }}" "{{ WEBSPA_ADMIN_DIR }}"

[group("docker")]
docker-build-cadence-engine:
    docker build -t "{{ CADENCE_ENGINE_IMAGE }}:{{ VERSION }}" "{{ CADENCE_ENGINE_DIR }}"

[group("docker")]
docker-build-core: docker-build-subscription-partner docker-build-subscription-external docker-build-notification docker-build-acquisition-api docker-build-cadence-engine

[group("docker")]
docker-build-all: docker-build-krakend docker-build-subscription-partner docker-build-subscription-external docker-build-notification docker-build-acquisition-api docker-build-postback-dispatcher docker-build-landing-web docker-build-webspa-admin docker-build-cadence-engine

build-krakend: docker-build-krakend
build-subscription: docker-build-subscription-partner
build-subscription-external: docker-build-subscription-external
build-billing: docker-build-billing
build-notification: docker-build-notification
build-acquisition-api: docker-build-acquisition-api
build-postback-dispatcher: docker-build-postback-dispatcher
build-landing-web: docker-build-landing-web
build-webspa-admin: docker-build-webspa-admin
build-cadence-engine: docker-build-cadence-engine
build-subSvc-notSvc: docker-build-subscription-partner docker-build-subscription-external docker-build-notification
build-all: docker-build-all

[group("docker")]
docker-login-check:
    "{{ TIMWE_JUST_HELPER }}" docker-login-check

[group("docker")]
docker-push-krakend: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ KRAKEND_IMAGE }}"

[group("docker")]
docker-push-subscription-partner: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ SUBSCRIPTION_PARTNER_IMAGE }}"

[group("docker")]
docker-push-subscription-external: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ SUBSCRIPTION_EXTERNAL_IMAGE }}"

[group("docker")]
docker-push-billing: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ BILLING_IMAGE }}"

[group("docker")]
docker-push-notification: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ NOTIFICATION_IMAGE }}"

[group("docker")]
docker-push-acquisition-api: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ ACQUISITION_API_IMAGE }}"

[group("docker")]
docker-push-postback-dispatcher: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ POSTBACK_DISPATCHER_IMAGE }}"

[group("docker")]
docker-push-landing-web: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ LANDING_WEB_IMAGE }}"

[group("docker")]
docker-push-webspa-admin: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ WEBSPA_ADMIN_IMAGE }}"

[group("docker")]
docker-push-cadence-engine: docker-login-check
    "{{ TIMWE_JUST_HELPER }}" push-image "{{ CADENCE_ENGINE_IMAGE }}"

[group("docker")]
docker-push-core: docker-push-subscription-partner docker-push-subscription-external docker-push-notification docker-push-acquisition-api docker-push-cadence-engine

[group("docker")]
docker-push-all: docker-push-krakend docker-push-subscription-partner docker-push-subscription-external docker-push-notification docker-push-acquisition-api docker-push-postback-dispatcher docker-push-landing-web docker-push-webspa-admin docker-push-cadence-engine

push-krakend: docker-push-krakend
push-subscription: docker-push-subscription-partner
push-subscription-external: docker-push-subscription-external
push-billing: docker-push-billing
push-notification: docker-push-notification
push-acquisition-api: docker-push-acquisition-api
push-postback-dispatcher: docker-push-postback-dispatcher
push-landing-web: docker-push-landing-web
push-webspa-admin: docker-push-webspa-admin
push-cadence-engine: docker-push-cadence-engine
push-subSvc-notSvc: docker-push-subscription-partner docker-push-subscription-external docker-push-notification
push-all: docker-push-all

[group("docker")]
docker-release-krakend: docker-build-krakend docker-push-krakend
docker-release-subscription-partner: docker-build-subscription-partner docker-push-subscription-partner
docker-release-subscription-external: docker-build-subscription-external docker-push-subscription-external
docker-release-billing: docker-build-billing docker-push-billing
docker-release-notification: docker-build-notification docker-push-notification
docker-release-acquisition-api: docker-build-acquisition-api docker-push-acquisition-api
docker-release-postback-dispatcher: docker-build-postback-dispatcher docker-push-postback-dispatcher
docker-release-landing-web: docker-build-landing-web docker-push-landing-web
docker-release-webspa-admin: docker-build-webspa-admin docker-push-webspa-admin
docker-release-cadence-engine: docker-build-cadence-engine docker-push-cadence-engine
docker-release-core: docker-build-core docker-push-core
docker-release-all: docker-build-all docker-push-all

release-subscription: docker-release-subscription-partner
release-billing: docker-release-billing
release-notification: docker-release-notification
release-all: docker-release-all

[group("deploy")]
krakend-sync: krakend-check-do
    rsync -av --delete "{{ KRAKEND_LOCAL_CONFIG }}/templates/" "{{ DEPLOY_SSH_HOST }}:{{ KRAKEND_REMOTE_CONFIG }}/templates/"
    rsync -av --delete "{{ KRAKEND_LOCAL_CONFIG }}/partials/" "{{ DEPLOY_SSH_HOST }}:{{ KRAKEND_REMOTE_CONFIG }}/partials/"
    rsync -av "{{ KRAKEND_LOCAL_CONFIG }}/settings/" "{{ DEPLOY_SSH_HOST }}:{{ KRAKEND_REMOTE_CONFIG }}/settings/"
    rsync -av "{{ KRAKEND_LOCAL_CONFIG }}/krakend.tmpl" "{{ DEPLOY_SSH_HOST }}:{{ KRAKEND_REMOTE_CONFIG }}/krakend.tmpl"
    ssh "{{ DEPLOY_SSH_HOST }}" "{{ DEPLOY_SCRIPT }} --krakend"

deploy-krakend: krakend-sync
deploy-subscription-partner: docker-release-subscription-partner
    "{{ TIMWE_JUST_HELPER }}" deploy-service subscription-partner
deploy-subscription-external: docker-release-subscription-external
    "{{ TIMWE_JUST_HELPER }}" deploy-service subscription-external
deploy-billing: docker-release-billing
    "{{ TIMWE_JUST_HELPER }}" deploy-service billing
deploy-notification: docker-release-notification
    "{{ TIMWE_JUST_HELPER }}" deploy-service notification
deploy-acquisition-api: docker-release-acquisition-api
    "{{ TIMWE_JUST_HELPER }}" deploy-service acquisition-api
deploy-postback-dispatcher: docker-release-postback-dispatcher
    "{{ TIMWE_JUST_HELPER }}" deploy-service postback-dispatcher
deploy-landing-web: docker-release-landing-web
    "{{ TIMWE_JUST_HELPER }}" deploy-service landing-web
deploy-webspa-admin: docker-release-webspa-admin
    "{{ TIMWE_JUST_HELPER }}" deploy-service webspa-admin
deploy-cadence-engine: docker-release-cadence-engine
    "{{ TIMWE_JUST_HELPER }}" deploy-service cadence-engine
deploy-core: docker-release-core
    "{{ TIMWE_JUST_HELPER }}" deploy-service subscription-partner subscription-external notification acquisition-api cadence-engine
deploy-all: docker-release-all krakend-sync
    ssh "{{ DEPLOY_SSH_HOST }}" "{{ DEPLOY_SCRIPT }}"

[group("compose")]
package:
    docker-compose down --remove-orphans -v
    docker-compose build

[group("compose")]
compose-up:
    docker compose up --build -d && docker compose logs -f

[group("compose")]
compose-down:
    docker compose down --remove-orphans

[group("compose")]
compose-prod-up:
    docker compose -f docker-compose.prod.yml up -d

[group("compose")]
compose-prod-down:
    docker compose -f docker-compose.prod.yml down

[group("compose")]
compose-prod-logs:
    docker compose -f docker-compose.prod.yml logs -f

[group("compose")]
compose-do-up:
    docker compose -f docker-compose.prod-do.yml up -d

[group("compose")]
compose-do-down:
    docker compose -f docker-compose.prod-do.yml down

[group("compose")]
compose-do-logs:
    docker compose -f docker-compose.prod-do.yml logs -f

[group("compose")]
compose-do-pull:
    docker compose -f docker-compose.prod-do.yml pull

[group("compose")]
compose-do-restart: compose-do-down compose-do-pull compose-do-up

[group("compose")]
docker-ps:
    docker ps --format 'table {{ "{{.Names}}\t{{.Status}}\t{{.Ports}}" }}'

[group("compose")]
docker-images:
    docker images --format 'table {{ "{{.Repository}}\t{{.Tag}}\t{{.Size}}" }}' | grep -E "{{ DOCKER_USER }}|REPOSITORY"

[group("docker")]
clean-docker:
    docker rmi $(docker images -f "dangling=true" -q) || true

[group("docker")]
docker_clean:
    docker rm -v $(docker ps --filter status=exited -q)

[group("docker")]
docker_clean_images:
    docker images -f "dangling=true" -q | xargs -r docker rmi
    docker images -f "label=org.label-schema.vendor=sumo" -q | xargs -r docker rmi
    docker image prune -f

[group("docker")]
docker_push:
    vcs_ref="$(git rev-parse --short HEAD)"; docker images -f "label=org.label-schema.vcs-ref=$vcs_ref" --format '{{ "{{.Repository}}:{{.Tag}}" }}' | while read -r image; do echo "Now pushing $image"; docker push "$image"; done

[group("database")]
migrate: migrate-subscription-external db-migrate-cadence

[group("database")]
migrate-subscription-external:
    PGPASSWORD="$DB_PASSWORD" psql -h "{{ DB_HOST }}" -p "{{ DB_PORT }}" -U "{{ DB_USER }}" -d "{{ DB_NAME }}" -f services/subscription-external/migrations/011_message_cadence_engine.sql

[group("database")]
db-connect:
    PGPASSWORD="$DB_PASSWORD" psql -h "{{ DB_HOST }}" -p "{{ DB_PORT }}" -U "{{ DB_USER }}" -d "{{ DB_NAME }}"

[group("database")]
db-exec-sql file="":
    "{{ TIMWE_JUST_HELPER }}" db-exec-sql "{{ file }}"

[group("database")]
db-migrate-campaigns:
    for file in services/subscription-external/migrations/006_web_acquisition_campaigns.sql services/subscription-external/migrations/007_add_charge_tracking_columns.sql services/subscription-external/migrations/008_landing_events.sql services/subscription-external/migrations/009_campaign_landing_page_urls.sql services/subscription-external/migrations/010_outbound_clicks.sql services/subscription-external/migrations/012_campaign_tracking_config.sql services/subscription-external/migrations/013_he_tracking.sql services/subscription-external/migrations/014_campaign_lp_copy.sql services/acquisition-api/migrations/add_admin_management_tables.sql services/acquisition-api/migrations/add_acquisition_transaction_offer_context.sql services/acquisition-api/migrations/update_ghana_lp_copy_msisdn_format.sql; do PGPASSWORD="$DB_PASSWORD" psql -h "{{ DB_HOST }}" -p "{{ DB_PORT }}" -U "{{ DB_USER }}" -d "{{ DB_NAME }}" -f "$file"; done

[group("database")]
db-migrate-cadence:
    PGPASSWORD="$DB_PASSWORD" psql -h "{{ DB_HOST }}" -p "{{ DB_PORT }}" -U "{{ DB_USER }}" -d "{{ DB_NAME }}" -f services/subscription-external/migrations/011_message_cadence_engine.sql

[group("database")]
db-migrate-tenant-platform-dry-run:
    "{{ TIMWE_JUST_HELPER }}" tenant-migration --dry-run

[group("database")]
db-migrate-tenant-platform:
    "{{ TIMWE_JUST_HELPER }}" tenant-migration --apply

[group("database")]
db-migrate-nrg-subscriptions-transactions-dry-run:
    "{{ TIMWE_JUST_HELPER }}" tenant-migration --tables subscriptions,acquisition_transactions --dry-run

[group("database")]
db-migrate-nrg-subscriptions-transactions:
    "{{ TIMWE_JUST_HELPER }}" tenant-migration --tables subscriptions,acquisition_transactions --apply

[group("database")]
db-create-mobplus-campaign:
    PGPASSWORD="$DB_PASSWORD" psql -h "{{ DB_HOST }}" -p "{{ DB_PORT }}" -U "{{ DB_USER }}" -d "{{ DB_NAME }}" -f services/acquisition-api/migrations/create_mobplus_campaign.sql

[group("database")]
db-configure-level23-campaign:
    "{{ TIMWE_JUST_HELPER }}" db-exec-sql services/acquisition-api/migrations/configure_level23_campaign.sql

[group("database")]
db-generate-level23-share-info:
    "{{ TIMWE_JUST_HELPER }}" db-exec-sql services/acquisition-api/migrations/generate_level23_share_info.sql

[group("database")]
db-update-gh-lp-copy-msisdn-format:
    "{{ TIMWE_JUST_HELPER }}" db-exec-sql services/acquisition-api/migrations/update_ghana_lp_copy_msisdn_format.sql

[group("database")]
db-list-campaigns:
    PGPASSWORD="$DB_PASSWORD" psql -h "{{ DB_HOST }}" -p "{{ DB_PORT }}" -U "{{ DB_USER }}" -d "{{ DB_NAME }}" -c "SELECT id, slug, country, operator, offer_product_id, flow_type, price, billing_cycle, enabled, created_at FROM campaigns ORDER BY created_at DESC;"

[group("database")]
db-campaign-details slug="":
    "{{ TIMWE_JUST_HELPER }}" db-campaign-details "{{ slug }}"

[group("database")]
db-generate-click-id:
    "{{ TIMWE_JUST_HELPER }}" db-generate-click-id

[group("meta")]
help:
    just --list
