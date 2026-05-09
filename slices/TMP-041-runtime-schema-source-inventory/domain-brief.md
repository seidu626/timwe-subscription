# TMP-041 Domain Brief

## Actor

Platform operator responsible for release verification and local runtime readiness.

## Business Outcome

The operator can decide on migration provisioning with exact schema-source evidence instead of treating runtime missing-relation failures as unknown product defects.

## Domain Invariant

Runtime readiness requires both schema definitions and an approved way to apply them to the compose database. Existing SQL files do not prove the runtime database is provisioned.

## Entrypoint

TMP-034, TMP-035, and TMP-036 blocker evidence plus the full-system verification matrix.

## Trigger

The full-system verifier audits schema-related blocked slices after supervisor reports no ready tasks.

## Risk

Applying SQL or compose changes without approval can mutate runtime state and hide migration-order defects. Failing to record the existing SQL sources makes the next decision broader than needed.
