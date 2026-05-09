# TMP-028 Spec

## Outcome

Local compose configuration no longer depends on checked-in subscription database credential material.

## Acceptance

- `docker-compose.yml` subscription service uses env inputs for database credentials, service-native host/port overrides, Docker `database` routing by default, and literal SSL mode.
- Root `.env.example` documents safe placeholder values for compose-required variables.
- `docker compose --env-file .env.example -f docker-compose.yml config` succeeds.
- Search confirms the previous hardcoded subscription DB host/password patterns are absent.
- TMP-021 evidence distinguishes config hygiene completion from remaining runtime/provider blockers.

## Non-Goals

- No real credential generation or storage.
- No compose runtime start.
- No provider-flow verification.
- No service source, dependency, vendor, frontend, manifest, or lockfile changes.
- No changes to `common/config/config.go` BindEnv or new `PG_HOST` / `PG_PORT` service config bindings.
- No local `main` integration.
