---
id: TMP-064
title: Admin Sentry Angular 18 peer dependency
class: vertical_defect_slice
status: queued
scope_limit: "Fix only the webspa-admin npm dependency conflict caused by @sentry/angular-ivy on Angular 18; do not change backend code, CORS behavior, tenant logic, service schemas, migrations, or unrelated frontend features."
merge_policy: "Merge only after HVC, npm install without legacy peer flags, lockfile update, and Angular build/type verification pass."
evidence_required:
  - "npm install without --legacy-peer-deps"
  - "npm run build"
  - "hvc check agent/backlog/issues/TMP-064-admin-sentry-angular18-peer.md --fail-on block"
acceptance_tests:
  - "hvc check agent/backlog/issues/TMP-064-admin-sentry-angular18-peer.md --fail-on block"
  - "cd frontend/webspa-admin && npm install --ignore-scripts"
  - "cd frontend/webspa-admin && npm run build"
actor: developer
outcome: "The admin panel dependencies install cleanly under Angular 18 without forcing or bypassing npm peer dependency resolution."
entrypoint: "cd frontend/webspa-admin && npm install"
trigger: "Developer starts or installs the admin panel dependencies."
broken_outcome: "npm ERESOLVE fails because @sentry/angular-ivy@7.120.4 peers @angular/common >=12 <=17 while the app uses Angular 18.2.9."
expected_behavior: "The admin panel uses a Sentry Angular package whose peer dependency range supports Angular 18."
desired_outcome: "npm install resolves normally, the lockfile records the compatible Sentry package, and the admin Angular build still compiles."
reproduction:
  command: "cd frontend/webspa-admin && npm install"
  observed: "ERESOLVE could not resolve @sentry/angular-ivy@7.120.4 peer @angular/common >=12 <=17 against @angular/common@18.2.9."
  expected: "npm install completes without --force or --legacy-peer-deps."
system_path:
  - "frontend/webspa-admin/package.json declares the Sentry Angular dependency."
  - "frontend/webspa-admin/src/main.ts initializes Sentry during standalone Angular bootstrap."
  - "frontend/webspa-admin/package-lock.json records dependency resolution."
change_layers:
  - webspa-admin
  - dependency-resolution
verification_layers:
  - npm-install
  - angular-build
blocked_by: []
blocks: []
parallel_group: admin-deps
file_scope:
  allowed:
    - "frontend/webspa-admin/package.json"
    - "frontend/webspa-admin/package-lock.json"
    - "frontend/webspa-admin/src/main.ts"
    - "frontend/webspa-admin/src/app/app.module.ts"
    - "agent/backlog/issues/TMP-064-admin-sentry-angular18-peer.md"
    - "agent/state/TMP-064.work-order.json"
    - "agent/state/TMP-064.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**"
    - "frontend/webspa-admin/src/app/features/**"
    - "frontend/webspa-admin/src/app/core/http-interceptors/**"
    - "frontend/webspa-admin/angular.json"
    - "docker-compose*.yml"
---

## Reproduction

`npm install` fails with:

`peer @angular/common@">= 12.x <= 17.x" from @sentry/angular-ivy@7.120.4`

The root project uses:

`@angular/common@18.2.9`

## Acceptance Criteria

- `@sentry/angular-ivy` is removed from the admin dependency graph.
- The replacement Sentry Angular package has a peer range that supports Angular 18.
- Existing Sentry initialization in standalone bootstrap remains wired.
- `npm install` succeeds without `--force` or `--legacy-peer-deps`.
- Admin Angular build succeeds.
