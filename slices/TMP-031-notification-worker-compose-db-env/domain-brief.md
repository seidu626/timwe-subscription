# TMP-031 Domain Brief

## Slice

Notification worker compose DB env

## Actor

Verification agent running the full-system compose smoke.

## Outcome

Notification worker connects to local compose Postgres with explicit DB host, port, user, password, database, and `sslmode=disable`, then starts its dispatcher loop and metrics endpoint.

## System Path

1. Compose starts local Postgres from `.env.example`.
2. Compose starts `notification-worker`.
3. Worker loads notification config and environment overrides.
4. Worker opens and pings Postgres.
5. Worker starts dispatcher polling and metrics.

## Invariants

- Compose-only runtime configuration change.
- No notification source, dependency metadata, vendor tree, package manifest, frontend, credential, or schema files changed.
- Missing message schema discovered after successful DB ping is recorded as a downstream blocker.

## Downstream Runtime Finding

After TMP-031, notification-worker starts and exposes metrics, then logs dispatcher batch failures because relation `message_outbox` is missing in the empty compose database. The table is defined by subscription-external migration `011_message_cadence_engine.sql`, so schema provisioning remains a separate runtime readiness gap.
