---
id: TMP-023
title: "Common package test failures"
class: vertical_defect_slice
status: ready
scope_limit: "Fix common package compile/test failures without changing dependency versions, vendor trees, or service behavior."
merge_policy: "Merge only after HVC and `go test ./...` pass in `common`."
evidence_required:
  - "cd common && go test ./..."
  - "slices/TMP-023-common-package-test-failures/value-gate-report.md"
acceptance_tests:
  - "cd common && go test ./..."
  - "test -f slices/TMP-023-common-package-test-failures/value-gate-report.md"
non_goals:
  - "No dependency updates."
  - "No vendor regeneration."
  - "No service API changes."
actor: platform-operator
outcome: "Shared common library tests pass so tenant auth and database helpers are reliable for dependent services."
entrypoint: "common package test suite"
trigger: "Operator runs `go test ./...` in `common`"
broken_outcome: "`go test ./...` in `common` fails with OpenAPI generator API drift, postgres test call signature drift, and trusted-service replay test failure."
expected_behavior: "`go test ./...` in `common` passes without dependency or vendor changes."
reproduction: "Run `cd common && go test ./...`; observe failures in `openApiGenerator.go`, `postgres/database_test.go`, and `auth/tenantctx` replay nonce test."
system_path:
  - "Common package compiles."
  - "Postgres tests call the current pool constructor contract."
  - "Trusted-service replay nonce tests use the same clock as the verifier."
change_layers:
  - common
  - tests
verification_layers:
  - tests
blocked_by: []
blocks: []
parallel_group: tenant-platform-defects
file_scope:
  allowed:
    - "common/openApiGenerator.go"
    - "common/postgres/database_test.go"
    - "common/auth/tenantctx/**"
    - "slices/TMP-023-common-package-test-failures/**"
    - "slices/manifest.json"
    - "agent/backlog/issues/TMP-023-common-package-test-failures.md"
    - "agent/state/TMP-023.work-order.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "common/go.mod"
    - "common/go.sum"
    - "common/vendor/**"
    - "services/**"
---

## Operator story

As a platform operator, I can rely on the shared common package tests before trusting services that depend on tenant auth and database helpers.

## Acceptance criteria

- `cd common && go test ./...` passes.
- OpenAPI generator helper no longer breaks normal common package builds.
- Postgres tests call the current `NewPGXPool` interface.
- Trusted-service replay nonce test rejects the second use deterministically.
