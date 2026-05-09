# TMP-040 Domain Brief

## Actor

Platform operator responsible for release verification.

## Business Outcome

The operator can distinguish admin UI code-health evidence from admin source reproducibility evidence before making a release decision.

## Domain Invariant

A passing local admin build is not enough for release readiness unless the source checkout can be reproduced from the superproject gitlink.

## Entrypoint

`frontend/webspa-admin` as both a superproject gitlink in clean `origin/main` worktrees and a nested local repository in the primary checkout.

## Trigger

The full-system verifier finds the local nested admin checkout at the pinned gitlink SHA after supervisor reports no ready tasks.

## Risk

If local-only build/test success is treated as release proof, clean clones and CI can still fail to fetch the admin UI. If the local result is ignored, a useful signal about the pinned admin code health is lost.
