# TMP-025 Domain Brief

- Actor: platform-operator
- Business outcome: TMP-021's blocked release-verification state is visible and consistent across the issue, manifest, task state, and value-gate evidence.
- Domain invariant: a blocked release-verification parent must remain blocked until all child blockers are cleared; metadata reconciliation must not convert unresolved readiness risk into a pass.
- Entrypoint: `agent/backlog/issues/TMP-021-full-system-verification.md` and `slices/manifest.json`
- Trigger: Verifier finds TMP-021 metadata out of sync with the accepted release matrix blocker evidence.
- Risk: Parent release readiness can be misread if TMP-021 task state, manifest state, issue status, or value-gate verdict disagree.

## Story Craft

The story is concrete and testable: querying TMP-021 across the issue, manifest, task state, and value-gate report should show the same blocked outcome and cite the release matrix evidence.

## Roadmap To Slices

TMP-025 is an operational reconciliation slice under TMP-021. It preserves the parent blocker while aligning control-plane metadata.
