# TMP-032 Domain Brief

## Slice

Postback dispatcher compose DB env

## Actor

Verification agent running the full-system compose smoke.

## Outcome

Postback dispatcher connects to local compose Postgres using environment variable names consumed by `common/config`, then starts its worker loop.

## System Path

1. Compose starts local Postgres from `.env.example`.
2. Compose starts `postback-dispatcher`.
3. Dispatcher loads common config.
4. Dispatcher opens and pings Postgres.
5. Dispatcher starts polling `postback_outbox`.

## Invariants

- Compose-only runtime configuration change.
- No postback source, common source, dependency metadata, vendor tree, package manifest, frontend, credential, or schema files changed.
- Missing postback schema discovered after successful DB connection is recorded as a downstream blocker.

## Downstream Runtime Finding

After TMP-032, postback-dispatcher connects to the database and starts. It then logs polling failures because relation `postback_outbox` is missing in the empty compose database. The postback schema is defined in existing migrations, so compose DB schema provisioning remains a separate readiness gap.
