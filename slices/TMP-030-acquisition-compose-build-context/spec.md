# TMP-030 Spec

## User Story

As a verification agent, I can build the acquisition API compose image from the repo-local module graph, so the full compose smoke can progress past a stale vendor/build-context failure.

## Acceptance Criteria

- `docker-compose.yml` points the acquisition API build at a context containing both `services/acquisition-api` and `common`.
- `services/acquisition-api/Dockerfile` copies the service and common module into matching relative paths for the existing `replace ../../common` directive.
- The Dockerfile builds with readonly module resolution and does not require a service-local vendor directory.
- Acquisition API image build succeeds using temporary isolated Docker auth files.
- No Go source, dependency metadata, vendor, frontend, package manifest, or lockfile files are changed.
- Runtime blockers found after image build are recorded as separate follow-up defects.

## Non-Goals

- No dependency or vendor regeneration.
- No schema migration rewrite.
- No Docker credential repair.
- No provider/live-flow verification.

## Verification

```bash
DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker compose --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml build acquisition-api
jq empty slices/manifest.json agent/state/TMP-030.work-order.json agent/state/TMP-030.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status && slice-harness sync --dry-run
```
