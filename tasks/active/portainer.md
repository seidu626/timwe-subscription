## Portainer for container management

**Status**: completed
**Owner**: ops
**Created**: 2026-01-21

### Goal
Add Portainer to the droplet Docker Compose stack to manage containers via a web UI.

### Constraints
- Do not expose the Portainer UI publicly by default (bind to `127.0.0.1`).
- Use persistent storage volume for Portainer state.

### Dependencies
- Docker engine and `docker compose` working on the droplet.

### ExitCriteria
- Portainer service exists in the droplet compose file with:
  - `portainer/portainer-ce` image
  - `/var/run/docker.sock` bind mount (local Docker management)
  - persistent `portainer_data` volume
  - UI bound to `127.0.0.1:9443`
- Documented access method via SSH tunnel.

### Notes
- 2026-01-21: Added `portainer` service to `docker-compose.prod-do.yml`, bound to `127.0.0.1:9443` (and `:9000`) with persistent `portainer_data`.
- 2026-01-21: Fixed notification service env binding so `APP_DATABASE_POSTGRESQL_*` overrides are honored (prevents defaulting to `localhost/::1:5432` inside containers).
- 2026-01-21: Updated `services/notification/Dockerfile` (and `services/billing/Dockerfile`) to Go 1.24-alpine to match go.mod `go 1.24.2` requirement.
- 2026-01-21: Updated `k8s/deployment.yml` to set `imagePullPolicy: Always` for `notification` and `notification-worker` (so `:latest` pulls include the env-binding fix).
- 2026-01-21: Fixed droplet compose healthchecks to use GET (`wget -q -O /dev/null`) instead of `wget --spider` (HEAD) which caused `/health` to return 405 and containers to remain `unhealthy`.
- 2026-01-21: Updated KrakenD admin routing/CORS/header passthrough for WebSPA admin access.
- 2026-01-21: Added admin input_headers + lowercase token passthrough in KrakenD configs/templates.
- 2026-01-21: Investigating KrakenD admin 401 for `/v1/admin/campaigns` on droplet.
- 2026-01-22: Started Auth0 migration work: updated KrakenD admin endpoints to require `Authorization: Bearer` JWT and removed `X-Admin-Token` passthrough.
- 2026-01-22: Updated acquisition-api admin auth to validate Auth0 JWTs from `Authorization: Bearer` (server-side defense-in-depth).
- 2026-01-22: Updated cadence-engine admin auth to validate Auth0 JWTs from `Authorization: Bearer` (server-side defense-in-depth).
- 2026-01-22: Updated webspa-admin to use Auth0 login and send `Authorization: Bearer` tokens (removed manual admin token UI/interceptor).
- 2026-01-22: Removed ACQUISITION_ADMIN_TOKEN/CADENCE_ADMIN_TOKEN from all compose/k8s configs; replaced with ADMIN_AUTH0_DOMAIN/ADMIN_AUTH0_AUDIENCE.
- 2026-01-22: Auth0 admin authentication migration complete. All admin endpoints now require valid JWT; frontend authenticates via Auth0 Universal Login.
- 2026-01-22: Added Portainer SSH tunnel access documentation to `docs/environment-variables.md`. All exit criteria met.

