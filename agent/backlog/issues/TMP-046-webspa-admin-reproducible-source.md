---
id: TMP-046
title: "webspa-admin reproducible source checkout"
class: vertical_defect_slice
status: done
scope_limit: "Replace the unreproducible webspa-admin gitlink with the exact tracked source from the pinned local checkout so clean superproject clones can build and test the tenant admin UI without fetching an unreachable submodule commit."
merge_policy: "Merge only after admin source checkout evidence, npm install/build/test evidence, HVC, supervisor preflight, JSON validity, and value-gate evidence pass."
evidence_required:
  - "frontend/webspa-admin/package.json"
  - "slices/TMP-046-webspa-admin-reproducible-source/value-gate-report.md"
  - "slices/TMP-026-webspa-submodule-verification/value-gate-report.md"
acceptance_tests:
  - "test ! -f .gitmodules"
  - "test -f frontend/webspa-admin/package.json"
  - "git ls-files -s frontend/webspa-admin | head"
  - "cd frontend/webspa-admin && npm ci"
  - "cd frontend/webspa-admin && npm run build"
  - "cd frontend/webspa-admin && CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false"
actor: platform-operator
outcome: "Admin frontend verification no longer depends on an unavailable submodule commit while preserving the tenant admin source that passed local verification."
entrypoint: "frontend/webspa-admin tracked source directory"
trigger: "Verifier tries to reproduce the admin frontend from a clean superproject checkout."
broken_outcome: "The superproject gitlink points at local commit 2ad95b18ecff4d8b23e5d1b7152975c477d5137a, but the configured public CoreUI remote does not advertise that commit."
expected_behavior: "The superproject tracks the admin source directly, package.json exists after checkout, and build/test run without submodule fetch."
reproduction:
  command: "git submodule update --init --recursive frontend/webspa-admin"
  observed: "fatal: remote error: upload-pack: not our ref 2ad95b18ecff4d8b23e5d1b7152975c477d5137a"
  expected: "frontend/webspa-admin source is present and verifiable after checking out the superproject."
system_path:
  - "Superproject checkout includes frontend/webspa-admin source files."
  - "Node dependencies install from the tracked lockfile."
  - "Angular build and ChromeHeadless tests run from tracked source."
change_layers:
  - frontend-source
  - repo-metadata
  - evidence
verification_layers:
  - frontend-build
  - frontend-tests
  - metadata
blocked_by: []
blocks:
  - "TMP-021"
  - "TMP-026"
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - ".gitmodules"
    - "frontend/webspa-admin/**"
    - "agent/backlog/issues/TMP-026-webspa-submodule-verification.md"
    - "agent/backlog/issues/TMP-046-webspa-admin-reproducible-source.md"
    - "agent/state/TMP-026.work-order.json"
    - "agent/state/TMP-046.work-order.json"
    - "agent/state/TMP-046.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-026-webspa-submodule-verification/**"
    - "slices/TMP-046-webspa-admin-reproducible-source/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**"
    - "common/**"
    - "ops/**"
    - "docker-compose*.yml"
    - "Makefile"
    - "go.mod"
    - "go.sum"
---

## Operator Story

As a platform operator, I can verify the admin frontend from the superproject checkout without relying on an unreachable submodule commit, so full-system release checks can include the tenant admin UI.

## Acceptance Criteria

- `.gitmodules` no longer references the broken `frontend/webspa-admin` submodule.
- `frontend/webspa-admin/package.json` is present as tracked source.
- Admin dependencies install from `package-lock.json`.
- `npm run build` passes from `frontend/webspa-admin`.
- ChromeHeadless test suite passes from `frontend/webspa-admin`.
- TMP-026 blocker evidence is reconciled to point at the reproducible tracked-source strategy.
