# TMP-028 Domain Brief

## Actors

- Platform operator: renders or starts the local Docker Compose stack to verify tenant-platform runtime readiness. Source: `agent/backlog/issues/TMP-028-compose-secret-env-hygiene.md`.
- Verification agent: checks compose configuration and records release-readiness evidence without handling real credentials. Source: `slices/TMP-021-full-system-verification/slice.yaml`.

## Ubiquitous Language

- Compose stack: the local runtime graph defined by `docker-compose.yml`.
- Secret-shaped config: credential-like material committed directly in configuration instead of supplied by environment.
- Environment scaffold: `.env.example`, a non-secret placeholder file used to render compose config locally.
- Runtime blocker: a condition that still prevents safe end-to-end startup or live flow verification.

## Domain Invariants

- Runtime configuration must not depend on checked-in credential material.
- Placeholder example values must be obviously non-production and replaceable.
- Subscription defaults route to the Docker `database` service for local compose, while SSL mode remains the existing literal `disable` value.
- Removing checked-in credential material must not imply live runtime or provider flows are verified.
- Product code, dependency metadata, vendor trees, and lockfiles must remain untouched.

## Failure Modes

- Compose renders with blank required values: services may start with empty secrets or database settings.
- Compose carries repository-embedded credentials: operators may leak or reuse unsafe values.
- Placeholder values are mistaken for production credentials: shared or production-like deployments become unsafe.
- Evidence overclaims readiness: config render passes but live dependencies/provider credentials remain unverified.

## User Journey

1. Platform operator copies `.env.example` to `.env` or supplies equivalent variables.
2. Operator runs `docker compose --env-file .env.example -f docker-compose.yml config`.
3. Compose resolves subscription service database settings from environment inputs.
4. Release matrix shows checked-in subscription DB credential material has been removed while runtime start remains blocked until real values are supplied.

Failure path:
1. Operator runs compose without real env/provider values.
2. Config may render, but live runtime/provider flow verification remains blocked and must not be marked passed.

## Open Questions

- Whether the operator wants local fake provider values for smoke startup or real provider credentials for full live-flow verification remains unresolved.
- Whether `shared-network` should stay external or be made local-dev-managed is outside this slice.
