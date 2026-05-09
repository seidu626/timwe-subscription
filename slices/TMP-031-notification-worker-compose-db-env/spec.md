# TMP-031 Spec

## User Story

As a verification agent, I can start notification-worker against the local compose database, so worker startup can be verified separately from message schema provisioning and provider delivery.

## Acceptance Criteria

- `notification-worker` compose env includes local Postgres host, port, user, password, database, and `sslmode=disable`.
- Compose config renders with `.env.example`.
- Targeted notification-worker smoke starts database and notification-worker.
- Worker remains running long enough to log startup and expose metrics.
- No notification source, dependency metadata, vendor, package manifest, frontend, credential, or schema files are changed.

## Non-Goals

- No message outbox schema migration or database bootstrap changes.
- No notification dispatcher behavior changes.
- No provider/live-flow delivery verification.

## Verification

```bash
docker compose --env-file .env.example -f docker-compose.yml config
docker compose --project-name timwe_tmp031 --env-file .env.example -f docker-compose.yml up -d --build database notification-worker
docker inspect -f '{{.State.Status}}' notification-worker
docker logs notification-worker
```
