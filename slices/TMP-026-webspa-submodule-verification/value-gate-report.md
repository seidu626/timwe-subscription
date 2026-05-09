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

## Audit 3: Local Nested Checkout Verification

- The primary checkout has a nested `frontend/webspa-admin` repository at the pinned gitlink commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- `npm run build` passed in that local nested checkout with existing Angular budget/selector warnings only.
- `CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false` passed with `TOTAL: 84 SUCCESS`.
- Environment observed: Node `v24.15.0`, npm `11.12.1`, Google Chrome `148.0.7778.96`. Angular CLI reports Node 24 as unsupported.

Audit 3 result: PASS for local pinned checkout code-health evidence, still BLOCKED for clean-clone reproducibility.

## Blocking Gate

The admin frontend local nested checkout can build and test, but the superproject cannot be reproduced from a clean `origin/main` checkout because `git submodule update --init --recursive frontend/webspa-admin` still fails with `upload-pack: not our ref 2ad95b18ecff4d8b23e5d1b7152975c477d5137a`. This requires one of these operator decisions before admin UI source checkout can be treated as release/CI ready:

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
git -C /home/xper626/workspace/apps/timwe-subscription/frontend/webspa-admin rev-parse HEAD
cd /home/xper626/workspace/apps/timwe-subscription/frontend/webspa-admin && npm run build
cd /home/xper626/workspace/apps/timwe-subscription/frontend/webspa-admin && CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false
```

Result: PASS for local nested checkout build/test evidence; BLOCKED by unavailable clean submodule checkout.
