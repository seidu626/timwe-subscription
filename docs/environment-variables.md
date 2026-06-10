# Environment Variables Reference

This document describes all environment variables used by the Subscription Manager platform.

## PostgreSQL Database Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `POSTGRESQL_VERSION` | PostgreSQL Docker image version | `16` | No |
| `PG_USER` | Database username | - | Yes |
| `PG_PASSWORD` | Database password | - | Yes |
| `PG_DB` | Database name | `subscription_manager` | Yes |

For services using `APP_` prefix (subscription-partner, notification, cadence-engine, acquisition-api):

| Variable | Description |
|----------|-------------|
| `APP_DATABASE_POSTGRESQL_HOST` | Database hostname (Docker Compose default: `database`) |
| `APP_DATABASE_POSTGRESQL_PORT` | Database port (default: 5432) |
| `APP_DATABASE_POSTGRESQL_USER` | Database username |
| `APP_DATABASE_POSTGRESQL_PASSWORD` | Database password |
| `APP_DATABASE_POSTGRESQL_DB_NAME` | Database name |
| `APP_DATABASE_POSTGRESQL_SSL_MODE` | SSL mode (`disable`, `require`, `verify-ca`, `verify-full`) |

## Redis Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_CACHE_REDIS_HOST` | Redis hostname | `redis` |
| `APP_CACHE_REDIS_PORT` | Redis port | `6379` |
| `APP_CACHE_REDIS_DB` | Redis database number | `0` |

## Authentication

| Variable | Description | Required |
|----------|-------------|----------|
| `JWT_SECRET` | Secret key for JWT token signing | Yes |

## Multi-Tenant Configuration Taxonomy

Production onboarding separates tenant/channel configuration from platform deployment configuration. Do not promote a tenant to production by adding provider credentials as global environment variables.

| Category | Examples | Production Storage | Notes |
|----------|----------|--------------------|-------|
| Tenant/channel-specific configuration | `tenant_key`, `channel_key`, partner/provider realm, callback URL, postback URL, enabled capabilities, product defaults, userbase defaults, cadence defaults | Admin onboarding record plus tenant/channel credential secret references | Scoped to one tenant/channel. Changes must not affect other tenants. |
| Tenant/channel credential secrets | `TIMWE_API_KEY`, `TIMWE_PSK`, callback shared secret, postback signing secret, equivalent provider auth material | Secret manager entry referenced by the tenant/channel credential record | Store references such as `<tenant-channel-credential-secret-ref>`, never raw values in docs, tickets, screenshots, or shared global env. |
| Platform-wide shared secrets | `JWT_SECRET`, `INTERNAL_API_SECRET`, `CADENCE_ADMIN_TOKEN`, database credentials, Redis credentials, Auth0 admin validation settings, monitoring admin credentials | Deployment secret store or environment for the whole platform | Shared infrastructure secrets. Rotate with platform release coordination. These are not tenant identity or provider routing keys. |
| Dev-only or legacy fallback | Local `.env` placeholders for `TIMWE_API_KEY`, `TIMWE_PSK`, `ACQUISITION_ADMIN_TOKEN`, MinIO defaults, local Auth0/demo values | Developer-local `.env` or local compose overrides only | Allowed only for sandbox/local smoke or deprecated static-token compatibility. Must not be used to onboard production tenants. |
| Deployment-only credentials | `PASS_KEY` or registry personal access tokens, object storage access keys, Kubernetes secret names, droplet/Portainer access | Deployment platform secret store | Used by infrastructure and release automation, not tenant routing decisions. Do not inject these into application config unless the deployment tool explicitly needs them. |

## TimWe / Provider Credential Fallback

| Variable | Description | Required |
|----------|-------------|----------|
| `TIMWE_API_KEY` | TimWe partner API key fallback for local development or sandbox-only smoke | No for production tenant onboarding |
| `TIMWE_PSK` | TimWe pre-shared key fallback for local development or sandbox-only smoke | No for production tenant onboarding |
| `SUBSCRIPTION_EXTERNAL_DEV_ALLOW_TENANT_PROVIDER_SHARED_CREDENTIAL_FALLBACK` | Explicit development-only gate that lets tenant-routed provider calls reuse shared TimWe auth material when a tenant credential secret is incomplete | No; leave unset outside local development |

For production, `TIMWE_API_KEY`, `TIMWE_PSK`, callback shared secrets, postback signing secrets, and equivalent provider auth material must be stored as tenant/channel credential secret references. They must not be treated as a single global env pair that applies to all tenants. The onboarding record should hold only the credential reference, owner, rotation cadence, and activation state.

Tenant-routed provider calls in `subscription-external` are strict by default. When tenant/channel context is present, `tenant_channel_credentials` purpose `provider_api` is authoritative for provider API key and authentication material. The shared TimWe environment values are only usable for tenant routes when the application runs in `DEVELOPMENT` and `SUBSCRIPTION_EXTERNAL_DEV_ALLOW_TENANT_PROVIDER_SHARED_CREDENTIAL_FALLBACK=true`.

If a raw credential-shaped value has been exposed to an agent, ticket, chat, log, screenshot, or documentation draft, assume it is compromised. Rotate the provider credential, replace the tenant/channel credential secret reference, run the tenant/channel smoke validation, and record the rotation in the onboarding evidence before activation or reactivation.

## Acquisition API

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ADMIN_AUTH0_DOMAIN` | Auth0 tenant domain for admin JWT validation (e.g. `dev-chliep5q.auth0.com`) | - | Yes |
| `ADMIN_AUTH0_AUDIENCE` | Auth0 API Audience/Identifier expected in `aud` claim (for this project: `https://dev-chliep5q.auth0.com/api/v2/`). | - | Yes |
| `ACQUISITION_ADMIN_CORS_ORIGINS` | Comma-separated allowed CORS origins | `http://localhost:4200` | No |

### Acquisition API Campaign Asset Storage (Optional)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `CAMPAIGN_ASSET_STORAGE_ENABLED` | Enables campaign background image upload endpoint | `false` | No |
| `CAMPAIGN_ASSET_STORAGE_ENDPOINT` | S3-compatible endpoint host/URL | - | Yes (if enabled) |
| `CAMPAIGN_ASSET_STORAGE_BUCKET` | Target object storage bucket/container | - | Yes (if enabled) |
| `CAMPAIGN_ASSET_STORAGE_REGION` | Object storage region | - | No |
| `CAMPAIGN_ASSET_STORAGE_ACCESS_KEY_ID` | Access key ID | - | Yes (if enabled) |
| `CAMPAIGN_ASSET_STORAGE_SECRET_ACCESS_KEY` | Secret access key | - | Yes (if enabled) |
| `CAMPAIGN_ASSET_STORAGE_USE_SSL` | Use TLS for storage endpoint | `true` | No |
| `CAMPAIGN_ASSET_STORAGE_PUBLIC_BASE_URL` | Public/CDN base URL used for generated asset URLs | - | No |
| `CAMPAIGN_ASSET_STORAGE_KEY_PREFIX` | Prefix/folder for campaign background objects | `campaign-backgrounds` | No |
| `CAMPAIGN_ASSET_STORAGE_MAX_UPLOAD_BYTES` | Max upload payload size in bytes | `2097152` | No |
| `CAMPAIGN_ASSET_STORAGE_PRESIGN_EXPIRY` | Presigned URL validity duration | `10m` | No |

### MinIO (Docker local S3 backend)

When using `docker-compose.yml`, MinIO is used as the default S3 backend for campaign background uploads.
MinIO API is exposed on `http://localhost:9100` and MinIO Console on `http://localhost:9101`.

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `MINIO_ROOT_USER` | MinIO root access key/user | `minioadmin` | No |
| `MINIO_ROOT_PASSWORD` | MinIO root secret key/password | `minioadmin` | No |

> These MinIO defaults are for local development only. Override them before any shared or production-like deployment.

## Notification Service

| Variable | Description | Default |
|----------|-------------|---------|
| `NOTIFICATION_REQUIRE_TENANT_CONTEXT` | When `true`, callback routes (MO/MT_DN/USER_OPTIN/USER_RENEWED/USER_OPTOUT/CHARGE) reject requests with no resolvable tenant_key+channel_key with HTTP 422. Set to `false` to restore the legacy tenantless passthrough. | `true` |

## Notification Worker

| Variable | Description | Default |
|----------|-------------|---------|
| `NOTIFICATION_WORKER_MT_BASE_URL` | Base URL for MT message sending | `http://subscription-external:8083` |
| `NOTIFICATION_WORKER_MT_CHANNEL` | Message channel type | `SMS` |
| `NOTIFICATION_WORKER_METRICS_ADDR` | Prometheus metrics bind address for the worker | `:9103` |

## Monitoring Stack

| Variable | Description | Required |
|----------|-------------|----------|
| `GRAFANA_ADMIN_PASSWORD` | Grafana admin password for the local monitoring stack | Yes |

## Landing Web (Next.js)

| Variable | Description | Required |
|----------|-------------|----------|
| `NEXT_PUBLIC_ACQUISITION_API_URL` | Public URL for acquisition API | Yes |

## PgAdmin (Development Only)

| Variable | Description |
|----------|-------------|
| `PGADMIN_DEFAULT_EMAIL` | Admin email for PgAdmin |
| `PGADMIN_DEFAULT_PASSWORD` | Admin password for PgAdmin |
| `PGADMIN_LISTEN_PORT` | Port for PgAdmin web interface |

## Kubernetes Secrets

For Kubernetes deployments, create the following secrets:

### db-credentials

```bash
kubectl create secret generic db-credentials \
  --from-literal=host=your-db-host \
  --from-literal=username=sm_admin \
  --from-literal=password=your_password \
  --from-literal=database=subscription_manager
```

### tenant-channel provider credentials

Production provider credentials are provisioned per tenant/channel. The deployment should expose a credential reference to the application or credential resolver; it should not create one shared `timwe-credentials` secret for every tenant.

Example secret-reference shape:

```text
tenant_key: <tenant-key>
channel_key: <channel-key>
provider: timwe
credential_ref: <tenant-channel-credential-secret-ref>
rotation_owner: <operations-owner>
rotation_required: true when raw credential-like values were exposed
```

The referenced secret can contain provider-specific entries such as `TIMWE_API_KEY`, `TIMWE_PSK`, callback signing material, and equivalent provider auth fields. Keep those raw values out of documentation and command examples.

### admin-auth0

```bash
kubectl create secret generic admin-auth0 \
  --from-literal=domain=your_auth0_domain \
  --from-literal=audience=your_auth0_audience
```

## Service Ports Reference

| Service | Port | Description |
|---------|------|-------------|
| subscription-partner | 8081 | Partner subscription API |
| notification | 8082 | Notification service |
| subscription-external | 8083 | External subscription API (TimWe integration) |
| acquisition-api | 8084 | Acquisition and campaign API |
| landing-web | 3000 | Landing page web application |
| krakend | 8080 | API Gateway |
| PostgreSQL | 5432 | Database |
| Redis | 6379 | Cache |
| PgAdmin | 5439 | Database admin interface |
| Portainer | 9443 | Container management UI (localhost only) |

## Operations Tools

### Portainer (Container Management)

Portainer is available on production droplets for Docker container management. It is bound to `127.0.0.1:9443` for security and must be accessed via SSH tunnel.

**Access via SSH tunnel:**

```bash
# Create SSH tunnel to access Portainer on the droplet
ssh -L 9443:127.0.0.1:9443 user@your-droplet-ip

# Then open in browser:
# https://localhost:9443
```

**First-time setup:**
1. Create an admin user when prompted
2. Select "Docker" environment (local)
3. Connect to manage containers, view logs, and restart services

**Alternative (port 9000 HTTP):**

```bash
ssh -L 9000:127.0.0.1:9000 user@your-droplet-ip
# Open: http://localhost:9000
```

> **Note:** The Portainer service is defined in `docker-compose.prod-do.yml` and uses a persistent `portainer_data` volume to retain configuration across restarts.

## Security Notes

1. **Use secret references, not literal secrets** - Keep credential-shaped values in environment variables or secret managers; do not hardcode them in compose files or documentation examples.
2. **Never commit secrets to version control** - Use `.env` files locally and Kubernetes secrets in production.
3. **Use strong passwords** - Generate secure random passwords for all database and API credentials.
4. **Rotate exposed secrets immediately** - If raw credential-like values were exposed to an agent or shared channel, rotate them before production activation and update the tenant/channel credential reference.
5. **Use SSL in production** - Set `APP_DATABASE_POSTGRESQL_SSL_MODE=require` for production deployments.
6. **Restrict CORS origins** - Don't use `*` for `ACQUISITION_ADMIN_CORS_ORIGINS` in production.
