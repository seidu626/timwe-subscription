# TMP-037 Spec

## Objective

Classify and track the dependency-change approval blocker. Do not change package manifests, lockfiles, frontend code, dependencies, or runtime behavior in this slice.

## Broken Behavior

npm audit reports Next/PostCSS advisories and npm audit fix proposes a breaking Next upgrade to next@16.2.6.

## Expected Behavior

Dependency upgrade scope, risk, and UI regression proof are approved before package manifests or lockfiles change.

## Acceptance Proof

```bash
jq empty slices/manifest.json agent/state/TMP-037.work-order.json agent/state/TMP-037.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
git diff --name-only
```

## Approval Gate

- Dependency changes required explicit user approval by repo policy.
- Approval was recorded on 2026-05-10 from the operator auto-proceed directive.
- The proposed remediation is breaking and requires UI regression verification in the implementation slice.
