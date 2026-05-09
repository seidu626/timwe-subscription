# TMP-040 Value Gate Report

- Timestamp: 2026-05-09T05:55:33Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:done

## Domain Grounding

- Actor: platform-operator.
- Business outcome: release evidence separates local admin UI code health from clean-clone source reproducibility.
- Domain invariant: a local nested checkout can build and test while the superproject remains unreproducible from its configured gitlink.
- Entrypoint: `frontend/webspa-admin` gitlink and primary checkout nested repository.
- Risk: treating local-only verification as release readiness would hide a source distribution blocker.

## Story Craft

As a platform operator, I can see that the local pinned admin checkout builds and passes tests, while a clean `origin/main` worktree still cannot initialize the submodule, so release readiness does not collapse two different signals into one vague blocker.

## Acceptance Results

| Criterion | Result | Evidence |
|---|---|---|
| Local nested admin checkout is at the pinned gitlink SHA | PASS | `git -C frontend/webspa-admin rev-parse HEAD` returned `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`. |
| Local admin build passes | PASS | `npm run build` completed successfully in `/home/xper626/workspace/apps/timwe-subscription/frontend/webspa-admin`; Angular emitted existing budget/selector warnings only. |
| Local admin headless tests pass | PASS | `CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false` completed `TOTAL: 84 SUCCESS`. |
| Clean superproject submodule initialization remains blocked | BLOCKED | `git submodule update --init --recursive frontend/webspa-admin` in clean `origin/main` worktree failed with `upload-pack: not our ref 2ad95b18ecff4d8b23e5d1b7152975c477d5137a`. |
| No frontend/source/runtime files changed | PASS | Slice changes are metadata and evidence only; `frontend/**` remains forbidden. |

## Environment

- Node: `v24.15.0`.
- npm: `11.12.1`.
- Chrome: `Google Chrome 148.0.7778.96`.
- Angular CLI reported Node 24 as unsupported, but the build and test commands completed successfully in this local environment.

## Remaining Gate

TMP-026 remains blocked for clean clones and CI until the pinned admin commit is published to an accessible submodule remote, the gitlink is repointed to an accessible commit, or the repository strategy is replaced.

## Commands

```bash
git -C frontend/webspa-admin rev-parse HEAD
git -C frontend/webspa-admin status --short --branch --untracked-files=all
git -C frontend/webspa-admin log -1 --oneline --decorate --stat
node --version
npm --version
/usr/bin/google-chrome-stable --version
npm run build
CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false
git submodule update --init --recursive frontend/webspa-admin
```

Result: PASS for local admin code-health evidence, BLOCKED for reproducible clean-clone submodule initialization.
