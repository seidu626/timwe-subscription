# Tenant Multi-Channel Platform Intake

This package is the planning intake for extending the current TIMWE subscription/acquisition repository into a tenant-aware, multi-channel platform.

Start here:

1. `existing-feature-service-inventory.md` — current services, features, and platform gaps.
2. `TMP-000-domain-grounding/domain-brief.md` — actors, ubiquitous language, invariants, failure modes, journeys, and decisions.
3. `roadmap.md` — implementation backbone, priority order, decision gates, and validation rules.
4. `work-required-index.md` — repository-layer work map for services, gateway, UI, workers, docs, ops, and migrations.
5. `manifest.json` — machine-readable slice list, dependencies, actors, entrypoints, and risk coverage.
6. `TMP-*/slice.yaml` — story-craft slice specs with acceptance criteria, invariants, scope, and value-gate target.
7. `value-gate-plan.md` — required proof before any slice is considered complete.
8. `plan-critique-and-amendments.md` — critique of the first pass and amendments applied.
9. `claude-async-review-synthesis.md` — independent Claude review findings that were integrated.

Current slice count: 20.

Use dependency order from `roadmap.md` and `manifest.json`; slice IDs are stable identifiers, not strict execution order.

Priority path:

- P0: tenant claim/auth contract, tenant context, tenant observability baseline, tenant admin scope, channel catalog.
- P1: credentials, tenant asset namespacing, campaign binding, public tenant routing, acquisition flow.
- P2: subscription routing, billing/charge ownership, inbound callback correlation, notification/cadence routing, postbacks, admin portal.
- P3: reports/ops, secret and observability hardening, partner onboarding contracts, legacy migration isolation.

Non-negotiable invariants:

- No protected path may fall back to a global tenant.
- Public default-tenant compatibility is allowed only when the legacy route maps uniquely.
- Every channel operation must enforce capability, credential, and tenant ownership.
- Every callback/worker flow must include replay/idempotency behavior.
- Secrets must be references, not durable business data.
- Reports and admin views must not aggregate across tenants unless the actor is platform-scoped.
