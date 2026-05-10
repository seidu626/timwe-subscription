# TMP-026 Decision Template: webspa-admin Source Reproducibility

Status: proposed

Approval recorded: no

## Context

`frontend/webspa-admin` is a gitlink pinned to `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`. The configured remote, `https://github.com/coreui/coreui-free-angular-admin-template.git`, does not provide that commit, so clean superproject initialization fails.

Local evidence says the pinned checkout builds and passes ChromeHeadless tests, but that does not make clean clone or CI reproducibility pass.

## Decision Required

Choose one path before implementation:

- Publish or move the pinned commit to an accessible remote and keep the gitlink.
- Repoint the gitlink to a commit available from the configured remote after review.
- Replace the gitlink strategy with tracked source or a different repository strategy.

## Decision

Pending operator decision.

## Consequences To Review

- Source reproducibility for clean clone and CI.
- Feature loss or replacement risk if repointing.
- Repository size and ownership if replacing the gitlink strategy.
- Required admin UI build/test evidence after the chosen path.

## Post-Decision Proof

```bash
git submodule update --init --recursive frontend/webspa-admin
git submodule status --recursive frontend/webspa-admin
cd frontend/webspa-admin && npm run build
cd frontend/webspa-admin && CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false
```

## Slice Impact

- Blocks: `TMP-021`, `TMP-026`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-026.handoff.json`
