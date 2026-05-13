---
id: TMP-059
title: "Admin dev startup latency"
class: vertical_defect_slice
status: queued
scope_limit: "Reduce repeated local `make dev` wait time for the Angular admin panel by fixing the admin dev startup automation without changing admin runtime behavior, dependencies, backend services, or production builds."
merge_policy: "Merge only after baseline/after timing evidence, HVC, supervisor preflight, Makefile syntax validation, and admin dev startup smoke evidence pass."
evidence_required:
  - "Baseline timing for the current admin dependency/start command path"
  - "After timing for the patched admin dependency/start command path"
  - "make -n dev-admin"
  - "WEBSPA_ADMIN_PORT=<free port> make dev-admin"
acceptance_tests:
  - "hvc check agent/backlog/issues/TMP-059-admin-dev-startup-latency.md --fail-on block"
  - "make -n dev-admin"
  - "WEBSPA_ADMIN_PORT=<free port> make dev-admin"
actor: developer
outcome: "Local `make dev` reaches the Angular admin server without paying an npm dependency check on every run when dependencies are already installed."
entrypoint: "Makefile dev-admin target"
trigger: "Developer runs `make dev` or `make dev-admin` in a checkout with existing webspa-admin dependencies."
broken_outcome: "`make dev` appears to hang at 'Starting Admin Panel (Angular)...' because the admin target runs npm install before Angular serve every time."
expected_behavior: "The admin target validates dependencies cheaply when node_modules is already present and only installs when dependencies are missing or stale."
reproduction:
  command: "time (cd frontend/webspa-admin && npm install --silent)"
  observed: "The admin startup path runs npm dependency setup before Angular serve, even when node_modules already exists."
  expected: "A repeated dev startup with current dependencies skips npm install and goes straight to Angular serve."
system_path:
  - "`make dev` reaches the `dev-admin` target after backend and landing services start."
  - "`dev-admin` prepares webspa-admin dependencies."
  - "`dev-admin` launches Angular dev server on the configured port and waits for bind evidence."
change_layers:
  - dev-automation
verification_layers:
  - makefile
  - frontend-dev-server-smoke
blocked_by: []
blocks: []
parallel_group: dev-workflow
file_scope:
  allowed:
    - "Makefile"
    - "agent/backlog/issues/TMP-059-admin-dev-startup-latency.md"
    - "agent/state/TMP-059.work-order.json"
    - "agent/state/TMP-059.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-059-admin-dev-startup-latency/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/webspa-admin/package.json"
    - "frontend/webspa-admin/package-lock.json"
    - "frontend/webspa-admin/src/**"
    - "services/**"
    - "common/**"
    - "ops/**"
    - "go.mod"
    - "go.sum"
    - "docker-compose*.yml"
---

## Operator Story

As a developer, I can run `make dev` repeatedly without the Angular admin step spending avoidable time in npm dependency setup when the checkout is already installed.

## Acceptance Criteria

- Baseline evidence identifies whether the slow portion is npm dependency setup, Angular serve compilation, or port detection.
- `dev-admin` skips dependency installation when `frontend/webspa-admin/node_modules/.package-lock.json` is at least as new as `frontend/webspa-admin/package-lock.json`.
- `dev-admin` still runs `npm install` when dependencies are missing or stale.
- The Angular dev server starts on the configured admin port after the patch.
- No admin source, dependency manifest, lockfile, backend service, schema, or production build behavior changes.
