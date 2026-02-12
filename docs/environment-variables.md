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
| `APP_DATABASE_POSTGRESQL_HOST` | Database hostname |
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

## TimWe API Integration

| Variable | Description | Required |
|----------|-------------|----------|
| `TIMWE_API_KEY` | TimWe partner API key | Yes |
| `TIMWE_PSK` | TimWe pre-shared key | Yes |

## Acquisition API

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ADMIN_AUTH0_DOMAIN` | Auth0 tenant domain for admin JWT validation (e.g. `dev-chliep5q.auth0.com`) | - | Yes |
| `ADMIN_AUTH0_AUDIENCE` | Auth0 API Audience/Identifier expected in `aud` claim. Supports comma-separated values to allow migration (e.g. `aud1,aud2`). | - | Yes |
| `ACQUISITION_ADMIN_CORS_ORIGINS` | Comma-separated allowed CORS origins | `http://localhost:4200` | No |

## Notification Worker

| Variable | Description | Default |
|----------|-------------|---------|
| `NOTIFICATION_WORKER_MT_BASE_URL` | Base URL for MT message sending | `http://subscription-external:8083` |
| `NOTIFICATION_WORKER_MT_CHANNEL` | Message channel type | `SMS` |

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

### timwe-credentials

```bash
kubectl create secret generic timwe-credentials \
  --from-literal=api-key=your_api_key \
  --from-literal=psk=your_psk
```

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

1. **Never commit secrets to version control** - Use `.env` files locally and Kubernetes secrets in production.
2. **Use strong passwords** - Generate secure random passwords for all database and API credentials.
3. **Rotate secrets regularly** - Implement a secret rotation policy.
4. **Use SSL in production** - Set `APP_DATABASE_POSTGRESQL_SSL_MODE=require` for production deployments.
5. **Restrict CORS origins** - Don't use `*` for `ACQUISITION_ADMIN_CORS_ORIGINS` in production.
