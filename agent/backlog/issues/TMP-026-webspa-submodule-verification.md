---
id: TMP-026
title: "webspa-admin submodule verification"
class: operational_slice
status: done
scope_limit: "Verify webspa-admin checkout reproducibility and record the replacement strategy for the unreachable submodule commit."
merge_policy: "Merge only after HVC, supervisor preflight, tracked-source evidence, and available webspa-admin verification commands pass."
evidence_required:
  - "test -f frontend/webspa-admin/package.json"
  - "cd frontend/webspa-admin && npm test -- --watch=false --browsers=ChromeHeadless --progress=false"
  - "slices/TMP-026-webspa-submodule-verification/value-gate-report.md"
  - "slices/TMP-046-webspa-admin-reproducible-source/value-gate-report.md"
acceptance_tests:
  - "test ! -f .gitmodules"
  - "test -f frontend/webspa-admin/package.json"
  - "git ls-files -s frontend/webspa-admin/package.json"
  - "cd frontend/webspa-admin && npm run build"
  - "cd frontend/webspa-admin && npm test -- --watch=false --browsers=ChromeHeadless --progress=false"
actor: platform-operator
outcome: "Admin frontend verification has a precise submodule checkout decision instead of a stale missing-metadata blocker."
entrypoint: "frontend/webspa-admin gitlink"
trigger: "Operator runs full-system verification and admin UI checks."
broken_outcome: "`git submodule update --init --recursive frontend/webspa-admin` cannot fetch the pinned gitlink commit from the configured submodule remote."
expected_behavior: "frontend/webspa-admin is tracked source in the superproject and admin tests/build evidence can run without a submodule fetch."
system_path:
  - "Superproject checkout contains frontend/webspa-admin source files."
  - "No .gitmodules webspa-admin entry remains."
  - "Admin frontend verification command runs from the initialized checkout."
change_layers:
  - repo-metadata
  - frontend-source
  - frontend-verification
verification_layers:
  - harness
  - frontend-tests
blocked_by: []
blocks: ["TMP-021"]
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - ".gitmodules"
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-026-webspa-submodule-verification/**"
    - "frontend/webspa-admin/**"
    - "agent/backlog/issues/TMP-026-webspa-submodule-verification.md"
    - "agent/state/TMP-026.work-order.json"
    - ".agent/**"
  forbidden:
    - "common/**"
    - "services/**"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
---

## Operator Story

As a platform operator, I can verify whether the admin frontend submodule can be initialized from the superproject so full-system release checks do not hide the admin UI behind stale repository metadata assumptions.

## Acceptance Criteria

- `.gitmodules` no longer references the broken `frontend/webspa-admin` submodule.
- `frontend/webspa-admin/package.json` exists as tracked source.
- `npm run build` and the ChromeHeadless test command are run from tracked source.
- The original remote, pinned commit, and exact fetch error remain recorded in the value-gate history.
- Source changes are limited to replacing the unreproducible gitlink with the exact pinned local checkout source in TMP-046.
