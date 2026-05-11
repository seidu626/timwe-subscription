# TMP-056 Spec: Acquisition Postback Tenant Routing Bootstrap

## Story

As a platform operator, I want acquisition-api startup to apply the acquisition-owned postback tenant-routing migration before the in-process dispatcher polls, so the dispatcher does not fail on missing `postback_outbox` tenant columns after admin schema bootstrap completes.

## Scope

In scope:
- Add the canonical acquisition-owned tenant postback routing migration to the service-local startup bootstrap sequence.
- Add repository tests for the bootstrap migration list and the migration columns required by `PostbackRepository`.
- Record value-gate evidence for the defect fix.

Out of scope:
- Remote database mutation.
- Subscription-external postback DDL.
- Compatibility or fallback postback query paths.

## Acceptance Criteria

1. `defaultAdminManagementSchemaPaths` includes `migrations/add_tenant_postback_routing.sql`.
2. Tests verify the migration adds `tenant_id`, `channel_id`, and `failure_reason` to `postback_outbox`.
3. The fix preserves the single canonical postback path: acquisition-api migration ownership only.
4. `cd services/acquisition-api && go test ./internal/repository` passes.

## Architecture Notes

This is a small deepening at the startup bootstrap module. The interface remains `AdminManagementRepository.EnsureSchema(ctx, "")`; the implementation now hides all schema prerequisites needed before acquisition-api starts tenant-aware runtime modules. No fallback branch is added, so there is still one canonical postback path.
