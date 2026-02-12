# Production Environment Configuration

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-17
- Completed: 2026-01-17

## Dependencies
- None

## ExitCriteria
- [x] docker-compose.prod.yml includes all services with consistent env var naming
- [x] k8s/deployment.yml includes all service deployments and services
- [x] Environment variable documentation updated

## Todos
1. docker-compose-fix - Fix env var naming and add missing services [completed]
2. k8s-complete - Add missing K8s deployments (landing-web, subscription-external, notification) [completed]
3. env-docs - Document all required environment variables [completed]

## Changes Made
1. Updated docker-compose.prod.yml:
   - Added subscription-partner, subscription-external, landing-web, postback-dispatcher services
   - Added redis service for caching
   - Standardized env var naming to `APP_DATABASE_POSTGRESQL_*` prefix
   - Fixed notification-worker to use consistent env vars
   - Added proper service dependencies

2. Updated k8s/deployment.yml:
   - Added subscription-partner Deployment + Service
   - Added subscription-external Deployment + Service
   - Added notification Deployment + Service
   - Added landing-web Deployment + Service
   - Added postback-dispatcher Deployment
   - All services use secrets for credentials

3. Created docs/environment-variables.md:
   - Comprehensive documentation of all env vars
   - Kubernetes secrets creation commands
   - Service ports reference
   - Security notes
