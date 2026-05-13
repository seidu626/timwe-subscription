# Session Capsule: codex-dev-loop-fast-20260513-105610

Task: `TMP-062`
Status: `done`

## Summary

Optimized the local dev restart loop by aligning stop coverage with the full dev service set, starting dev targets through bounded parallel make fan-out, skipping current Go rebuilds, and skipping current Landing Web dependency installs.

## Completed Work

- Created TMP-062 classified defect issue and work order.
- Identified root cause: make stop omitted subscription-external, billing, and cadence-engine while make dev started them, causing stale processes and port drift.
- Changed make stop and stop-all to stop the full dev service set in parallel.
- Changed make dev and dev-all to use bounded parallel recursive make fan-out.
- Added a reusable Makefile Go binary freshness check for local dev build targets.
- Changed dev-landing to skip npm install when dependencies are current.

## Unfinished Work


## Next Tasks

