# TMP-028 Value Gate Report

- Timestamp: 2026-06-08T06:00:00Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:verified

## Audit 1: Acceptance Criteria Coverage

- Subscription service DB config uses env inputs: COVERED by `docker-compose.yml` diff.
- Safe placeholder env scaffold exists: COVERED by `.env.example`.
- Compose config renders with example env: COVERED by `docker compose --env-file .env.example -f docker-compose.yml config`.
- Previous hardcoded subscription DB host/password patterns are absent: COVERED by `rg -n 'APP_DATABASE_POSTGRESQL_HOST=139|APP_DATABASE_POSTGRESQL_PASSWORD=[^$]' docker-compose.yml || true`.
- Slice manifest is valid JSON: COVERED by `jq empty slices/manifest.json`.
- vNext schema and reconcile gates pass: COVERED by `agent-hub schema validate-all --root . --json` and `agent-hub reconcile --root . --check --json`.
- TMP-021 keeps runtime start blocked until real env/provider values are supplied: COVERED by value-gate update.

Audit 1 result: PASS.

## Audit 2: Failure Mode Coverage

- Blank env render: MITIGATED by `.env.example` for local config rendering.
- Checked-in credential material: MITIGATED for the subscription service DB config by switching to env inputs.
- Placeholder mistaken for live credential: MITIGATED by comments and continued TMP-021 blocker.
- Overclaiming runtime readiness: MITIGATED by leaving runtime start blocked.

Audit 2 result: PASS.

## Audit 3: Domain Invariant Preservation

- Runtime config must not depend on checked-in credential material: PRESERVED for the subscription service DB config.
- Config render is not runtime verification: PRESERVED.
- No product/dependency/vendor/frontend changes: PRESERVED.

Audit 3 result: PASS.

## Commands

```bash
docker compose --env-file .env.example -f docker-compose.yml config
rg -n 'APP_DATABASE_POSTGRESQL_HOST=139|APP_DATABASE_POSTGRESQL_PASSWORD=[^$]' docker-compose.yml || true
jq empty slices/manifest.json
agent-hub schema validate-all --root . --json
agent-hub reconcile --root . --check --json
git diff --check
```

Result: PASS on 2026-06-08. `docker compose` rendered 382 config lines with `.env.example`; the hardcoded host/password pattern search returned no matches; `agent-hub reconcile` reported zero findings.
