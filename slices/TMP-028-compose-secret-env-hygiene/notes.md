# TMP-028 Notes

## Domain Grounding

- Actor: platform-operator.
- Business outcome: local compose config can be rendered and shared without repository-embedded subscription database credential material.
- Domain invariant: removing checked-in credential material must not imply runtime/provider flows are verified.
- Entrypoint: `docker-compose.yml`.
- Risk: placeholder env values can make config render but are not proof of end-to-end runtime behavior.

## Story Craft

The story is concrete and testable: the operator renders compose config with `.env.example`, and the subscription service DB settings come from environment inputs instead of hardcoded values.

## Value Gate

Pass criteria:
- `docker compose --env-file .env.example -f docker-compose.yml config` succeeds.
- The subscription service no longer contains hardcoded DB host or password material.
- Host/port override names use the existing `APP_DATABASE_POSTGRESQL_*` service env shape; no new `PG_HOST` or `PG_PORT` service bindings were introduced.
- The subscription service defaults to Docker `database` routing, and the existing literal SSL mode is preserved.
- `.env.example` contains safe placeholders, not real credentials.
- TMP-021 still records runtime start as blocked until real env/provider values are supplied.

## Claude Critique

Claude was asked to review actor, business outcome, invariant, entrypoint, risk, acceptance proof, and file scope before implementation. Its useful corrections were folded in: service-native host/port override names, explicit Docker `database` default routing, literal SSL-mode preservation, fuller `.env.example` scope, and stronger command evidence. The slice keeps source, dependency, vendor, frontend, package manifest, and lockfile files out of scope.
