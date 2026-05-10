# TMP-032 Spec

## User Story

As a verification agent, I can start postback-dispatcher against the local compose database, so dispatcher startup can be verified separately from postback schema provisioning and external delivery.

## Acceptance Criteria

- `postback-dispatcher` compose env includes `DB_POSTGRESQL_*` names consumed by `common/config`.
- Compose config renders with `.env.example`.
- Targeted postback-dispatcher smoke starts database and dispatcher.
- Dispatcher logs database connection and starts its worker loop.
- No postback source, common source, dependency metadata, vendor, package manifest, frontend, credential, or schema files are changed.

## Non-Goals

- No postback schema migration or database bootstrap changes.
- No dispatcher behavior changes.
- No external postback delivery verification.

## Verification

```bash
docker compose --env-file .env.example -f docker-compose.yml config
docker compose --project-name timwe_tmp032 --env-file .env.example -f docker-compose.yml up -d --build database postback-dispatcher
docker inspect -f '{{.State.Status}}' postback-dispatcher
docker logs postback-dispatcher
```
