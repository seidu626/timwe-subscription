# Tenant Multi-Channel Platform Roadmap

This roadmap converts the current single-platform TIMWE subscription stack into a tenant-aware, multi-channel platform through vertical slices. It intentionally starts with a walking skeleton that proves tenant context end to end before expanding into acquisition, subscription, notification, cadence, postback, reporting, and migration hardening.

## Backbone

1. Platform operator creates a tenant -> tenant identity exists and can be selected by authenticated admin/API flows.
2. Tenant admin enters the tenant workspace -> existing admin data is scoped by tenant and cannot leak across tenants.
3. Tenant admin defines a channel -> channel capabilities and credential references are available for campaign/subscription routing.
4. Tenant admin binds a campaign to a tenant channel -> landing/acquisition can resolve the right tenant/channel.
5. End subscriber converts through a tenant campaign -> acquisition transaction, subscription, consent, and postback state share the same tenant context.
6. API-integrated partner sends channel-specific subscription/charge actions -> service-to-service calls route to the correct tenant/provider configuration.
7. MNO/TIMWE and workers emit notifications and cadence messages -> async flows preserve tenant/channel and remain idempotent.
8. Tenant admin reviews reports and operations -> dashboards, exports, DLQ, failures, and health are tenant/channel-filtered.
9. Platform operator migrates existing production data -> legacy global rows are backfilled safely into a default tenant and validated.

## Priority Order

- P0 walking skeleton: TMP-001, TMP-002, TMP-003
- P1 minimum useful tenant acquisition: TMP-004, TMP-005, TMP-006
- P2 full channel execution: TMP-007, TMP-008, TMP-009
- P3 production readiness: TMP-010, TMP-011

## Validation Rules

- Every implementation slice must preserve tenant isolation, auditability, credential secrecy, and existing single-tenant behavior.
- Every slice must include happy path, invalid input, missing required, duplicate/conflict where applicable, authorization, and dependency failure tests.
- No slice may introduce global fallback tenant behavior in protected paths.
- Any migration affecting existing tables must include rollback notes, backfill strategy, and production verification queries.
- Value-gate must PASS before slice-verify or release handoff.
