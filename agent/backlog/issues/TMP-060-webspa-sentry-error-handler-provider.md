---
id: TMP-060
title: "webspa-admin Sentry error handler provider"
class: vertical_defect_slice
status: queued
scope_limit: "Fix the Angular admin startup crash caused by the Sentry ErrorHandler provider and keep the admin build reproducible without changing Sentry dependencies, lockfiles, admin feature behavior, backend services, or production deployment configuration."
merge_policy: "Merge only after HVC, targeted admin build/typecheck, and a dev-server startup smoke show the NullInjectorError is gone."
evidence_required:
  - "Browser/runtime error: NullInjectorError No provider for errorHandlerOptions"
  - "Sentry Angular package API evidence for createErrorHandler"
  - "cd frontend/webspa-admin && npm run build"
  - "WEBSPA_ADMIN_PORT=<free port> make dev-admin"
acceptance_tests:
  - "hvc check agent/backlog/issues/TMP-060-webspa-sentry-error-handler-provider.md --fail-on block"
  - "cd frontend/webspa-admin && npm run build"
  - "WEBSPA_ADMIN_PORT=<free port> make dev-admin"
actor: developer
outcome: "The Angular admin app boots without Sentry's ErrorHandler provider throwing a missing errorHandlerOptions injector error."
entrypoint: "frontend/webspa-admin/src/main.ts"
trigger: "Developer opens the Angular admin app after `make dev-admin` starts the dev server."
broken_outcome: "Angular startup fails with `NullInjectorError: No provider for errorHandlerOptions!` from SentryErrorHandler."
expected_behavior: "Sentry error handling is registered through the package-supported factory value so Angular does not need to resolve Sentry's internal options token, and the production build does not require network access to inline external font CSS."
reproduction:
  command: "Open http://localhost:4200/ after make dev-admin"
  observed: "main.ts logs `NullInjectorError: No provider for errorHandlerOptions!`."
  expected: "Angular app bootstraps without the Sentry provider injection error."
system_path:
  - "`main.ts` initializes Sentry."
  - "`bootstrapApplication` registers Angular providers."
  - "Angular resolves `ErrorHandler` during app startup."
change_layers:
  - frontend-bootstrap
  - frontend-build-config
verification_layers:
  - frontend-build
  - frontend-dev-server-smoke
blocked_by: []
blocks: []
parallel_group: dev-workflow
file_scope:
  allowed:
    - "frontend/webspa-admin/src/main.ts"
    - "frontend/webspa-admin/angular.json"
    - "agent/backlog/issues/TMP-060-webspa-sentry-error-handler-provider.md"
    - "agent/state/TMP-060.work-order.json"
    - "agent/state/TMP-060.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/webspa-admin/package.json"
    - "frontend/webspa-admin/package-lock.json"
    - "frontend/webspa-admin/src/app/features/**"
    - "services/**"
    - "common/**"
    - "go.mod"
    - "go.sum"
---

## Operator Story

As a developer, I can open the admin panel after `make dev-admin` without Angular failing during bootstrap because of Sentry provider wiring.

## Acceptance Criteria

- The Sentry `ErrorHandler` provider uses the package-supported factory value instead of injecting `SentryErrorHandler` as a class.
- The admin build succeeds.
- The production build does not fail when Angular cannot fetch Google Fonts for font CSS inlining.
- The admin dev-server startup smoke succeeds on a free port.
- No dependency manifests, lockfiles, backend services, admin feature modules, or production deployment files change.
