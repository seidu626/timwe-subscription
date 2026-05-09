# TMP-029 Value Gate Report

- Timestamp: 2026-05-09T02:44:23Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:verified

## Audit 1: Acceptance Criteria Coverage

- Bounded compose smoke attempt recorded: COVERED by release matrix command row.
- Temporary Redis port override recorded: COVERED by release matrix and notes.
- Temporary external network creation and cleanup recorded: COVERED by release matrix and handoff.
- Docker auth/tooling blocker recorded: COVERED by compose smoke failure and direct image-pull reproduction.
- Source-scope preservation: COVERED by file-scope validation.

Audit 1 result: PASS.

## Audit 2: Failure Mode Coverage

- Host port conflict: MITIGATED for the smoke by temporary Redis host-port override.
- Missing external network: MITIGATED for the smoke by temporary `shared-network` creation.
- Docker registry auth failure: OBSERVED and recorded; no credential mutation attempted.
- App runtime overclaim: MITIGATED by keeping compose runtime blocked.

Audit 2 result: PASS.

## Audit 3: Domain Invariant Preservation

- Config render is not runtime verification: PRESERVED.
- Tooling failure before app container startup is not an app runtime defect: PRESERVED.
- No source, compose, dependency, vendor, frontend, package manifest, or lockfile changes: PRESERVED.

Audit 3 result: PASS.

## Commands

```bash
docker compose --env-file .env.example -f docker-compose.yml config
docker compose --project-name timwe_smoke_024125 --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-smoke-override.yml up -d --build database redis minio minio-init subscription notification cadence-engine acquisition-api krakend landing-web postback-dispatcher notification-worker
docker pull docker.io/library/golang:1.24-alpine
docker network inspect shared-network
test ! -e /tmp/timwe-compose-smoke-override.yml
git status file-scope check for forbidden paths
```

Result: PASS for evidence integrity; compose runtime remains BLOCKED by local Docker registry auth/tooling before app startup.
