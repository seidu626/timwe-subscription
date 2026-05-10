# Usage:
# make        	# compile all binary
# make clean  	# remove ALL binaries and objects
# make release  # add git TAG and push
GITHUB_REPO_OWNER 				:= seidu.abdulai
GITHUB_REPO_NAME 					:= sm
GITHUB_RELEASES_UI_URL 		:= https://github.com/$(GITHUB_REPO_OWNER)/$(GITHUB_REPO_NAME)/releases
GITHUB_RELEASES_API_URL 	:= https://api.github.com/repos/$(GITHUB_REPO_OWNER)/$(GITHUB_REPO_NAME)/releases
GITHUB_RELEASE_ASSET_URL	:= https://uploads.github.com/repos/$(GITHUB_REPO_OWNER)/$(GITHUB_REPO_NAME)/releases
GITHUB_DEPLOY_API_URL			:= https://api.github.com/repos/$(GITHUB_REPO_OWNER)/$(GITHUB_REPO_NAME)/deployments
DOCKER_REGISTRY 					:= docker.pkg.github.com
# DOCKER_REGISTRY 					:= us.gcr.io
DOCKER_CONTEXT_PATH 			:= $(GITHUB_REPO_OWNER)/$(GITHUB_REPO_NAME)
# DOCKER_REGISTRY 					:= docker.io
# DOCKER_CONTEXT_PATH 			:= seidu.abdulai
GO_MICRO_VERSION 					:= latest

#VERSION					:= $(shell git describe --tags || echo "HEAD")
GOPATH					:= $(shell go env GOPATH)
CODECOV_FILE 			:= build/coverage.txt
TIMEOUT  				:= 60s
# don't override
#GIT_TAG					:= $(shell git describe --tags --abbrev=0 --always --match "v*")
#GIT_DIRTY 				:= $(shell git status --porcelain 2> /dev/null)
#GIT_BRANCH  			:= $(shell git rev-parse --abbrev-ref HEAD)

# Variables
DOCKER_USER ?= xper626
VERSION ?= latest
DOCKER_PUSH_REGISTRY ?= docker.io
PUSH_RETRIES ?= 4
PUSH_RETRY_DELAY_SECONDS ?= 5

# Directories
SUBSCRIPTION_DIR = services/subscription-partner
SUBSCRIPTION_EXTERNAL_DIR = services/subscription-external
BILLING_DIR = services/billing
NOTIFICATION_DIR = services/notification
ACQUISITION_API_DIR = services/acquisition-api
POSTBACK_DISPATCHER_DIR = services/postback-dispatcher
LANDING_WEB_DIR = services/landing-web
WEBSPA_ADMIN_DIR = frontend/webspa-admin
KRAKEND_DIR = krakend
CADENCE_ENGINE_DIR = services/cadence-engine

# Docker image names (consistent with docker-compose.prod-do.yml)
SUBSCRIPTION_PARTNER_IMAGE = $(DOCKER_USER)/subscription-partner
SUBSCRIPTION_EXTERNAL_IMAGE = $(DOCKER_USER)/subscription-external
BILLING_IMAGE = $(DOCKER_USER)/billing-service
NOTIFICATION_IMAGE = $(DOCKER_USER)/notification-service
ACQUISITION_API_IMAGE = $(DOCKER_USER)/acquisition-api
POSTBACK_DISPATCHER_IMAGE = $(DOCKER_USER)/postback-dispatcher
LANDING_WEB_IMAGE = $(DOCKER_USER)/landing-web
WEBSPA_ADMIN_IMAGE = $(DOCKER_USER)/nr-subscription-webspa-admin
KRAKEND_IMAGE = $(DOCKER_USER)/krakend-timwe-ma
CADENCE_ENGINE_IMAGE = $(DOCKER_USER)/cadence-engine
GOPATH := $(shell go env GOPATH)

# Service ports
SUBSCRIPTION_EXTERNAL_PORT = 8083
SUBSCRIPTION_PORT = 8087
BILLING_PORT = 8085
NOTIFICATION_PORT = 8082
ACQUISITION_API_PORT = 8084
KRAKEND_PORT = 8080
LANDING_WEB_PORT = 3000
WEBSPA_ADMIN_PORT = 4200
SUBSCRIPTION_EXTERNAL_PID_FILE = $(SUBSCRIPTION_EXTERNAL_DIR)/subscription-external.pid
SUBSCRIPTION_PID_FILE = $(SUBSCRIPTION_DIR)/subscription.pid
BILLING_PID_FILE = $(BILLING_DIR)/billing.pid
NOTIFICATION_PID_FILE = $(NOTIFICATION_DIR)/notification.pid
ACQUISITION_API_PID_FILE = $(ACQUISITION_API_DIR)/acquisition-api.pid
CADENCE_ENGINE_PID_FILE = $(CADENCE_ENGINE_DIR)/cadence-engine.pid
LANDING_WEB_PID_FILE = $(LANDING_WEB_DIR)/landing-web.pid
WEBSPA_ADMIN_PID_FILE = $(WEBSPA_ADMIN_DIR)/webspa-admin.pid

# Development targets
.PHONY: dev
dev: dev-subscription-external dev-subscription dev-billing dev-notification dev-acquisition-api dev-cadence-engine dev-landing dev-admin
	@echo ""
	@echo "🚀 All development services started! (see ports above)"

.PHONY: dev-all
dev-all: dev-subscription-external dev-subscription dev-billing dev-notification dev-acquisition-api
	@echo ""
	@echo "🚀 All development services started! (see ports above)"

# Development service targets (build and run in background)
.PHONY: dev-subscription-external
dev-subscription-external: build-local-subscription-external
	@echo "🚀 Starting Subscription External Service (with Monitoring & Worker)..."
	@PORT=$(SUBSCRIPTION_EXTERNAL_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(SUBSCRIPTION_EXTERNAL_DIR); APP_APPLICATION_PORT=$$PORT nohup ./subscription-external > subscription-external.log 2>&1 & echo $$! > subscription-external.pid ); \
	sleep 2; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Subscription External Service started on port $$PORT"; \
	else \
		echo "❌ Subscription External Service failed to start (check $(SUBSCRIPTION_EXTERNAL_DIR)/subscription-external.log)"; \
		tail -5 $(SUBSCRIPTION_EXTERNAL_DIR)/subscription-external.log 2>/dev/null || true; \
	fi

.PHONY: dev-subscription
dev-subscription: build-local-subscription
	@echo "📱 Starting Subscription Service..."
	@PORT=$(SUBSCRIPTION_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(SUBSCRIPTION_DIR); APP_APPLICATION_PORT=$$PORT nohup ./subscription > subscription.log 2>&1 & echo $$! > subscription.pid ); \
	sleep 2; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Subscription Service started on port $$PORT"; \
	else \
		echo "❌ Subscription Service failed to start (check $(SUBSCRIPTION_DIR)/subscription.log)"; \
		tail -5 $(SUBSCRIPTION_DIR)/subscription.log 2>/dev/null || true; \
	fi

.PHONY: dev-billing
dev-billing: build-local-billing
	@echo "💰 Starting Billing Service..."
	@PORT=$(BILLING_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(BILLING_DIR); APPLICATION_PORT=$$PORT nohup ./billing > billing.log 2>&1 & echo $$! > billing.pid ); \
	sleep 2; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Billing Service started on port $$PORT"; \
	else \
		echo "❌ Billing Service failed to start (check $(BILLING_DIR)/billing.log)"; \
		tail -5 $(BILLING_DIR)/billing.log 2>/dev/null || true; \
	fi

.PHONY: dev-notification
dev-notification: build-local-notification
	@echo "🔔 Starting Notification Service..."
	@PORT=$(NOTIFICATION_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(NOTIFICATION_DIR); APPLICATION_PORT=$$PORT nohup ./notification > notification.log 2>&1 & echo $$! > notification.pid ); \
	sleep 2; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Notification Service started on port $$PORT"; \
	else \
		echo "❌ Notification Service failed to start (check $(NOTIFICATION_DIR)/notification.log)"; \
		tail -5 $(NOTIFICATION_DIR)/notification.log 2>/dev/null || true; \
	fi

.PHONY: dev-acquisition-api
dev-acquisition-api: build-local-acquisition-api
	@echo "📲 Starting Acquisition API..."
	@# Check for DB password in env or .env file
	@if [ -z "$$APP_DATABASE_POSTGRESQL_PASSWORD" ]; then \
		if ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' $(ACQUISITION_API_DIR)/.env 2>/dev/null && \
		   ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' .env 2>/dev/null; then \
			echo "❌ ERROR: APP_DATABASE_POSTGRESQL_PASSWORD is not set"; \
			echo "   Add it to $(ACQUISITION_API_DIR)/.env or export it:"; \
			echo "   echo 'APP_DATABASE_POSTGRESQL_PASSWORD=your_password' >> $(ACQUISITION_API_DIR)/.env"; \
			exit 1; \
		fi; \
	fi
	@PORT=$(ACQUISITION_API_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(ACQUISITION_API_DIR); APP_APPLICATION_PORT=$$PORT nohup ./acquisition-api > acquisition-api.log 2>&1 & echo $$! > acquisition-api.pid ); \
	for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
		if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
			break; \
		fi; \
		sleep 1; \
	done; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Acquisition API started on port $$PORT"; \
	else \
		echo "❌ Acquisition API failed to start (check $(ACQUISITION_API_DIR)/acquisition-api.log)"; \
		tail -10 $(ACQUISITION_API_DIR)/acquisition-api.log 2>/dev/null || true; \
	fi

.PHONY: dev-cadence-engine
dev-cadence-engine: build-local-cadence-engine
	@echo "⏰ Starting Cadence Engine..."
	@PORT=8091; \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(CADENCE_ENGINE_DIR); CADENCE_ADMIN_HTTP_ADDR=:$$PORT nohup ./cadence-engine > cadence-engine.log 2>&1 & echo $$! > cadence-engine.pid ); \
	for i in 1 2 3 4 5 6 7 8 9 10; do \
		if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
			break; \
		fi; \
		sleep 1; \
	done; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Cadence Engine started on port $$PORT"; \
	else \
		echo "❌ Cadence Engine failed to start (check $(CADENCE_ENGINE_DIR)/cadence-engine.log)"; \
		tail -5 $(CADENCE_ENGINE_DIR)/cadence-engine.log 2>/dev/null || true; \
	fi

.PHONY: dev-landing
dev-landing:
	@echo "🌐 Starting Landing Web (Next.js)..."
	@cd $(LANDING_WEB_DIR) && npm install --silent 2>/dev/null || true
	@(cd $(LANDING_WEB_DIR); nohup npm run dev > landing-web.log 2>&1 & echo $$! > landing-web.pid)
	@sleep 4
	@LANDING_PORT=$$(grep -oE 'localhost:[0-9]+' $(LANDING_WEB_DIR)/landing-web.log 2>/dev/null | head -1 | grep -oE '[0-9]+$$'); \
	if [ -n "$$LANDING_PORT" ]; then \
		echo "✅ Landing Web started on port $$LANDING_PORT"; \
	else \
		echo "✅ Landing Web started (check $(LANDING_WEB_DIR)/landing-web.log for port)"; \
	fi

.PHONY: dev-admin
dev-admin:
	@echo "🖥️  Starting Admin Panel (Angular)..."
	@cd $(WEBSPA_ADMIN_DIR) && npm install --silent 2>/dev/null || true
	@PORT=$(WEBSPA_ADMIN_PORT); \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "ℹ️  Admin Panel port $$PORT is already in use; skipping start."; \
		echo "   Use 'make stop-admin' first if you want to restart it."; \
		exit 0; \
	fi; \
	(cd $(WEBSPA_ADMIN_DIR); nohup npx ng serve --port $$PORT > webspa-admin.log 2>&1 & echo $$! > webspa-admin.pid)
	@echo "   Waiting for Angular to compile..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		sleep 2; \
		if grep -q "Compiled successfully\|listening on\|open your browser" $(WEBSPA_ADMIN_DIR)/webspa-admin.log 2>/dev/null; then \
			break; \
		fi; \
	done
	@ADMIN_PORT=$$(grep -oE 'localhost:[0-9]+' $(WEBSPA_ADMIN_DIR)/webspa-admin.log 2>/dev/null | head -1 | grep -oE '[0-9]+$$'); \
	if [ "$$ADMIN_PORT" = "$(WEBSPA_ADMIN_PORT)" ]; then \
		echo "✅ Admin Panel started on port $(WEBSPA_ADMIN_PORT)"; \
	else \
		echo "❌ Admin Panel did not bind to expected port $(WEBSPA_ADMIN_PORT)"; \
		tail -20 $(WEBSPA_ADMIN_DIR)/webspa-admin.log 2>/dev/null || true; \
		exit 1; \
	fi

# Service management targets
.PHONY: start
start: start-subscription start-notification start-acquisition-api
	@echo ""
	@echo "🚀 All services started! (see ports above)"

.PHONY: start-all
start-all: start-subscription-external start-subscription start-billing start-notification start-acquisition-api
	@echo ""
	@echo "🚀 All services started! (see ports above)"

.PHONY: start-subscription-external
start-subscription-external:
	@echo "🚀 Starting Subscription External Service..."
	@(cd $(SUBSCRIPTION_EXTERNAL_DIR); nohup ./subscription-external > subscription-external.log 2>&1 & echo $$! > subscription-external.pid)
	@echo "✅ Subscription External Service started on port $(SUBSCRIPTION_EXTERNAL_PORT)"

.PHONY: start-subscription
start-subscription:
	@echo "📱 Starting Subscription Service..."
	@PORT=$(SUBSCRIPTION_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(SUBSCRIPTION_DIR); APP_APPLICATION_PORT=$$PORT nohup ./subscription > subscription.log 2>&1 & echo $$! > subscription.pid ); \
	sleep 2; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Subscription Service started on port $$PORT"; \
	else \
		echo "❌ Subscription Service failed to start (check $(SUBSCRIPTION_DIR)/subscription.log)"; \
		tail -5 $(SUBSCRIPTION_DIR)/subscription.log 2>/dev/null || true; \
	fi

.PHONY: start-billing
start-billing:
	@echo "💰 Starting Billing Service..."
	@(cd $(BILLING_DIR); nohup ./billing > billing.log 2>&1 & echo $$! > billing.pid)
	@echo "✅ Billing Service started on port $(BILLING_PORT)"

.PHONY: start-notification
start-notification:
	@echo "🔔 Starting Notification Service..."
	@PORT=$(NOTIFICATION_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(NOTIFICATION_DIR); APPLICATION_PORT=$$PORT nohup ./notification > notification.log 2>&1 & echo $$! > notification.pid ); \
	sleep 2; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Notification Service started on port $$PORT"; \
	else \
		echo "❌ Notification Service failed to start (check $(NOTIFICATION_DIR)/notification.log)"; \
		tail -5 $(NOTIFICATION_DIR)/notification.log 2>/dev/null || true; \
	fi

.PHONY: start-acquisition-api
start-acquisition-api:
	@echo "📲 Starting Acquisition API..."
	@# Check for DB password in env or .env file
	@if [ -z "$$APP_DATABASE_POSTGRESQL_PASSWORD" ]; then \
		if ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' $(ACQUISITION_API_DIR)/.env 2>/dev/null && \
		   ! grep -q '^APP_DATABASE_POSTGRESQL_PASSWORD=' .env 2>/dev/null; then \
			echo "❌ ERROR: APP_DATABASE_POSTGRESQL_PASSWORD is not set"; \
			echo "   Add it to $(ACQUISITION_API_DIR)/.env or export it:"; \
			echo "   echo 'APP_DATABASE_POSTGRESQL_PASSWORD=your_password' >> $(ACQUISITION_API_DIR)/.env"; \
			exit 1; \
		fi; \
	fi
	@PORT=$(ACQUISITION_API_PORT); \
	while ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; do \
		PORT=$$((PORT + 1)); \
	done; \
	( cd $(ACQUISITION_API_DIR); APP_APPLICATION_PORT=$$PORT nohup ./acquisition-api > acquisition-api.log 2>&1 & echo $$! > acquisition-api.pid ); \
	sleep 3; \
	if ss -ltn 2>/dev/null | grep -q ":$$PORT " || netstat -ltn 2>/dev/null | grep -q ":$$PORT "; then \
		echo "✅ Acquisition API started on port $$PORT"; \
	else \
		echo "❌ Acquisition API failed to start (check $(ACQUISITION_API_DIR)/acquisition-api.log)"; \
		tail -10 $(ACQUISITION_API_DIR)/acquisition-api.log 2>/dev/null || true; \
	fi

.PHONY: start-cadence-engine
start-cadence-engine:
	@echo "⏰ Starting Cadence Engine..."
	@(cd $(CADENCE_ENGINE_DIR); nohup ./cadence-engine > cadence-engine.log 2>&1 & echo $$! > cadence-engine.pid)
	@echo "✅ Cadence Engine started on port 8091"

# Stop services
.PHONY: stop
stop: stop-subscription stop-notification stop-acquisition-api stop-landing stop-admin
	@echo "🛑 All services stopped!"

.PHONY: stop-all
stop-all: stop-subscription-external stop-subscription stop-billing stop-notification stop-acquisition-api stop-cadence-engine stop-landing stop-admin
	@echo "🛑 All services stopped!"

.PHONY: stop-subscription-external
stop-subscription-external:
	@echo "🛑 Stopping Subscription External Service..."
	@if [ -f "$(SUBSCRIPTION_EXTERNAL_PID_FILE)" ]; then \
		PID=$$(cat "$(SUBSCRIPTION_EXTERNAL_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(SUBSCRIPTION_EXTERNAL_PID_FILE)"; \
	fi
	@rm -f subscription-external.pid
	@pkill -f "[s]ervices/subscription-external/subscription-external$$" 2>/dev/null || true
	@pkill -f "./subscription-external$$" 2>/dev/null || true
	@echo "✅ Subscription External Service stopped"

.PHONY: stop-subscription
stop-subscription:
	@echo "🛑 Stopping Subscription Service..."
	@if [ -f "$(SUBSCRIPTION_PID_FILE)" ]; then \
		PID=$$(cat "$(SUBSCRIPTION_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(SUBSCRIPTION_PID_FILE)"; \
	fi
	@rm -f subscription.pid
	@pkill -f "[s]ervices/subscription-partner/subscription$$" 2>/dev/null || true
	@pkill -f "./subscription$$" 2>/dev/null || true
	@echo "✅ Subscription Service stopped"

.PHONY: stop-billing
stop-billing:
	@echo "🛑 Stopping Billing Service..."
	@if [ -f "$(BILLING_PID_FILE)" ]; then \
		PID=$$(cat "$(BILLING_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(BILLING_PID_FILE)"; \
	fi
	@rm -f billing.pid
	@pkill -f "[s]ervices/billing/billing$$" 2>/dev/null || true
	@pkill -f "./billing$$" 2>/dev/null || true
	@echo "✅ Billing Service stopped"

.PHONY: stop-notification
stop-notification:
	@echo "🛑 Stopping Notification Service..."
	@if [ -f "$(NOTIFICATION_PID_FILE)" ]; then \
		PID=$$(cat "$(NOTIFICATION_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(NOTIFICATION_PID_FILE)"; \
	fi
	@rm -f notification.pid
	@pkill -f "[s]ervices/notification/notification$$" 2>/dev/null || true
	@pkill -f "./notification$$" 2>/dev/null || true
	@echo "✅ Notification Service stopped"

.PHONY: stop-acquisition-api
stop-acquisition-api:
	@echo "🛑 Stopping Acquisition API..."
	@if [ -f "$(ACQUISITION_API_PID_FILE)" ]; then \
		PID=$$(cat "$(ACQUISITION_API_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(ACQUISITION_API_PID_FILE)"; \
	fi
	@rm -f acquisition-api.pid
	@pkill -f "[s]ervices/acquisition-api/acquisition-api$$" 2>/dev/null || true
	@pkill -f "./acquisition-api$$" 2>/dev/null || true
	@echo "✅ Acquisition API stopped"

.PHONY: stop-cadence-engine
stop-cadence-engine:
	@echo "🛑 Stopping Cadence Engine..."
	@if [ -f "$(CADENCE_ENGINE_PID_FILE)" ]; then \
		PID=$$(cat "$(CADENCE_ENGINE_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(CADENCE_ENGINE_PID_FILE)"; \
	fi
	@rm -f cadence-engine.pid
	@pkill -f "[s]ervices/cadence-engine/cadence-engine$$" 2>/dev/null || true
	@pkill -f "./cadence-engine$$" 2>/dev/null || true
	@echo "✅ Cadence Engine stopped"

.PHONY: stop-landing
stop-landing:
	@echo "🛑 Stopping Landing Web..."
	@if [ -f "$(LANDING_WEB_PID_FILE)" ]; then \
		PID=$$(cat "$(LANDING_WEB_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(LANDING_WEB_PID_FILE)"; \
	fi
	@rm -f landing-web.pid
	@pkill -f "[s]ervices/landing-web/node_modules/.bin/next dev" 2>/dev/null || true
	@echo "✅ Landing Web stopped"

.PHONY: stop-admin
stop-admin:
	@echo "🛑 Stopping Admin Panel..."
	@if [ -f "$(WEBSPA_ADMIN_PID_FILE)" ]; then \
		PID=$$(cat "$(WEBSPA_ADMIN_PID_FILE)" 2>/dev/null); \
		if [ -n "$$PID" ] && kill -0 "$$PID" 2>/dev/null; then \
			for C in $$(pgrep -P "$$PID" 2>/dev/null); do \
				pkill -TERM -P "$$C" 2>/dev/null || true; \
				kill "$$C" 2>/dev/null || true; \
			done; \
			pkill -TERM -P "$$PID" 2>/dev/null || true; \
			kill "$$PID" 2>/dev/null || true; \
		fi; \
		rm -f "$(WEBSPA_ADMIN_PID_FILE)"; \
	fi
	@rm -f webspa-admin.pid
	@pkill -f "[n]px ng serve --port $(WEBSPA_ADMIN_PORT)" 2>/dev/null || true
	@pkill -f "[n]ode ./node_modules/@angular/cli/bin/ng.js serve --port $(WEBSPA_ADMIN_PORT)" 2>/dev/null || true
	@pkill -f "[f]rontend/webspa-admin/node_modules/@esbuild/" 2>/dev/null || true
	@echo "✅ Admin Panel stopped"

# Restart services
.PHONY: restart
restart: stop start
	@echo "🔄 All services restarted!"

.PHONY: restart-subscription-external
restart-subscription-external: stop-subscription-external start-subscription-external
	@echo "🔄 Subscription External Service restarted!"

.PHONY: restart-subscription
restart-subscription: stop-subscription start-subscription
	@echo "🔄 Subscription Service restarted!"

.PHONY: restart-billing
restart-billing: stop-billing start-billing
	@echo "🔄 Billing Service restarted!"

.PHONY: restart-notification
restart-notification: stop-notification start-notification
	@echo "🔄 Notification Service restarted!"

# Build targets (local binaries)
.PHONY: build
build: build-local-subscription-external build-local-subscription build-local-notification build-local-notification-worker build-local-acquisition-api build-local-cadence-engine
	@echo "🔨 All services built successfully!"

.PHONY: build-all-local
build-all-local: build-local-subscription-external build-local-subscription build-local-billing build-local-notification build-local-notification-worker build-local-acquisition-api build-local-cadence-engine
	@echo "🔨 All services built successfully!"

.PHONY: build-local-subscription-external
build-local-subscription-external:
	@echo "🔨 Building Subscription External Service..."
	@cd $(SUBSCRIPTION_EXTERNAL_DIR) && go build -o subscription-external cmd/main.go
	@echo "✅ Subscription External Service built successfully"

.PHONY: build-local-subscription
build-local-subscription:
	@echo "🔨 Building Subscription Service..."
	@cd $(SUBSCRIPTION_DIR) && go build -o subscription cmd/main.go
	@echo "✅ Subscription Service built successfully"

.PHONY: build-local-billing
build-local-billing:
	@echo "🔨 Building Billing Service..."
	@cd $(BILLING_DIR) && go build -o billing cmd/main.go
	@echo "✅ Billing Service built successfully"

.PHONY: build-local-notification
build-local-notification:
	@echo "🔨 Building Notification Service..."
	@cd $(NOTIFICATION_DIR) && go build -o notification cmd/main.go
	@echo "✅ Notification Service built successfully"

.PHONY: build-local-notification-worker
build-local-notification-worker:
	@echo "🔨 Building Notification Worker..."
	@cd $(NOTIFICATION_DIR) && go build -o notification-worker cmd/notification-worker/main.go
	@echo "✅ Notification Worker built successfully"

.PHONY: build-local-acquisition-api
build-local-acquisition-api:
	@echo "🔨 Building Acquisition API..."
	@cd $(ACQUISITION_API_DIR) && go build -o acquisition-api cmd/main.go
	@echo "✅ Acquisition API built successfully"

.PHONY: build-local-postback-dispatcher
build-local-postback-dispatcher:
	@echo "🔨 Building Postback Dispatcher..."
	@cd $(POSTBACK_DISPATCHER_DIR) && go build -o postback-dispatcher cmd/main.go
	@echo "✅ Postback Dispatcher built successfully"

.PHONY: build-local-cadence-engine
build-local-cadence-engine:
	@echo "🔨 Building Cadence Engine..."
	@cd $(CADENCE_ENGINE_DIR) && go build -o cadence-engine cmd/cadence-engine/main.go
	@echo "✅ Cadence Engine built successfully"

# Test targets
.PHONY: test
test: test-subscription-external test-subscription test-billing test-notification
	@echo "🧪 All tests completed!"

.PHONY: test-subscription-external
test-subscription-external:
	@echo "🧪 Testing Subscription External Service..."
	@cd $(SUBSCRIPTION_EXTERNAL_DIR) && go test -v ./... -cover
	@echo "✅ Subscription External Service tests completed"

.PHONY: test-subscription
test-subscription:
	@echo "🧪 Testing Subscription Service..."
	@cd $(SUBSCRIPTION_DIR) && go test -v ./... -cover
	@echo "✅ Subscription Service tests completed"

.PHONY: test-billing
test-billing:
	@echo "🧪 Testing Billing Service..."
	@cd $(BILLING_DIR) && go test -v ./... -cover
	@echo "✅ Billing Service tests completed"

.PHONY: test-notification
test-notification:
	@echo "🧪 Testing Notification Service..."
	@cd $(NOTIFICATION_DIR) && go test -v ./... -cover
	@echo "✅ Notification Service tests completed"

# Monitoring and health check targets
.PHONY: health
health: health-subscription-external health-subscription health-billing health-notification
	@echo "🏥 All service health checks completed!"

.PHONY: health-subscription-external
health-subscription-external:
	@echo "🏥 Checking Subscription External Service health..."
	@curl -s "http://localhost:$(SUBSCRIPTION_EXTERNAL_PORT)/api/v1/subscription-external/monitoring/health" | jq '.health.overall_status' || echo "❌ Service not responding"
	@echo "✅ Subscription External Service health check completed"

.PHONY: health-subscription
health-subscription:
	@echo "🏥 Checking Subscription Service health..."
	@curl -s "http://localhost:$(SUBSCRIPTION_PORT)/health" | jq '.status' || echo "❌ Service not responding"
	@echo "✅ Subscription Service health check completed"

.PHONY: health-billing
health-billing:
	@echo "🏥 Checking Billing Service health..."
	@curl -s "http://localhost:$(BILLING_PORT)/health" | jq '.status' || echo "❌ Service not responding"
	@echo "✅ Billing Service health check completed"

.PHONY: health-notification
health-notification:
	@echo "🏥 Checking Notification Service health..."
	@curl -s "http://localhost:$(NOTIFICATION_PORT)/health" | jq '.status' || echo "❌ Service not responding"
	@echo "✅ Notification Service health check completed"

# Database migration targets
.PHONY: migrate
migrate: migrate-subscription-external db-migrate-cadence
	@echo "🗄️ All database migrations completed!"

.PHONY: migrate-subscription-external
migrate-subscription-external:
	@echo "🗄️ Running Subscription External Service migrations..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/011_message_cadence_engine.sql
	@echo "✅ Subscription External Service migrations completed"

# Log targets
.PHONY: logs
logs: logs-subscription-external logs-subscription logs-billing logs-notification
	@echo "📋 All service logs displayed!"

.PHONY: logs-subscription-external
logs-subscription-external:
	@echo "📋 Subscription External Service logs:"
	@tail -f $(SUBSCRIPTION_EXTERNAL_DIR)/logs/*.log || echo "No log files found"

.PHONY: logs-subscription
logs-subscription:
	@echo "📋 Subscription Service logs:"
	@tail -f $(SUBSCRIPTION_DIR)/logs/*.log || echo "No log files found"

.PHONY: logs-billing
logs-billing:
	@echo "📋 Billing Service logs:"
	@tail -f $(BILLING_DIR)/logs/*.log || echo "No log files found"

.PHONY: logs-notification
logs-notification:
	@echo "📋 Notification Service logs:"
	@tail -f $(NOTIFICATION_DIR)/logs/*.log || echo "No log files found"

# Clean targets
.PHONY: clean
clean: clean-subscription-external clean-subscription clean-billing clean-notification
	@echo "🧹 All services cleaned!"

.PHONY: clean-subscription-external
clean-subscription-external:
	@echo "🧹 Cleaning Subscription External Service..."
	@cd $(SUBSCRIPTION_EXTERNAL_DIR) && rm -f subscription-external main
	@echo "✅ Subscription External Service cleaned"

.PHONY: clean-subscription
clean-subscription:
	@echo "🧹 Cleaning Subscription Service..."
	@cd $(SUBSCRIPTION_DIR) && rm -f subscription
	@echo "✅ Subscription Service cleaned"

.PHONY: clean-billing
clean-billing:
	@echo "🧹 Cleaning Billing Service..."
	@cd $(BILLING_DIR) && rm -f billing
	@echo "✅ Billing Service cleaned"

.PHONY: clean-notification
clean-notification:
	@echo "🧹 Cleaning Notification Service..."
	@cd $(NOTIFICATION_DIR) && rm -f notification
	@echo "✅ Notification Service cleaned"

# Development tools
.PHONY: tools
tools: init update_deps
	@echo "🛠️ Development tools installed!"

# Service status
.PHONY: status
status:
	@echo "📊 Service Status:"
	@# PID-first detection with repo-scoped fallbacks (includes legacy root PID files during transition).
	@state='🔴 Stopped'; \
	if [ -f "$(SUBSCRIPTION_EXTERNAL_PID_FILE)" ] && kill -0 $$(cat "$(SUBSCRIPTION_EXTERNAL_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "subscription-external.pid" ] && kill -0 $$(cat subscription-external.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[s]ervices/subscription-external/subscription-external$$' > /dev/null || pgrep -f '^\./subscription-external$$' > /dev/null; then state='🟢 Running'; fi; \
	echo "Subscription External: $$state"
	@state='🔴 Stopped'; \
	if [ -f "$(SUBSCRIPTION_PID_FILE)" ] && kill -0 $$(cat "$(SUBSCRIPTION_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "subscription.pid" ] && kill -0 $$(cat subscription.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[s]ervices/subscription-partner/subscription$$' > /dev/null || pgrep -f '^\./subscription$$' > /dev/null; then state='🟢 Running'; fi; \
	echo "Subscription: $$state"
	@state='🔴 Stopped'; \
	if [ -f "$(BILLING_PID_FILE)" ] && kill -0 $$(cat "$(BILLING_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "billing.pid" ] && kill -0 $$(cat billing.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[s]ervices/billing/billing$$' > /dev/null || pgrep -f '^\./billing$$' > /dev/null; then state='🟢 Running'; fi; \
	echo "Billing: $$state"
	@state='🔴 Stopped'; \
	if [ -f "$(NOTIFICATION_PID_FILE)" ] && kill -0 $$(cat "$(NOTIFICATION_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "notification.pid" ] && kill -0 $$(cat notification.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[s]ervices/notification/notification$$' > /dev/null || pgrep -f '^\./notification$$' > /dev/null; then state='🟢 Running'; fi; \
	echo "Notification: $$state"
	@state='🔴 Stopped'; \
	if [ -f "$(ACQUISITION_API_PID_FILE)" ] && kill -0 $$(cat "$(ACQUISITION_API_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "acquisition-api.pid" ] && kill -0 $$(cat acquisition-api.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[s]ervices/acquisition-api/acquisition-api$$' > /dev/null || pgrep -f '^\./acquisition-api$$' > /dev/null; then state='🟢 Running'; fi; \
	echo "Acquisition API: $$state"
	@state='🔴 Stopped'; \
	if [ -f "$(CADENCE_ENGINE_PID_FILE)" ] && kill -0 $$(cat "$(CADENCE_ENGINE_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "cadence-engine.pid" ] && kill -0 $$(cat cadence-engine.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[s]ervices/cadence-engine/cadence-engine$$' > /dev/null || pgrep -f '^\./cadence-engine$$' > /dev/null; then state='🟢 Running'; fi; \
	echo "Cadence Engine: $$state"
	@state='🔴 Stopped'; \
	if [ -f "$(LANDING_WEB_PID_FILE)" ] && kill -0 $$(cat "$(LANDING_WEB_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "landing-web.pid" ] && kill -0 $$(cat landing-web.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[s]ervices/landing-web/node_modules/.bin/next dev' > /dev/null; then state='🟢 Running'; fi; \
	echo "Landing Web: $$state"
	@state='🔴 Stopped'; \
	if [ -f "$(WEBSPA_ADMIN_PID_FILE)" ] && kill -0 $$(cat "$(WEBSPA_ADMIN_PID_FILE)" 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif [ -f "webspa-admin.pid" ] && kill -0 $$(cat webspa-admin.pid 2>/dev/null) 2>/dev/null; then state='🟢 Running'; \
	elif pgrep -f '[n]px ng serve --port $(WEBSPA_ADMIN_PORT)' > /dev/null; then state='🟢 Running'; \
	elif pgrep -f '[n]ode ./node_modules/@angular/cli/bin/ng.js serve --port $(WEBSPA_ADMIN_PORT)' > /dev/null; then state='🟢 Running'; \
	elif pgrep -f '[f]rontend/webspa-admin/node_modules/@esbuild/' > /dev/null; then state='🟢 Running'; fi; \
	echo "Admin Panel: $$state"

# Quick start for development
.PHONY: quick-start
quick-start: build-local-subscription-external start-subscription-external
	@echo "🚀 Subscription External Service (with Monitoring & Worker) started quickly!"
	@echo "📊 Dashboard: http://localhost:$(SUBSCRIPTION_EXTERNAL_PORT)"
	@echo "🔍 API Docs: http://localhost:$(SUBSCRIPTION_EXTERNAL_PORT)/docs"

# Legacy targets (keeping for backward compatibility)
.PHONY: init
init:
	@if ! [ -f "$(GOPATH)/bin/protoc-gen-go" ]; then \
		echo "Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
	else \
		echo "protoc-gen-go already installed"; \
	fi

	@if ! [ -f "$(GOPATH)/bin/protoc-gen-go-grpc" ]; then \
		echo "Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	else \
		echo "protoc-gen-go-grpc already installed"; \
	fi

	@if ! [ -f "$(GOPATH)/bin/protoc-gen-micro" ]; then \
		echo "Installing protoc-gen-micro..."; \
		go install github.com/micro/micro/v3/cmd/protoc-gen-micro@latest; \
	else \
		echo "protoc-gen-micro already installed"; \
	fi

	@if ! [ -f "$(GOPATH)/bin/protoc-gen-openapi" ]; then \
		echo "Installing protoc-gen-openapi..."; \
		go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest; \
	else \
		echo "protoc-gen-openapi already installed"; \
	fi

.PHONY: update_deps
update_deps:
	go mod verify
	go mod tidy

.PHONY: proto
proto:
	protoc --openapi_out=. --proto_path=. --micro_out=. --go_out=. --go-grpc_out=. services/**/proto/*.proto

docs:
	protoc --openapi_out=. --proto_path=. --micro_out=. --go_out=. proto/tenant.proto
	@redoc-cli bundle api-sm.json --options.theme.colors.primary.main=orange

generate-docs:
	@swag init --generalInfo ./cmd/main.go --output ./docs

# Legacy build targets
.PHONY: build-legacy
build-legacy:
	cd ./services/tenants/ && $(MAKE) -f MakeFile build
	cd ./services/subscriptions/ && $(MAKE) -f MakeFile build
	cd ./services/workflows/ && $(MAKE) -f MakeFile build_worker
	cd ./services/workflows/ && $(MAKE) -f MakeFile build_starter

# =============================================================================
# Docker Build Targets
# =============================================================================

.PHONY: docker-build-krakend
docker-build-krakend:
	@echo "🐳 Building KrakenD image..."
	docker build -t $(KRAKEND_IMAGE):$(VERSION) $(KRAKEND_DIR)
	@echo "✅ KrakenD image built: $(KRAKEND_IMAGE):$(VERSION)"

.PHONY: krakend-query-forwarding-check krakend-check krakend-check-do
krakend-query-forwarding-check:
	@echo "🔎 Checking KrakenD list query forwarding..."
	@python3 scripts/check-krakend-query-forwarding.py

krakend-check: krakend-query-forwarding-check
	@echo "🔎 Validating KrakenD flexible config (local settings)..."
	docker run --rm \
		-v "$(PWD)/krakend:/etc/krakend" \
		-e FC_ENABLE=1 \
		-e FC_SETTINGS="/etc/krakend/config/settings" \
		-e FC_PARTIALS="/etc/krakend/config/partials" \
		-e FC_TEMPLATES="/etc/krakend/config/templates" \
		docker.io/library/krakend:latest \
		krakend check -t -c "/etc/krakend/config/krakend.tmpl"

krakend-check-do: krakend-query-forwarding-check
	@echo "🔎 Validating KrakenD flexible config (DO settings)..."
	docker run --rm \
		-v "$(PWD)/krakend:/etc/krakend" \
		-e FC_ENABLE=1 \
		-e FC_SETTINGS="/etc/krakend/config/settings/do" \
		-e FC_PARTIALS="/etc/krakend/config/partials" \
		-e FC_TEMPLATES="/etc/krakend/config/templates" \
		docker.io/library/krakend:latest \
		krakend check -t -c "/etc/krakend/config/krakend.tmpl"

.PHONY: krakend-debug-do
krakend-debug-do:
	@echo "🧾 Dumping rendered KrakenD config (DO settings)..."
	docker run --rm \
		-v "$(PWD)/krakend:/etc/krakend" \
		-e FC_ENABLE=1 \
		-e FC_SETTINGS="/etc/krakend/config/settings/do" \
		-e FC_PARTIALS="/etc/krakend/config/partials" \
		-e FC_TEMPLATES="/etc/krakend/config/templates" \
		docker.io/library/krakend:latest \
		krakend check -d -c "/etc/krakend/config/krakend.tmpl"

.PHONY: docker-build-subscription-partner
docker-build-subscription-partner:
	@echo "🐳 Building Subscription Partner image..."
	docker build -t $(SUBSCRIPTION_PARTNER_IMAGE):$(VERSION) $(SUBSCRIPTION_DIR)
	@echo "✅ Subscription Partner image built: $(SUBSCRIPTION_PARTNER_IMAGE):$(VERSION)"

.PHONY: docker-build-subscription-external
docker-build-subscription-external:
	@echo "🐳 Building Subscription External image..."
	@echo "🔄 Refreshing vendored dependencies for Subscription External..."
	@(cd $(SUBSCRIPTION_EXTERNAL_DIR) && go mod vendor)
	docker build -t $(SUBSCRIPTION_EXTERNAL_IMAGE):$(VERSION) $(SUBSCRIPTION_EXTERNAL_DIR)
	@echo "✅ Subscription External image built: $(SUBSCRIPTION_EXTERNAL_IMAGE):$(VERSION)"

.PHONY: vendor-check
vendor-check:
	@echo "🔎 Checking vendor sync for services using -mod=vendor..."
	@./scripts/check-vendor-sync.sh

.PHONY: docker-build-billing
docker-build-billing:
	@echo "🐳 Building Billing image..."
	docker build -t $(BILLING_IMAGE):$(VERSION) $(BILLING_DIR)
	@echo "✅ Billing image built: $(BILLING_IMAGE):$(VERSION)"

.PHONY: docker-build-notification
docker-build-notification:
	@echo "🐳 Building Notification image..."
	docker build -t $(NOTIFICATION_IMAGE):$(VERSION) $(NOTIFICATION_DIR)
	@echo "✅ Notification image built: $(NOTIFICATION_IMAGE):$(VERSION)"

.PHONY: docker-build-acquisition-api
docker-build-acquisition-api:
	@echo "🐳 Building Acquisition API image..."
	docker build -t $(ACQUISITION_API_IMAGE):$(VERSION) $(ACQUISITION_API_DIR)
	@echo "✅ Acquisition API image built: $(ACQUISITION_API_IMAGE):$(VERSION)"

.PHONY: docker-build-postback-dispatcher
docker-build-postback-dispatcher:
	@echo "🐳 Building Postback Dispatcher image..."
	docker build -t $(POSTBACK_DISPATCHER_IMAGE):$(VERSION) $(POSTBACK_DISPATCHER_DIR)
	@echo "✅ Postback Dispatcher image built: $(POSTBACK_DISPATCHER_IMAGE):$(VERSION)"

.PHONY: docker-build-landing-web
docker-build-landing-web:
	@echo "🐳 Building Landing Web image..."
	docker build -t $(LANDING_WEB_IMAGE):$(VERSION) $(LANDING_WEB_DIR)
	@echo "✅ Landing Web image built: $(LANDING_WEB_IMAGE):$(VERSION)"

.PHONY: docker-build-webspa-admin
docker-build-webspa-admin:
	@echo "🐳 Building WebSPA Admin image..."
	docker build -t $(WEBSPA_ADMIN_IMAGE):$(VERSION) $(WEBSPA_ADMIN_DIR)
	@echo "✅ WebSPA Admin image built: $(WEBSPA_ADMIN_IMAGE):$(VERSION)"

.PHONY: docker-build-cadence-engine
docker-build-cadence-engine:
	@echo "🐳 Building Cadence Engine image..."
	docker build -t $(CADENCE_ENGINE_IMAGE):$(VERSION) $(CADENCE_ENGINE_DIR)
	@echo "✅ Cadence Engine image built: $(CADENCE_ENGINE_IMAGE):$(VERSION)"

# Build core services (commonly deployed together)
.PHONY: docker-build-core
docker-build-core: docker-build-subscription-partner docker-build-subscription-external docker-build-notification docker-build-acquisition-api docker-build-cadence-engine
	@echo "🐳 Core service images built successfully!"

# Build all Docker images
.PHONY: docker-build-all
docker-build-all: docker-build-krakend docker-build-subscription-partner docker-build-subscription-external docker-build-notification docker-build-acquisition-api docker-build-postback-dispatcher docker-build-landing-web docker-build-webspa-admin docker-build-cadence-engine
	@echo "🐳 All Docker images built successfully!"
	@echo ""
	@echo "Built images:"
	@echo "  - $(KRAKEND_IMAGE):$(VERSION)"
	@echo "  - $(SUBSCRIPTION_PARTNER_IMAGE):$(VERSION)"
	@echo "  - $(SUBSCRIPTION_EXTERNAL_IMAGE):$(VERSION)"
	@echo "  - $(NOTIFICATION_IMAGE):$(VERSION)"
	@echo "  - $(ACQUISITION_API_IMAGE):$(VERSION)"
	@echo "  - $(POSTBACK_DISPATCHER_IMAGE):$(VERSION)"
	@echo "  - $(LANDING_WEB_IMAGE):$(VERSION)"
	@echo "  - $(WEBSPA_ADMIN_IMAGE):$(VERSION)"
	@echo "  - $(CADENCE_ENGINE_IMAGE):$(VERSION)"

# Legacy aliases (backward compatibility)
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

# =============================================================================
# Docker Push Targets
# =============================================================================

# Keep local build tags short to match compose/k8s manifests, but push fully
# qualified refs so Podman does not rely on implicit Docker Hub resolution.
define push_image
	@docker tag $(1):$(VERSION) $(DOCKER_PUSH_REGISTRY)/$(1):$(VERSION)
	@attempt=1; \
	while [ $$attempt -le $(PUSH_RETRIES) ]; do \
		if docker push $(DOCKER_PUSH_REGISTRY)/$(1):$(VERSION); then \
			break; \
		fi; \
		if [ $$attempt -eq $(PUSH_RETRIES) ]; then \
			echo "❌ Push failed after $(PUSH_RETRIES) attempts: $(DOCKER_PUSH_REGISTRY)/$(1):$(VERSION)"; \
			exit 1; \
		fi; \
		echo "⚠️ Push attempt $$attempt failed for $(DOCKER_PUSH_REGISTRY)/$(1):$(VERSION); retrying in $(PUSH_RETRY_DELAY_SECONDS)s..."; \
		attempt=$$((attempt + 1)); \
		sleep $(PUSH_RETRY_DELAY_SECONDS); \
	done
endef

.PHONY: docker-login-check
docker-login-check:
	@logged_in_user="$$(docker login --get-login $(DOCKER_PUSH_REGISTRY) 2>/dev/null || true)"; \
	if [ -z "$$logged_in_user" ]; then \
		echo "❌ Not logged into $(DOCKER_PUSH_REGISTRY). Run: docker login $(DOCKER_PUSH_REGISTRY)"; \
		echo "   If you need a different namespace, override DOCKER_USER, for example:"; \
		echo "   make DOCKER_USER=<your-dockerhub-namespace> docker-push-subscription-external"; \
		exit 1; \
	fi; \
	echo "🔐 Logged into $(DOCKER_PUSH_REGISTRY) as $$logged_in_user"

.PHONY: docker-push-krakend
docker-push-krakend: docker-login-check
	@echo "📤 Pushing KrakenD image..."
	$(call push_image,$(KRAKEND_IMAGE))
	@echo "✅ KrakenD image pushed: $(DOCKER_PUSH_REGISTRY)/$(KRAKEND_IMAGE):$(VERSION)"

.PHONY: docker-push-subscription-partner
docker-push-subscription-partner: docker-login-check
	@echo "📤 Pushing Subscription Partner image..."
	$(call push_image,$(SUBSCRIPTION_PARTNER_IMAGE))
	@echo "✅ Subscription Partner image pushed: $(DOCKER_PUSH_REGISTRY)/$(SUBSCRIPTION_PARTNER_IMAGE):$(VERSION)"

.PHONY: docker-push-subscription-external
docker-push-subscription-external: docker-login-check
	@echo "📤 Pushing Subscription External image..."
	$(call push_image,$(SUBSCRIPTION_EXTERNAL_IMAGE))
	@echo "✅ Subscription External image pushed: $(DOCKER_PUSH_REGISTRY)/$(SUBSCRIPTION_EXTERNAL_IMAGE):$(VERSION)"

.PHONY: docker-push-billing
docker-push-billing: docker-login-check
	@echo "📤 Pushing Billing image..."
	$(call push_image,$(BILLING_IMAGE))
	@echo "✅ Billing image pushed: $(DOCKER_PUSH_REGISTRY)/$(BILLING_IMAGE):$(VERSION)"

.PHONY: docker-push-notification
docker-push-notification: docker-login-check
	@echo "📤 Pushing Notification image..."
	$(call push_image,$(NOTIFICATION_IMAGE))
	@echo "✅ Notification image pushed: $(DOCKER_PUSH_REGISTRY)/$(NOTIFICATION_IMAGE):$(VERSION)"

.PHONY: docker-push-acquisition-api
docker-push-acquisition-api: docker-login-check
	@echo "📤 Pushing Acquisition API image..."
	$(call push_image,$(ACQUISITION_API_IMAGE))
	@echo "✅ Acquisition API image pushed: $(DOCKER_PUSH_REGISTRY)/$(ACQUISITION_API_IMAGE):$(VERSION)"

.PHONY: docker-push-postback-dispatcher
docker-push-postback-dispatcher: docker-login-check
	@echo "📤 Pushing Postback Dispatcher image..."
	$(call push_image,$(POSTBACK_DISPATCHER_IMAGE))
	@echo "✅ Postback Dispatcher image pushed: $(DOCKER_PUSH_REGISTRY)/$(POSTBACK_DISPATCHER_IMAGE):$(VERSION)"

.PHONY: docker-push-landing-web
docker-push-landing-web: docker-login-check
	@echo "📤 Pushing Landing Web image..."
	$(call push_image,$(LANDING_WEB_IMAGE))
	@echo "✅ Landing Web image pushed: $(DOCKER_PUSH_REGISTRY)/$(LANDING_WEB_IMAGE):$(VERSION)"

.PHONY: docker-push-webspa-admin
docker-push-webspa-admin: docker-login-check
	@echo "📤 Pushing WebSPA Admin image..."
	$(call push_image,$(WEBSPA_ADMIN_IMAGE))
	@echo "✅ WebSPA Admin image pushed: $(DOCKER_PUSH_REGISTRY)/$(WEBSPA_ADMIN_IMAGE):$(VERSION)"

.PHONY: docker-push-cadence-engine
docker-push-cadence-engine: docker-login-check
	@echo "📤 Pushing Cadence Engine image..."
	$(call push_image,$(CADENCE_ENGINE_IMAGE))
	@echo "✅ Cadence Engine image pushed: $(DOCKER_PUSH_REGISTRY)/$(CADENCE_ENGINE_IMAGE):$(VERSION)"

# Push core services
.PHONY: docker-push-core
docker-push-core: docker-push-subscription-partner docker-push-subscription-external docker-push-notification docker-push-acquisition-api docker-push-cadence-engine
	@echo "📤 Core service images pushed successfully!"

# Push all Docker images
.PHONY: docker-push-all
docker-push-all: docker-push-krakend docker-push-subscription-partner docker-push-subscription-external docker-push-notification docker-push-acquisition-api docker-push-postback-dispatcher docker-push-landing-web docker-push-webspa-admin docker-push-cadence-engine
	@echo "📤 All Docker images pushed successfully!"
	@echo ""
	@echo "Pushed images:"
	@echo "  - $(KRAKEND_IMAGE):$(VERSION)"
	@echo "  - $(SUBSCRIPTION_PARTNER_IMAGE):$(VERSION)"
	@echo "  - $(SUBSCRIPTION_EXTERNAL_IMAGE):$(VERSION)"
	@echo "  - $(NOTIFICATION_IMAGE):$(VERSION)"
	@echo "  - $(ACQUISITION_API_IMAGE):$(VERSION)"
	@echo "  - $(POSTBACK_DISPATCHER_IMAGE):$(VERSION)"
	@echo "  - $(LANDING_WEB_IMAGE):$(VERSION)"
	@echo "  - $(WEBSPA_ADMIN_IMAGE):$(VERSION)"
	@echo "  - $(CADENCE_ENGINE_IMAGE):$(VERSION)"

# Legacy aliases (backward compatibility)
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

# =============================================================================
# Docker Release Targets (Build + Push)
# =============================================================================

.PHONY: docker-release-krakend
docker-release-krakend: docker-build-krakend docker-push-krakend
	@echo "🚀 KrakenD released: $(KRAKEND_IMAGE):$(VERSION)"

.PHONY: docker-release-subscription-partner
docker-release-subscription-partner: docker-build-subscription-partner docker-push-subscription-partner
	@echo "🚀 Subscription Partner released: $(SUBSCRIPTION_PARTNER_IMAGE):$(VERSION)"

.PHONY: docker-release-subscription-external
docker-release-subscription-external: docker-build-subscription-external docker-push-subscription-external
	@echo "🚀 Subscription External released: $(SUBSCRIPTION_EXTERNAL_IMAGE):$(VERSION)"

.PHONY: docker-release-billing
docker-release-billing: docker-build-billing docker-push-billing
	@echo "🚀 Billing released: $(BILLING_IMAGE):$(VERSION)"

.PHONY: docker-release-notification
docker-release-notification: docker-build-notification docker-push-notification
	@echo "🚀 Notification released: $(NOTIFICATION_IMAGE):$(VERSION)"

.PHONY: docker-release-acquisition-api
docker-release-acquisition-api: docker-build-acquisition-api docker-push-acquisition-api
	@echo "🚀 Acquisition API released: $(ACQUISITION_API_IMAGE):$(VERSION)"

.PHONY: docker-release-postback-dispatcher
docker-release-postback-dispatcher: docker-build-postback-dispatcher docker-push-postback-dispatcher
	@echo "🚀 Postback Dispatcher released: $(POSTBACK_DISPATCHER_IMAGE):$(VERSION)"

.PHONY: docker-release-landing-web
docker-release-landing-web: docker-build-landing-web docker-push-landing-web
	@echo "🚀 Landing Web released: $(LANDING_WEB_IMAGE):$(VERSION)"

.PHONY: docker-release-webspa-admin
docker-release-webspa-admin: docker-build-webspa-admin docker-push-webspa-admin
	@echo "🚀 WebSPA Admin released: $(WEBSPA_ADMIN_IMAGE):$(VERSION)"

.PHONY: docker-release-cadence-engine
docker-release-cadence-engine: docker-build-cadence-engine docker-push-cadence-engine
	@echo "🚀 Cadence Engine released: $(CADENCE_ENGINE_IMAGE):$(VERSION)"

# Release core services
.PHONY: docker-release-core
docker-release-core: docker-build-core docker-push-core
	@echo "🚀 Core services released successfully!"

# Release all services
.PHONY: docker-release-all
docker-release-all: docker-build-all docker-push-all
	@echo "🚀 All services released successfully!"

# =============================================================================
# Deploy Targets (Build + Push + Remote Deploy)
# =============================================================================
# Runs build, push, then SSH into the server to pull & restart the service.
# Override DEPLOY_SSH_HOST to change the target server.

DEPLOY_SSH_HOST ?= do-sa-user
DEPLOY_SCRIPT ?= ~/services/nouveauricheglobalgroup/deploy.sh
KRAKEND_REMOTE_CONFIG ?= /etc/krakend/config
KRAKEND_LOCAL_CONFIG = krakend/config

define deploy_service
	@echo "🚀 Deploying $(1) to $(DEPLOY_SSH_HOST)..."
	ssh $(DEPLOY_SSH_HOST) "$(DEPLOY_SCRIPT) $(1)"
	@echo "✅ $(1) deployed successfully!"
endef

# KraKend config: validate locally, sync to server, restart systemd service
.PHONY: krakend-sync
krakend-sync: krakend-check-do
	@echo "🚀 Syncing KraKend config to $(DEPLOY_SSH_HOST)..."
	rsync -av --delete \
		$(KRAKEND_LOCAL_CONFIG)/templates/ \
		$(DEPLOY_SSH_HOST):$(KRAKEND_REMOTE_CONFIG)/templates/
	rsync -av --delete \
		$(KRAKEND_LOCAL_CONFIG)/partials/ \
		$(DEPLOY_SSH_HOST):$(KRAKEND_REMOTE_CONFIG)/partials/
	rsync -av \
		$(KRAKEND_LOCAL_CONFIG)/settings/ \
		$(DEPLOY_SSH_HOST):$(KRAKEND_REMOTE_CONFIG)/settings/
	rsync -av \
		$(KRAKEND_LOCAL_CONFIG)/krakend.tmpl \
		$(DEPLOY_SSH_HOST):$(KRAKEND_REMOTE_CONFIG)/krakend.tmpl
	@echo "✅ Config synced. Restarting KraKend..."
	ssh $(DEPLOY_SSH_HOST) "$(DEPLOY_SCRIPT) --krakend"

.PHONY: deploy-krakend
deploy-krakend: krakend-sync

.PHONY: deploy-subscription-partner
deploy-subscription-partner: docker-release-subscription-partner
	$(call deploy_service,subscription-partner)

.PHONY: deploy-subscription-external
deploy-subscription-external: docker-release-subscription-external
	$(call deploy_service,subscription-external)

.PHONY: deploy-billing
deploy-billing: docker-release-billing
	$(call deploy_service,billing)

.PHONY: deploy-notification
deploy-notification: docker-release-notification
	$(call deploy_service,notification)

.PHONY: deploy-acquisition-api
deploy-acquisition-api: docker-release-acquisition-api
	$(call deploy_service,acquisition-api)

.PHONY: deploy-postback-dispatcher
deploy-postback-dispatcher: docker-release-postback-dispatcher
	$(call deploy_service,postback-dispatcher)

.PHONY: deploy-landing-web
deploy-landing-web: docker-release-landing-web
	$(call deploy_service,landing-web)

.PHONY: deploy-webspa-admin
deploy-webspa-admin: docker-release-webspa-admin
	$(call deploy_service,webspa-admin)

.PHONY: deploy-cadence-engine
deploy-cadence-engine: docker-release-cadence-engine
	$(call deploy_service,cadence-engine)

.PHONY: deploy-core
deploy-core: docker-release-core
	$(call deploy_service,subscription-partner subscription-external notification acquisition-api cadence-engine)

.PHONY: deploy-all
deploy-all: docker-release-all krakend-sync
	@echo "🚀 Deploying all docker services to $(DEPLOY_SSH_HOST)..."
	ssh $(DEPLOY_SSH_HOST) "$(DEPLOY_SCRIPT)"
	@echo "✅ All services deployed successfully!"

# Legacy aliases (backward compatibility)
release-subscription: docker-release-subscription-partner
release-billing: docker-release-billing
release-notification: docker-release-notification
release-all: docker-release-all

# Clean up dangling Docker images
clean-docker:
	docker rmi $$(docker images -f "dangling=true" -q) || true
	@echo "Cleaned up dangling Docker images."

.PHONY: test-legacy
test-legacy:
	go test -v ./... -cover

.PHONY: docker
docker:
	cd ./services/tenants/ && $(MAKE) -f MakeFile docker
	cd ./services/subscriptions/ && $(MAKE) -f MakeFile docker
	cd ./services/workflows/ && $(MAKE) -f MakeFile docker_worker
	cd ./services/workflows/ && $(MAKE) -f MakeFile docker_starter

docker_clean:
	@echo "Prune containers"
	docker rm -v $(docker ps --filter status=exited -q)

docker_clean_images:
	@echo "Cleaning dangling images..."
	@docker images -f "dangling=true" -q  | xargs docker rmi
	@echo "Removing microservice images..."
	@docker images -f "label=org.label-schema.vendor=sumo" -q | xargs docker rmi
	@echo "Pruneing images..."
	@docker image prune -f

docker_push:
	@echo "Piblishing images with VCS_REF=$(shell git rev-parse --short HEAD)"
	@docker images -f "label=org.label-schema.vcs-ref=$(shell git rev-parse --short HEAD)" --format {{.Repository}}:{{.Tag}} | \
	while read -r image; do \
		echo Now pushing $$image; \
		echo Now pushing $$image; \
		docker push $$image; \
	done;

# =============================================================================
# Docker Compose Targets
# =============================================================================

.PHONY: package
package:
	docker-compose down --remove-orphans -v
	docker-compose build

.PHONY: compose-up
compose-up: ## Run docker-compose (development)
	docker compose up --build -d && docker compose logs -f

.PHONY: compose-down
compose-down: ## Stop docker-compose
	docker compose down --remove-orphans

.PHONY: compose-prod-up
compose-prod-up: ## Start production stack (docker-compose.prod.yml)
	docker compose -f docker-compose.prod.yml up -d
	@echo "🚀 Production stack started!"
	@echo "📊 Services:"
	@echo "  - KrakenD Gateway: http://localhost:8080"
	@echo "  - Subscription Partner: http://localhost:8081"
	@echo "  - Notification: http://localhost:8082"
	@echo "  - Subscription External: http://localhost:8083"
	@echo "  - Acquisition API: http://localhost:8084"
	@echo "  - Cadence Engine: http://localhost:8091"
	@echo "  - Landing Web: http://localhost:3000"

.PHONY: compose-prod-down
compose-prod-down: ## Stop production stack
	docker compose -f docker-compose.prod.yml down
	@echo "🛑 Production stack stopped."

.PHONY: compose-prod-logs
compose-prod-logs: ## View production stack logs
	docker compose -f docker-compose.prod.yml logs -f

.PHONY: compose-do-up
compose-do-up: ## Start DigitalOcean droplet stack (docker-compose.prod-do.yml)
	docker compose -f docker-compose.prod-do.yml up -d
	@echo "🚀 DigitalOcean stack started!"

.PHONY: compose-do-down
compose-do-down: ## Stop DigitalOcean droplet stack
	docker compose -f docker-compose.prod-do.yml down
	@echo "🛑 DigitalOcean stack stopped."

.PHONY: compose-do-logs
compose-do-logs: ## View DigitalOcean stack logs
	docker compose -f docker-compose.prod-do.yml logs -f

.PHONY: compose-do-pull
compose-do-pull: ## Pull latest images for DigitalOcean stack
	docker compose -f docker-compose.prod-do.yml pull
	@echo "📥 All images pulled!"

.PHONY: compose-do-restart
compose-do-restart: compose-do-down compose-do-pull compose-do-up ## Restart DigitalOcean stack with latest images
	@echo "🔄 DigitalOcean stack restarted with latest images!"

# List all running containers
.PHONY: docker-ps
docker-ps: ## List running Docker containers
	@docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# Show Docker images
.PHONY: docker-images
docker-images: ## List all service Docker images
	@echo "📦 Service Docker Images:"
	@docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}" | grep -E "$(DOCKER_USER)|REPOSITORY"

.PHONY: git
git:
ifeq ($(VERSION),)
     VERSION:=$(shell git describe --tags --abbrev=0 | awk -F .   '{OFS="."; $$NF+=1; print}')
endif

# =============================================================================
# Database Commands
# =============================================================================

# Database connection settings (can be overridden via environment)
DB_HOST ?= 139.59.135.253
DB_PORT ?= 5432
DB_USER ?= sm_admin
DB_NAME ?= subscription_manager

.PHONY: db-connect
db-connect: ## Connect to remote PostgreSQL database
	@echo "🔌 Connecting to PostgreSQL at $(DB_HOST):$(DB_PORT)..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME)

.PHONY: db-exec-sql
db-exec-sql: ## Execute SQL file against remote database (usage: make db-exec-sql FILE=path/to/file.sql)
	@if [ -z "$(FILE)" ]; then \
		echo "❌ Error: FILE parameter required. Usage: make db-exec-sql FILE=path/to/file.sql"; \
		exit 1; \
	fi
	@echo "📄 Executing $(FILE) against $(DB_HOST):$(DB_PORT)/$(DB_NAME)..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f $(FILE)

.PHONY: db-migrate-campaigns
db-migrate-campaigns: ## Run all campaign-related migrations
	@echo "🔄 Running campaign migrations..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/006_web_acquisition_campaigns.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/007_add_charge_tracking_columns.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/008_landing_events.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/009_campaign_landing_page_urls.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/010_outbound_clicks.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/012_campaign_tracking_config.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/013_he_tracking.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/014_campaign_lp_copy.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/add_admin_management_tables.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/add_acquisition_transaction_offer_context.sql
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/update_ghana_lp_copy_msisdn_format.sql
	@echo "✅ Campaign migrations complete!"

.PHONY: db-migrate-cadence
db-migrate-cadence: ## Run cadence engine migration
	@echo "🔄 Running cadence engine migration..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/subscription-external/migrations/011_message_cadence_engine.sql
	@echo "✅ Cadence migration complete!"

.PHONY: db-migrate-tenant-platform-dry-run
db-migrate-tenant-platform-dry-run: ## Dry-run the canonical nrg tenant backfill and readiness checks
	@echo "🔎 Dry-running tenant platform migration..."
	@bash scripts/db-migrate-tenant-platform.sh --dry-run

.PHONY: db-migrate-tenant-platform
db-migrate-tenant-platform: ## Backfill tenantless rows into the canonical nrg tenant
	@echo "🗄️ Running tenant platform migration..."
	@bash scripts/db-migrate-tenant-platform.sh --apply
	@echo "✅ Tenant platform migration complete!"

.PHONY: db-create-mobplus-campaign
db-create-mobplus-campaign: ## Create a new Mobplus campaign with click_id support
	@echo "🚀 Creating Mobplus campaign..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/create_mobplus_campaign.sql
	@echo "✅ Mobplus campaign created! Check output above for click_id and URLs."

.PHONY: db-configure-level23-campaign
db-configure-level23-campaign: ## Configure Level23 campaign postback + share link mapping
	@echo "🚀 Configuring Level23 campaign..."
	@if command -v psql >/dev/null 2>&1; then \
		PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/configure_level23_campaign.sql; \
	else \
		echo "ℹ️  psql not found locally, using dockerized postgres client..."; \
		docker run --rm \
			-e PGPASSWORD="$$DB_PASSWORD" \
			-v "$(CURDIR):/work" \
			-w /work \
			postgres:16 \
			psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/configure_level23_campaign.sql; \
	fi
	@echo "✅ Level23 campaign configured!"

.PHONY: db-generate-level23-share-info
db-generate-level23-share-info: ## Generate Level23 campaign sharing details
	@echo "📋 Generating Level23 share info..."
	@if command -v psql >/dev/null 2>&1; then \
		PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/generate_level23_share_info.sql; \
	else \
		echo "ℹ️  psql not found locally, using dockerized postgres client..."; \
		docker run --rm \
			-e PGPASSWORD="$$DB_PASSWORD" \
			-v "$(CURDIR):/work" \
			-w /work \
			postgres:16 \
			psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/generate_level23_share_info.sql; \
	fi

.PHONY: db-update-gh-lp-copy-msisdn-format
db-update-gh-lp-copy-msisdn-format: ## Update GH campaign lp_copy text for strict 9-digit MSISDN UX
	@echo "📝 Updating GH lp_copy msisdn text..."
	@if command -v psql >/dev/null 2>&1; then \
		PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/update_ghana_lp_copy_msisdn_format.sql; \
	else \
		echo "ℹ️  psql not found locally, using dockerized postgres client..."; \
		docker run --rm \
			-e PGPASSWORD="$$DB_PASSWORD" \
			-v "$(CURDIR):/work" \
			-w /work \
			postgres:16 \
			psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f services/acquisition-api/migrations/update_ghana_lp_copy_msisdn_format.sql; \
	fi

.PHONY: db-list-campaigns
db-list-campaigns: ## List all campaigns in the database
	@echo "📋 Listing campaigns..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -c "\
		SELECT id, slug, country, operator, offer_product_id, flow_type, price, billing_cycle, enabled, created_at \
		FROM campaigns ORDER BY created_at DESC;"

.PHONY: db-campaign-details
db-campaign-details: ## Show detailed campaign info (usage: make db-campaign-details SLUG=campaign-slug)
	@if [ -z "$(SLUG)" ]; then \
		echo "❌ Error: SLUG parameter required. Usage: make db-campaign-details SLUG=campaign-slug"; \
		exit 1; \
	fi
	@echo "📊 Campaign details for $(SLUG)..."
	@PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -c "\
		SELECT slug, country, operator, offer_product_id, partner_role_id, flow_type, price, billing_cycle, \
		       enabled, postback_rules, attribution_mapping, landing_page_urls \
		FROM campaigns WHERE slug = '$(SLUG)';"

.PHONY: db-generate-click-id
db-generate-click-id: ## Generate a new UUID click_id for testing
	@echo "🔑 Generating new click_id..."
	@CLICK_ID=$$(PGPASSWORD="$$DB_PASSWORD" psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -t -c "SELECT gen_random_uuid();"); \
	echo ""; \
	echo "============================================================"; \
	echo "New Click ID: $$CLICK_ID"; \
	echo "============================================================"; \
	echo ""; \
	echo "Use in landing page URL:"; \
	echo "  ?click_id=$$CLICK_ID"; \
	echo "  ?txid=$$CLICK_ID  (Mobplus format)"; \
	echo ""

# Help target
.PHONY: help
help:
	@echo "🚀 TimWe Subscription Services - Makefile Commands"
	@echo "================================================"
	@echo ""
	@echo "📊 Development Commands:"
	@echo "  make dev                    - Start core services in development mode"
	@echo "  make dev-all                - Start all services in development mode"
	@echo "  make dev-subscription-external - Start subscription external service only"
	@echo "  make dev-cadence-engine     - Start cadence engine only"
	@echo "  make quick-start            - Quick start for subscription external service"
	@echo ""
	@echo "🔄 Service Management:"
	@echo "  make start                  - Start all services"
	@echo "  make stop                   - Stop all services"
	@echo "  make restart                - Restart all services"
	@echo "  make status                 - Show repo-scoped, PID-aware service status"
	@echo ""
	@echo "🔨 Local Build Commands:"
	@echo "  make build                  - Build all service binaries"
	@echo "  make build-local-subscription-partner  - Build subscription partner binary"
	@echo "  make build-local-subscription-external - Build subscription external binary"
	@echo "  make build-local-notification          - Build notification binary"
	@echo "  make build-local-acquisition-api       - Build acquisition API binary"
	@echo "  make build-local-cadence-engine        - Build cadence engine binary"
	@echo "  make clean                  - Clean all built binaries"
	@echo ""
	@echo "🐳 Docker Build Commands:"
	@echo "  make docker-build-all      - Build ALL Docker images"
	@echo "  make docker-build-core     - Build core service images"
	@echo "  make docker-build-krakend              - Build KrakenD image"
	@echo "  make docker-build-subscription-partner - Build subscription partner image"
	@echo "  make docker-build-subscription-external - Build subscription external image"
	@echo "  make docker-build-notification         - Build notification image"
	@echo "  make docker-build-acquisition-api      - Build acquisition API image"
	@echo "  make docker-build-cadence-engine       - Build cadence engine image"
	@echo "  make docker-build-landing-web          - Build landing web image"
	@echo "  make docker-build-postback-dispatcher  - Build postback dispatcher image"
	@echo ""
	@echo "📤 Docker Push Commands:"
	@echo "  make docker-push-all       - Push ALL Docker images to registry"
	@echo "  make docker-push-core      - Push core service images"
	@echo "  make docker-push-<service> - Push specific service image"
	@echo ""
	@echo "🚀 Docker Release Commands (Build + Push):"
	@echo "  make docker-release-all    - Build and push ALL images"
	@echo "  make docker-release-core   - Build and push core service images"
	@echo "  make docker-release-<service> - Build and push specific service"
	@echo ""
	@echo "🐳 Docker Compose Commands:"
	@echo "  make compose-up            - Start development stack"
	@echo "  make compose-down          - Stop development stack"
	@echo "  make compose-prod-up       - Start production stack (docker-compose.prod.yml)"
	@echo "  make compose-prod-down     - Stop production stack"
	@echo "  make compose-do-up         - Start DigitalOcean stack (docker-compose.prod-do.yml)"
	@echo "  make compose-do-down       - Stop DigitalOcean stack"
	@echo "  make compose-do-pull       - Pull latest images for DO stack"
	@echo "  make compose-do-restart    - Restart DO stack with latest images"
	@echo "  make docker-ps             - List running containers"
	@echo "  make docker-images         - List service Docker images"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  make test                   - Run all tests"
	@echo "  make test-subscription-external - Test subscription external service"
	@echo ""
	@echo "🏥 Health Checks:"
	@echo "  make health                 - Check health of all services"
	@echo "  make health-subscription-external - Check subscription external service health"
	@echo ""
	@echo "📋 Logs:"
	@echo "  make logs                   - Show logs for all services"
	@echo "  make compose-prod-logs      - View production stack logs"
	@echo "  make compose-do-logs        - View DigitalOcean stack logs"
	@echo ""
	@echo "🗄️  Database Commands:"
	@echo "  make db-connect             - Connect to remote PostgreSQL database"
	@echo "  make db-exec-sql FILE=...   - Execute SQL file against remote database"
	@echo "  make db-migrate-campaigns   - Run all campaign-related migrations"
	@echo "  make db-migrate-cadence     - Run cadence engine migration"
	@echo "  make db-migrate-tenant-platform-dry-run - Dry-run tenant platform migration"
	@echo "  make db-migrate-tenant-platform - Apply tenant platform migration"
	@echo "  make db-create-mobplus-campaign - Create Mobplus campaign with click_id"
	@echo "  make db-configure-level23-campaign - Configure Level23 campaign/postback"
	@echo "  make db-generate-level23-share-info - Print Level23 sharing templates"
	@echo "  make db-update-gh-lp-copy-msisdn-format - Update GH campaign lp_copy text"
	@echo "  make db-list-campaigns      - List all campaigns"
	@echo "  make db-campaign-details SLUG=... - Show detailed campaign info"
	@echo ""
	@echo "🛠️ Development Tools:"
	@echo "  make tools                  - Install development tools"
	@echo "  make init                   - Initialize protobuf tools"
	@echo "  make update_deps            - Update Go dependencies"
	@echo ""
	@echo "📚 Documentation:"
	@echo "  make help                   - Show this help message"
	@echo ""
	@echo "💡 Examples:"
	@echo "  # Build and push all images:"
	@echo "  make docker-release-all"
	@echo ""
	@echo "  # Deploy to DigitalOcean droplet:"
	@echo "  make docker-release-all"
	@echo "  ssh user@droplet 'cd /app && make compose-do-restart'"
