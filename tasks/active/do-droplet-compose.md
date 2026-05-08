# DO Droplet Docker Compose Readiness

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-19
- Completed: 2026-01-19

## Dependencies
- None

## ExitCriteria
- [x] KrakenD settings use compose service DNS names (not host.docker.internal)
- [x] Endpoint.tmpl and compose files use consistent subscription_external_api_url
- [x] docker-compose.prod-do.yml includes all services with proper networking
- [x] Health endpoints added to notification and subscription-partner services
- [x] subscription-external Dockerfile uses alpine with CA certs
- [x] Hardcoded credentials removed from all service config.yaml files

## Changes Made
1. Created `krakend/config/settings/do/service.json` with compose service DNS names
2. Updated `krakend/config/templates/Endpoint.tmpl` to use `subscription_external_api_url`
3. Updated `krakend/config/settings/service.json` to use `subscription_external_api_url`
4. Rewrote `docker-compose.prod-do.yml` with:
   - All services (krakend, subscription-partner, subscription-external, notification, notification-worker, acquisition-api, cadence-engine, postback-dispatcher, landing-web, redis, database, pgadmin)
   - Proper compose DNS networking (no host.docker.internal)
   - TLS certificate volume mounts for KrakenD
   - Docker healthchecks for all services
   - Internal backend network (not external)
5. Added `/health` endpoint to `services/notification/internal/transport/router.go`
6. Added `/health` endpoint to `services/subscription-partner/internal/transport/router.go`
7. Updated `services/subscription-external/Dockerfile` to use alpine with CA certs
8. Removed hardcoded credentials from:
   - `services/notification/config.yaml`
   - `services/subscription-partner/config.yaml`
   - `services/acquisition-api/config.yaml`
   - `services/cadence-engine/config.yaml`
   - `services/subscription-external/config.yaml`
   - `services/postback-dispatcher/config.yaml`

## Notes
- SSL certificates must be placed in `./certs/` directory on the droplet
- Environment variables must be set via `.env` file (see docs/environment-variables.md)
- KrakenD uses `FC_SETTINGS=/etc/krakend/config/settings/do` to load droplet-specific settings
