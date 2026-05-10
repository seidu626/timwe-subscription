# TMP-046 Value Gate Report

Timestamp: 2026-05-10T05:20:00Z  
Agent: codex  
Verdict: PASS

## Audit 1: Acceptance Criteria Coverage

- Criterion: `.gitmodules` no longer references broken webspa-admin submodule.
  - Evidence: `.gitmodules` removed; `test ! -f .gitmodules` passes.
  - Verdict: COVERED
- Criterion: `frontend/webspa-admin/package.json` is tracked source.
  - Evidence: `git ls-files -s frontend/webspa-admin/package.json` shows normal `100644` file mode, not a `160000` gitlink.
  - Verdict: COVERED
- Criterion: Admin dependencies install from lockfile.
  - Evidence: `npm ci` added 1007 packages and completed.
  - Verdict: COVERED
- Criterion: Admin build passes.
  - Evidence: `npm run build` completed; existing SCSS budget and selector warnings only.
  - Verdict: COVERED
- Criterion: Admin tests pass.
  - Evidence: `CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false` reported `TOTAL: 84 SUCCESS`.
  - Verdict: COVERED

## Audit 2: Failure Mode Coverage

- Missing submodule commit: resolved by deleting the gitlink and tracking source directly.
- Feature loss from repointing to upstream CoreUI: avoided by materializing the local pinned checkout that contains tenant workspace guardrails.
- Dependency warnings: recorded but not remediated here; TMP-037 remains the dependency remediation approval surface.

Verdict: PASS

## Audit 3: Domain Invariant Preservation

- Tenant admin source is preserved: `tenant-workspace.guard`, tenant workspace service, interceptor, and page403 source exist in tracked source.
- One canonical source path: preserved by removing `.gitmodules` and the `160000` gitlink.
- Verifier entrypoint is stable: `frontend/webspa-admin/package.json` exists immediately after checkout.

Verdict: PASS

## Audit 4: User Journey

1. Platform operator checks out the superproject.
2. Admin source exists at `frontend/webspa-admin`.
3. Verifier installs dependencies, builds, and runs ChromeHeadless tests.
4. Full-system admin UI verification is no longer blocked by `upload-pack: not our ref`.

Verdict: PASS

## Audit 5: Test Quality

Commands run:

```bash
test ! -f .gitmodules
test -f frontend/webspa-admin/package.json
git ls-files -s frontend/webspa-admin/package.json
cd frontend/webspa-admin && npm ci
cd frontend/webspa-admin && npm run build
cd frontend/webspa-admin && CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false
```

Verdict: PASS

## Remaining Risks

- `npm ci` reports 79 vulnerabilities and Node `v24.15.0` is outside the package engine range. This was already a release blocker under dependency remediation and is not changed by this slice.
