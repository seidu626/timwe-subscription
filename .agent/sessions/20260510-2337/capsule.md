# Session Capsule: 20260510-2337

Task: `T-TMP-050`
Status: `done`

## Summary

Replaced the legacy-default tenant migration path with canonical nrg tenant ownership and removed active rollback-to-null migration tooling.

## Completed Work

- Changed tenant migration defaults from legacy-default to nrg.
- Removed active rollback-to-null migration mode and Make target.
- Changed configured table eligibility to tenant_id IS NULL, including rows with channel_id.
- Updated active runbook, onboarding examples, frontend bootstrap defaults, and backend/frontend tests to nrg.
- Recorded TMP-050 issue, work order, slice evidence, and handoff.

## Unfinished Work


## Next Tasks

- `TMP-051` — Tenant management admin list/update UI and API
- `TMP-052` — Tenant NOT NULL enforcement and nullable-path audit
