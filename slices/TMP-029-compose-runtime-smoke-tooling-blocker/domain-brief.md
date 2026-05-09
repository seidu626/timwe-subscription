# TMP-029 Domain Brief

## Actors

- Verification agent: performs bounded local compose runtime smoke checks and records release evidence.
- Platform operator: uses blocker evidence to prepare local runtime dependencies and tooling before rerunning smoke checks.

## Ubiquitous Language

- Compose smoke: a bounded local `docker compose up` attempt intended to prove app containers can build, start, and expose health endpoints.
- Temporary override: a non-repo compose override file used to avoid an unrelated host port conflict during verification.
- Tooling blocker: a local container runtime or registry-auth failure that prevents app containers from starting.

## Domain Invariants

- Compose config rendering is not runtime verification.
- A build-image pull failure before app containers start must not be labeled as an application runtime defect.
- Temporary verification scaffolding must be cleaned up and must not change tracked source or compose files.

## Failure Modes

- Host port already in use: mitigated by the temporary Redis port override.
- Required external network missing: mitigated for the smoke by temporarily creating `shared-network`.
- Registry auth invalid: observed blocker; Docker/Podman could not pull the Go builder image.
- Evidence overclaim: avoided by keeping compose runtime blocked until app containers actually start and health checks pass.

## User Journey

1. Verification agent renders compose config with `.env.example`.
2. Verification agent applies a temporary port override and creates the missing external network.
3. Compose attempts to build runtime images.
4. Container tooling fails while pulling the Go builder image, before app containers start.
5. Evidence records the blocker and cleanup state.

