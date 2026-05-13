---
id: TMP-062
title: Dev loop fast restart
class: vertical_defect_slice
status: queued
scope_limit: "Optimize the local Makefile dev loop only; do not change service runtime code, dependency manifests, lockfiles, Go modules, schemas, or production Docker/deploy targets."
merge_policy: "Merge only after HVC, supervisor preflight, Makefile dry-run validation, and warm build/dependency-skip evidence pass."
evidence_required:
  - "make -n stop"
  - "make -n dev"
  - "make -n dev-landing"
  - "make -n dev-admin"
  - "Warm build target skip evidence"
acceptance_tests:
  - "hvc check agent/backlog/issues/TMP-062-dev-loop-fast-restart.md --fail-on block"
  - "make -n stop"
  - "make -n dev"
  - "make -n dev-landing"
  - "make -n dev-admin"
actor: developer
outcome: "Local `make stop && make dev` restarts the intended dev service set without stale processes, avoidable Go rebuilds, or avoidable npm installs."
entrypoint: Makefile dev and stop targets
trigger: "Developer runs `make stop && make dev` while iterating locally."
broken_outcome: "`make stop` leaves some dev services running, `make dev` rebuilds every Go service and reinstalls frontend dependencies unnecessarily, and services shift to higher ports."
expected_behavior: "`make stop` stops every service started by `make dev`; repeated `make dev` starts services through bounded parallelism and skips Go/npm work when outputs are current."
desired_outcome: "A repeated local dev restart stops the same service set that `make dev` starts, skips current builds and dependency installs, and starts services concurrently."
reproduction:
  command: "make stop && make dev"
  observed: "The stop target omits subscription-external, billing, and cadence-engine, while dev starts them; dev service targets also depend on phony build-local targets and landing always runs npm install."
  expected: "The stop target covers the full dev service set, warm build/dependency checks skip current work, and dev startup fan-out is bounded and parallel."
system_path:
  - "`make stop` stops local services and clears pid files."
  - "`make dev` builds or validates service binaries."
  - "`make dev` starts backend and frontend dev services."
change_layers:
  - dev-automation
verification_layers:
  - makefile
  - local-dev-workflow
blocked_by: []
blocks: []
parallel_group: dev-workflow
file_scope:
  allowed:
    - "Makefile"
    - "agent/backlog/issues/TMP-062-dev-loop-fast-restart.md"
    - "agent/state/TMP-062.work-order.json"
    - "agent/state/TMP-062.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/webspa-admin/package.json"
    - "frontend/webspa-admin/package-lock.json"
    - "services/landing-web/package.json"
    - "services/landing-web/package-lock.json"
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "services/**/cmd/**"
    - "services/**/internal/**"
    - "docker-compose*.yml"
---

## Reproduction

The pasted operator log shows `make stop` stopped only subscription, notification, acquisition-api, landing, and admin, while `make dev` then started subscription-external, subscription, billing, notification, acquisition-api, cadence-engine, landing, and admin. This leaves old subscription-external, billing, and cadence-engine processes alive, causes port drift, and forces unnecessary rebuild/install work on each iteration.

## Evidence commands

- `make -n stop`
- `make -n dev`
- `make -n dev-landing`
- `make -n dev-admin`
- `make build-local-subscription-external`

## Acceptance Criteria

- `make -n stop` shows subscription-external, subscription, billing, notification, acquisition-api, cadence-engine, landing, and admin stop targets.
- `make -n dev` shows a bounded parallel recursive make invocation for the dev service set.
- Warm build targets skip Go rebuilds when the output binary is newer than Go source and module files.
- Landing and admin dependency install checks skip npm install when node_modules is current.
- No package manifests, lockfiles, Go modules, or service source files are changed.
