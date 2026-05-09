# TMP-040 Slice Spec

## Story

As a platform operator, I can see that the local pinned `webspa-admin` checkout builds and passes its headless tests while the clean superproject submodule path remains blocked, so release readiness separates code health from reproducibility.

## Scope

Allowed:
- Record evidence in the full-system matrix and TMP-021/TMP-026 value gates.
- Add TMP-040 issue, work order, handoff, manifest entry, and slice artifacts.
- Keep product source and submodule metadata unchanged.

Forbidden:
- Editing `frontend/**`.
- Repointing `.gitmodules` or the gitlink.
- Changing package manifests, lockfiles, schemas, compose files, runtime code, or branch integration state.

## Acceptance Proof

- `git -C frontend/webspa-admin rev-parse HEAD` in the primary checkout returns `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- `npm run build` passes in the local nested admin checkout.
- `CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false` passes 84/84 tests.
- `git submodule update --init --recursive frontend/webspa-admin` in a clean `origin/main` worktree still fails with `upload-pack: not our ref`.
- HVC, slice-harness, supervisor preflight, and JSON gates pass.

## Pass/Fail Criteria

Pass when evidence is recorded without changing frontend source or submodule metadata and the remaining clean-clone blocker is preserved.

Fail if the slice claims release readiness, hides the submodule reproducibility blocker, or changes product/runtime files.
