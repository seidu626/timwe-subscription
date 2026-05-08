# Tenant Multi-Channel Platform Roadmap

This roadmap converts the current single-platform TIMWE subscription stack into a tenant-aware, multi-channel platform through vertical slices. It intentionally starts with a walking skeleton that proves tenant context end to end before expanding into acquisition, subscription, notification, cadence, postback, reporting, and migration hardening.

## Backbone

1. Platform operator defines tenant claim, role, trust-boundary, and service-auth contract -> every service has one typed tenant context.
2. Platform operator creates a tenant -> tenant identity exists and can be selected by authenticated admin/API flows.
3. Operations analyst sees safe tenant/channel context in logs, metrics, and worker health -> observability is ready before tenant work fans out.
4. Tenant admin enters the tenant workspace -> existing admin data is scoped by tenant and cannot leak across tenants.
5. Tenant admin defines a channel -> channel capabilities and credential references are available for campaign/subscription routing.
6. Campaign operator uploads tenant-scoped assets -> landing media and signed URLs cannot cross tenants.
7. Tenant admin binds a campaign to a tenant channel -> landing/acquisition can resolve the right tenant/channel.
8. End subscriber converts through a tenant campaign -> acquisition transaction, subscription, consent, and postback state share the same tenant context.
9. Public subscriber, gateway, HE, and callback routes resolve tenant deterministically -> overlapping campaign slugs and trusted headers cannot route to the wrong tenant.
10. API-integrated partner sends channel-specific subscription/charge actions -> service-to-service calls route to the correct tenant/provider configuration.
11. TIMWE/MNO, partner callbacks, and workers emit events -> inbound and async flows preserve tenant/channel and remain idempotent.
12. Tenant admin uses the admin portal workspace -> UI route guards and API error handling align with backend tenant enforcement.
13. Tenant admin reviews reports and operations -> dashboards, exports, DLQ, failures, charge state, and health are tenant/channel-filtered.
14. Platform operator hardens secrets, config, contracts, and observability -> tenant channels are operable and partner onboarding is repeatable.
15. Platform operator migrates existing production data -> legacy global rows are backfilled safely into a default tenant and validated.

## Priority Order

- P0 walking skeleton: TMP-018, TMP-001, TMP-020, TMP-002, TMP-003
- P1 minimum useful tenant acquisition: TMP-004, TMP-019, TMP-005, TMP-012, TMP-006
- P2 full channel execution: TMP-007, TMP-017, TMP-013, TMP-008, TMP-009, TMP-014
- P3 production readiness: TMP-010, TMP-015, TMP-016, TMP-011

## Decision Gates

- Tenant source of truth and isolation model must be decided before TMP-001 implementation expands beyond a walking skeleton.
- Tenant claim, role, and service-to-service auth contract must be decided in TMP-018 before protected tenant APIs are implemented.
- Public tenant routing policy must be decided before TMP-012 and before allowing duplicate public campaign slugs.
- Secret backend and credential reference shape must be decided before TMP-004 can pass value-gate.
- Asset namespace strategy must be decided before TMP-019 and before tenant campaigns can serve public media.
- Charge ownership must be decided in TMP-017 before TMP-010 reporting and TMP-015 operations can claim production readiness.
- Admin portal ownership for `frontend/webspa-admin` must be confirmed before TMP-014 starts.

## Validation Rules

- Every implementation slice must preserve tenant isolation, auditability, credential secrecy, and existing single-tenant behavior.
- Every slice must include happy path, invalid input, missing required, duplicate/conflict where applicable, authorization, and dependency failure tests.
- No slice may introduce global fallback tenant behavior in protected paths.
- Public routes may use default-tenant compatibility only when the legacy route maps uniquely and is covered by tests.
- Callback and worker slices must include replay/idempotency and quarantine/reject behavior for uncorrelatable events.
- Any migration affecting existing tables must include rollback notes, backfill strategy, and production verification queries.
- Value-gate must PASS before slice-verify or release handoff.
