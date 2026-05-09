---
id: TMP-040
title: "webspa-admin local checkout verification evidence"
class: operational_slice
status: done
scope_limit: "Record local verification evidence for the pinned webspa-admin checkout while preserving the clean-clone submodule blocker. Do not change frontend source, submodule metadata, runtime behavior, schemas, compose files, dependencies, package manifests, or branch state."
merge_policy: "Merge only after HVC, slice-harness, supervisor preflight, JSON validity, value-gate evidence, and file-scope checks pass."
evidence_required:
  - "git -C frontend/webspa-admin rev-parse HEAD"
  - "npm run build in the local nested frontend/webspa-admin checkout"
  - "CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false in the local nested frontend/webspa-admin checkout"
  - "git submodule update --init --recursive frontend/webspa-admin in a clean origin/main worktree"
  - "slices/TMP-040-webspa-local-checkout-verification/value-gate-report.md"
acceptance_tests:
  - "test -f slices/TMP-040-webspa-local-checkout-verification/value-gate-report.md"
  - "jq empty slices/manifest.json agent/state/TMP-040.work-order.json agent/state/TMP-040.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Release-verification evidence distinguishes local admin UI build/test health from clean-clone submodule reproducibility."
entrypoint: "frontend/webspa-admin gitlink and local nested checkout"
trigger: "Verifier discovers the primary checkout has a nested webspa-admin repository at the pinned gitlink SHA."
broken_outcome: "The release matrix only says admin verification is unavailable, even though the local pinned checkout can be built and tested; clean clones still cannot initialize that gitlink."
expected_behavior: "The release matrix records both facts: local pinned checkout build/test evidence passes, and source-of-truth clean submodule initialization remains blocked until the gitlink is published, repointed, or replaced."
system_path:
  - "Verifier inspects the superproject gitlink."
  - "Verifier checks the local nested admin repository at the pinned SHA."
  - "Verifier runs local admin build and headless tests."
  - "Verifier reruns clean worktree submodule initialization and records the remaining reproducibility blocker."
change_layers:
  - evidence
  - harness
verification_layers:
  - control-plane
  - frontend-tests
  - metadata
blocked_by: []
blocks: []
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-040-webspa-local-checkout-verification.md"
    - "agent/state/TMP-040.work-order.json"
    - "agent/state/TMP-040.handoff.json"
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-026-webspa-submodule-verification/value-gate-report.md"
    - "slices/TMP-040-webspa-local-checkout-verification/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "frontend/**"
    - "services/**"
    - "common/**"
    - "ops/**"
    - "docker-compose*.yml"
    - "Makefile"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
    - ".git/**"
---

## Operator Story

As a platform operator, I can separate admin UI code-health evidence from admin submodule reproducibility evidence so release readiness does not hide either the passing local build/test signal or the remaining clean-clone blocker.

## Acceptance Criteria

- The local nested `frontend/webspa-admin` checkout is confirmed to be at gitlink `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- Local admin `npm run build` and ChromeHeadless test evidence are recorded with observed environment details.
- Clean `origin/main` submodule initialization is rerun and still records the upload-pack blocker.
- TMP-021/TMP-026 evidence is updated to distinguish local checkout health from reproducible source checkout readiness.
- No frontend source, submodule metadata, package, dependency, schema, compose, runtime, or branch-integration files change.
