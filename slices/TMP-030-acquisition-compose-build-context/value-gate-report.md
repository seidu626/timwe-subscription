# TMP-030 Value Gate Report

- Timestamp: 2026-05-09T03:10:20Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:fixed

## Audit 1: Acceptance Criteria Coverage

- Compose build context includes repo-local `common`: COVERED by `docker-compose.yml` moving acquisition-api build context to repo root.
- Dockerfile preserves relative module replacement: COVERED by copying `common` to `/common` and service files to `/build`.
- Vendor dependency removed from image build path: COVERED by `go build -mod=readonly`.
- Acquisition API image build succeeds with isolated temporary auth: COVERED by compose build evidence.
- Source/dependency scope preserved: COVERED by forbidden file-scope validation.

Audit 1 result: PASS.

## Audit 2: Failure Mode Coverage

- Service-only build context cannot see `../../common`: FIXED by repo-root context and explicit Dockerfile path.
- Missing service-local vendor tree: FIXED by readonly module build instead of vendor mode.
- Docker auth/tooling blocker from TMP-029: MITIGATED for this verification by temporary empty auth files; no credential mutation.
- App runtime overclaim: MITIGATED by recording the acquisition schema bootstrap failure as a downstream blocker.

Audit 2 result: PASS.

## Audit 3: Domain Invariant Preservation

- No Go source, dependency metadata, vendor, frontend, package manifest, or lockfile files changed: PRESERVED.
- Compose config remains example-env driven and does not add secret material: PRESERVED.
- Full compose readiness is not claimed from image build success: PRESERVED.

Audit 3 result: PASS.

## Commands

```bash
DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker pull docker.io/library/golang:1.24-alpine
DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker compose --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml build acquisition-api
DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker compose --project-name timwe_probe_<id> --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml up -d --build database redis minio minio-init subscription notification cadence-engine acquisition-api krakend landing-web postback-dispatcher notification-worker
targeted acquisition-api runtime probe with database, redis, minio, minio-init, and acquisition-api
```

## Result

PASS for the TMP-030 defect: the acquisition API compose image builds successfully.

The full compose smoke now progresses beyond image build and starts multiple services. Health probes passed for subscription, notification API, cadence, and landing-web. Acquisition API then exited at runtime during admin schema bootstrap because `migrations/add_admin_management_tables.sql` expects relation `products`; notification-worker and postback-dispatcher also exposed separate runtime configuration blockers. Those are downstream verification defects, not TMP-030 build-context failures.
