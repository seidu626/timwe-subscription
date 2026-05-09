---
id: TMP-026
title: "webspa-admin submodule verification"
class: operational_slice
status: blocked
scope_limit: "Verify webspa-admin submodule metadata, prove whether the pinned checkout can be initialized, and record the exact blocker if it cannot."
merge_policy: "Merge only after submodule status, HVC, slice-harness, and available webspa-admin verification commands pass or record a concrete blocker."
evidence_required:
  - "git submodule status --recursive frontend/webspa-admin"
  - "test -f frontend/webspa-admin/package.json"
  - "cd frontend/webspa-admin && npm test -- --watch=false --browsers=ChromeHeadless --progress=false"
  - "slices/TMP-026-webspa-submodule-verification/value-gate-report.md"
acceptance_tests:
  - "git submodule update --init --recursive frontend/webspa-admin"
  - "git submodule status --recursive frontend/webspa-admin"
  - "test -f frontend/webspa-admin/package.json"
  - "cd frontend/webspa-admin && npm test -- --watch=false --browsers=ChromeHeadless --progress=false"
actor: platform-operator
outcome: "Admin frontend verification has a precise submodule checkout decision instead of a stale missing-metadata blocker."
entrypoint: "frontend/webspa-admin gitlink"
trigger: "Operator runs full-system verification and admin UI checks."
broken_outcome: "`git submodule update --init --recursive frontend/webspa-admin` cannot fetch the pinned gitlink commit from the configured submodule remote."
expected_behavior: "frontend/webspa-admin initializes to the tracked gitlink commit and admin tests/build evidence can run."
system_path:
  - "Superproject reads .gitmodules."
  - "Submodule URL resolves for frontend/webspa-admin."
  - "Submodule checkout reaches the gitlink commit."
  - "Admin frontend verification command runs from the initialized checkout."
change_layers:
  - repo-metadata
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
    - "agent/backlog/issues/TMP-026-webspa-submodule-verification.md"
    - "agent/state/TMP-026.work-order.json"
    - ".agent/**"
  forbidden:
    - "frontend/webspa-admin/**"
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

- `.gitmodules` contains the tracked `frontend/webspa-admin` path and URL.
- `git submodule update --init --recursive frontend/webspa-admin` is run and recorded.
- `git submodule status --recursive frontend/webspa-admin` is run and recorded.
- If initialization succeeds, `frontend/webspa-admin/package.json` exists and the available admin verification command is run.
- If initialization fails, the remote, pinned commit, and exact fetch error are recorded.
- No source files inside `frontend/webspa-admin` are edited by this slice.
