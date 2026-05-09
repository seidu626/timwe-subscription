# TMP-026 Value Gate Report

- Timestamp: 2026-05-09T01:58:30Z
- Agent: Codex
- Verdict: BLOCKED
- Outcome code: outcome:blocked

## Audit 1: Submodule Metadata

- `.gitmodules` is present and tracked: COVERED.
- `frontend/webspa-admin` maps to `https://github.com/coreui/coreui-free-angular-admin-template.git`: COVERED.
- The superproject gitlink pins `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`: COVERED.

Audit 1 result: PASS for metadata presence.

## Audit 2: Checkout Verification

- `git submodule update --init --recursive frontend/webspa-admin` was run.
- The command failed with `fatal: remote error: upload-pack: not our ref 2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- The command also reported that the submodule fetch did not contain the pinned commit and direct fetching failed.
- `git submodule status --recursive frontend/webspa-admin` showed the deinitialized tracked gitlink as `-2ad95b18ecff4d8b23e5d1b7152975c477d5137a frontend/webspa-admin`.

Audit 2 result: BLOCKED.

## Blocking Gate

The admin frontend cannot be initialized from the configured submodule remote because the pinned gitlink commit is unavailable there. This requires one of these operator decisions before admin UI build/test evidence can run:

- publish or move the `2ad95b18ecff4d8b23e5d1b7152975c477d5137a` admin commit to an accessible remote and update submodule metadata if needed;
- repoint the superproject gitlink to a commit available from the configured remote after accepting any feature loss or replacement;
- replace the gitlink strategy with tracked source or a different repository strategy.

## Commands

```bash
sed -n '1,60p' .gitmodules
git ls-files -s frontend/webspa-admin .gitmodules
git submodule update --init --recursive frontend/webspa-admin
git submodule deinit -f frontend/webspa-admin
git submodule status --recursive frontend/webspa-admin
git status --short --branch
```

Result: BLOCKED by unavailable pinned submodule commit.

