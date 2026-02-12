# Project workflow (AI-assisted)

## Prime directive
- Keep diffs minimal; do not refactor unrelated code.
- Preserve public APIs unless explicitly required.
- If behavior changes, add/adjust tests (fail-before/pass-after).

## Commands (use these)
- Lint: `make lint`
- Tests: `make test`
- Typecheck: `make typecheck`
- Full check: `make check`

## Review bundle
- When a change is non-trivial, generate a review bundle:
  - `python tools/review_bundle.py --base origin/main --cmd "make test" --cmd "make lint"`

## Codex handoff format
- Preferred handoff artifact is a git diff + test logs (use review bundle output).
- When asked to prepare PR instructions for Codex, produce a paste-ready `@codex ...` comment.

## Imported repo standards
- See @AGENTS.md


## Development Commands

### Building and Running Services

```bash
# Build all services with Docker
make build-all

# Build individual services
make build-subscription        # subscription-partner service
make build-subscription-external     # subscription-external service  
make build-billing
make build-notification
make build-krakend

# Run entire stack with Docker Compose
make compose-up
docker compose up --build -d && docker compose logs -f

# Stop services
make compose-down
docker compose down --remove-orphans
```

### Testing

```bash
# Run all tests across all services
make test
go test -v ./... -cover

# Run tests for specific services
cd services/billing && go test -v ./...
cd services/notification && go test -v ./...
cd services/subscription-partner && go test -v ./...
cd services/subscription-external && go test -v ./...

# Run specific test file
go test -v ./services/billing/internal/service/billing_test.go
```

### Development Setup

```bash
# Initialize protobuf tools and dependencies
make init

# Update Go module dependencies (all services)
make update_deps
go mod verify && go mod tidy

# Generate protobuf files
make proto

# Generate API documentation (Swagger)
make generate-docs
```

### Docker Operations

```bash
# Push Docker images to registry
make push-all
make push-subSvc-notSvc  # Push subscription and notification services only

# Clean up dangling Docker images
make clean

# Complete build and push workflow
make release-all
```

## Architecture Overview

### Microservices Structure
This is a subscription management system built with microservices architecture using Go 1.24.2:

- **Subscription Partner Service** (Port 8081): Partner-facing subscription operations and product management
- **Subscription External Service** (Port varies): External API integration for TIMWE MA subscription operations  
- **Billing Service** (Port 8083): Payment transactions, billing operations with Saga pattern implementation
- **Notification Service** (Port 8082): Event-driven notifications using Redis pub/sub and PostgreSQL
- **KrakenD API Gateway** (Port 8080): Service mesh with routing, rate limiting, circuit breaking

### Technology Stack

- **Go 1.24.2**: All backend services
- **PostgreSQL**: Primary database (shared database approach)
- **Redis**: Caching layer and pub/sub messaging (notification service)
- **KrakenD**: API Gateway with templated configuration
- **FastHTTP**: High-performance HTTP server (notification service)
- **Prometheus**: Metrics collection across all services
- **Zap**: Structured logging
- **Circuit Breakers**: Sony gobreaker implementation for resilience

### External Integration Architecture

- **TIMWE MA API**: Third-party telecom service integration
  - Base URL: `https://prp.timwe.com/api/external/v1`
  - Authentication: API key + partner authentication key
  - Operations: opt-in, opt-out, status checks via partner role IDs
- **Multi-operator Telecom Support**:
  - MTN: prefixes ["233540", "233550", "233244", "233240"]
  - AirtelTigo: prefixes ["233260", "233270", "233505"]
  - Vodafone: prefixes ["233201", "233202", "233203"]

### Key Design Patterns

- **Clean Architecture**: Domain → Service → Handler → Transport layers consistently applied
- **Circuit Breaker Pattern**: Fault tolerance with configurable failure thresholds
- **Saga Pattern**: Distributed transaction management in billing service
- **Repository Pattern**: Data access abstraction with PostgreSQL implementations
- **Shared Module Architecture**: Common utilities in `/common/` with Go module replacement

### Service Communication & Data Flow

- **API Gateway Routing**: KrakenD routes external requests to appropriate services
- **Database Schema Separation**: Each service manages dedicated tables in shared PostgreSQL
- **Event-Driven Notifications**: Redis pub/sub for asynchronous cross-service communication
- **Health Check Standardization**: `/health` endpoints with Prometheus metrics at `/metrics`

### Business Domain Logic

- **Subscription Lifecycle**: Request → Confirmation (with auth code) → Active/Cancelled states
- **MSISDN-based Operations**: Mobile number as primary user identifier with MCC/MNC routing
- **Product Catalog Management**: Products with price points, partner role associations
- **Billing Transaction Flow**: Transaction creation → Processing → Settlement via Saga pattern
- **Notification Event System**: Tagged events with delivery tracking and retry mechanisms

### Frontend Integration Points

- **Angular Admin Panel**: CoreUI-based interface in `/frontend/webspa-admin/`
- **JWT Authentication**: Configurable token expiration (24h access, 72h refresh)
- **CORS Configuration**: Environment-specific allowed origins
- **Real-time Dashboard**: Integration with notification service for live updates

### Development Environment

- **Module Structure**: Each service has independent go.mod with shared common module
- **Configuration**: YAML-based with environment-specific overrides
- **Docker Development**: Complete stack runnable via docker-compose
- **Database Tooling**: pgAdmin interface accessible on port 5439

### Deployment Architecture

- **Containerization**: Multi-stage Docker builds for each service
- **Kubernetes Deployment**: Manifests in `/k8s/` directory
- **CI/CD Pipeline**: GitHub Actions with automated testing and deployment
- **Environment Parity**: Docker Compose for development, Kubernetes for production
- **Service Discovery**: KrakenD service mesh handles internal service routing