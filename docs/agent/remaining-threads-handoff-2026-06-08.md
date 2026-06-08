# Remaining Threads Handoff - 2026-06-08

## Runtime Routing Evidence

| requested_runtime | actual_executor | executor_evidence |
|---|---|---|
| codex | codex | codex session; `./docs/agent/remaining-threads-handoff-2026-06-08.md` |

`.harness/config.json` has Codex enabled; Claude and Gemini are disabled. T2 classifier produced `assignment.decision: review`, so the WorkOrder is parked in `agent/state/work-orders/review/`.

## Current State

- Branch: `agent/codex/multitenant-onboarding-20260608-043933`
- Latest commit: `9f6b19f chore(control): gate T2 tenancy RFC`
- Prior completed control-plane commits:
  - `645cbf2 chore(control): archive TMP-016`
  - `257a9bb chore(control): refresh TMP-016 evidence`
  - `a026bf3 chore(control): activate TMP-016`
- Context snapshot: `20260608-044254`
- Queue counts at handoff:
  - Ready: 45
  - Review: 4
  - Active: 0
  - Blocked: 0
  - Archive: 3

## Verification Evidence

- `agent-hub schema validate-all --root . --json` -> pass, 126 schemas checked.
- `agent-hub reconcile --root . --check --json` -> pass, 0 errors, 0 warnings.
- `agent-hub dispatch ready --root . --out .harness/runs/remaining-threads-handoff-20260608-0646/dispatch-ready.json --json` -> warn, 45 claimable, 0 errors, 208 warnings.
- `context-cycle save --repo "$PWD" && context-cycle restore --primer --repo "$PWD"` -> snapshot `20260608-044254`.

## Review-Gated Threads

These are not claimable until the review decision is handled.

| WorkOrder | Class | State | Why it matters |
|---|---|---|---|
| `WO-CREATE-A-DOCS-ONLY-BOUNDED-ENABLER-FOR-T2-DRAFT` | bounded_enabler | review | T2 canonical tenancy model RFC gate. Do this before tenant-management implementation threads: `TMP-048`, `TMP-051`, `TMP-055`, `TMP-057`, `TMP-065`, `TMP-071`, `TMP-074`. |
| `WO-TMP-015-001` | operational_slice | review | Tenant channel safe credentials and tenant-specific health visibility. Runtime tier HIGH. |
| `WO-TMP-042-001` | operational_slice | review | Release decision packet for remaining full-system verification blockers. Runtime tier HIGH. |
| `WO-TMP-056-001` | vertical_defect_slice | review | Acquisition API startup after admin schema bootstrap without postback dispatcher missing-column errors. |

## Ready Threads

### Tenant Model, Admin, And Enforcement

- `WO-TMP-011-001` - backfill and verify existing production data under tenant isolation.
- `WO-TMP-048-001` - named admin users enter tenant workspace and operate across configured tenants.
- `WO-TMP-050-001` - assign tenantless production data to canonical `nrg` tenant.
- `WO-TMP-051-001` - tenant catalog list/update API and UI.
- `WO-TMP-052-001` - classify tenant nullable ownership paths.
- `WO-TMP-053-001` - acquisition/admin tenant-owned tables proof.
- `WO-TMP-054-001` - subscription/cadence tenant-owned tables proof.
- `WO-TMP-055-001` - runtime paths use tenant-aware canonical ownership.
- `WO-TMP-057-001` - tenant dashboard KPIs load for selected tenant workspace.
- `WO-TMP-058-001` - bootstrap admins work when Auth0 omits `email_verified`.
- `WO-TMP-063-001` - cadence admin tenant-header CORS preflight.
- `WO-TMP-065-001` - Angular tenant workspace interceptor readiness.
- `WO-TMP-074-001` - residual tenant-null cleanup and fail-closed runtime behavior.

Resume rule: resolve the T2 review gate before starting any thread that changes tenant membership, tenant resolver behavior, tenant catalog behavior, tenant workspace UI, or tenant-null runtime enforcement.

### Runtime, Compose, And Schema

- `WO-TMP-030-001` - acquisition API compose image build context.
- `WO-TMP-031-001` - notification worker compose DB env.
- `WO-TMP-032-001` - postback dispatcher compose DB env.
- `WO-TMP-034-001` - acquisition runtime schema provisioning.
- `WO-TMP-035-001` - notification `message_outbox` schema.
- `WO-TMP-036-001` - postback `postback_outbox` schema.
- `WO-TMP-045-001` - clean compose database bootstrap for runtime prerequisites.
- `WO-TMP-049-001` - acquisition campaign slug migration startup.

Resume rule: prefer source-inventory and decision-template work before schema/bootstrap changes if runtime SQL ownership is uncertain.

### Release And Control-Plane Evidence

- `WO-TMP-021-001` - full-system verification matrix.
- `WO-TMP-024-001` - slice registry evidence reconciliation.
- `WO-TMP-025-001` - TMP-021 metadata reconciliation.
- `WO-TMP-026-001` - webspa submodule verification.
- `WO-TMP-027-001` - release matrix stale blocker reconciliation.
- `WO-TMP-029-001` - compose runtime smoke tooling blocker.
- `WO-TMP-033-001` - TMP-032 ledger state reconciliation.
- `WO-TMP-037-001` - landing-web dependency remediation approval.
- `WO-TMP-038-001` - local main integration strategy.
- `WO-TMP-039-001` - operational domain brief reconciliation.
- `WO-TMP-040-001` - webspa local checkout verification.
- `WO-TMP-041-001` - runtime schema source inventory.
- `WO-TMP-043-001` - release decision templates.
- `WO-TMP-044-001` - stale value-gate evidence reconciliation.

Resume rule: these are good candidates while T2 waits on review, as long as they stay docs/control-plane-only and do not mutate runtime source.

### Frontend And Dev Loop

- `WO-TMP-014-001` - tenant admin resources in admin UI.
- `WO-TMP-046-001` - reproducible admin frontend source.
- `WO-TMP-059-001` - faster local admin dev startup.
- `WO-TMP-060-001` - Sentry `ErrorHandler` provider startup fix.
- `WO-TMP-062-001` - faster `just stop && just dev` restart.
- `WO-TMP-064-001` - Angular 18 Sentry peer dependency install.
- `WO-TMP-071-001` - webspa-admin UI refresh.

Resume rule: `TMP-014`, `TMP-065`, and `TMP-071` intersect tenant workspace semantics; treat them as dependent on the T2 tenancy RFC unless scoped strictly to non-semantic UI/dev-loop repair.

### Landing And Common Library

- `WO-TMP-022-001` - landing-web production build with legacy and tenant-qualified URLs.
- `WO-TMP-023-001` - common package tests for tenant auth/database helpers.
- `WO-TMP-047-001` - landing-web Next/PostCSS vulnerability remediation.

Resume rule: `TMP-023` is a low-overlap prerequisite-style thread for tenant helper confidence. `TMP-047` involves dependency remediation and may require operator approval if package upgrades become breaking.

## Recommended Resume Order

1. Review or approve `WO-CREATE-A-DOCS-ONLY-BOUNDED-ENABLER-FOR-T2-DRAFT`.
2. If T2 is approved, move it to `ready` with `agent-hub workorders advance --root . --work-order WO-CREATE-A-DOCS-ONLY-BOUNDED-ENABLER-FOR-T2-DRAFT --to ready --force --json --execute`, then draft `docs/architecture/canonical-tenancy-model.md`.
3. After the RFC draft passes review, resume tenant implementation threads in dependency order: `TMP-048` / `TMP-051`, then `TMP-053` / `TMP-054`, then `TMP-055` / `TMP-074`.
4. While waiting on T2 review, safe independent options are control-plane evidence threads such as `TMP-021`, `TMP-024`, `TMP-025`, `TMP-027`, `TMP-039`, `TMP-041`, `TMP-043`, or `TMP-044`.
5. Keep `agent-hub schema validate-all --root . --json` and `agent-hub reconcile --root . --check --json` as the closeout gate for each slice.

## Known Risks

- The T2 WorkOrder objective is truncated by the current `agent-hub workorders emit` output, but the full prompt survives in `agent/state/goals/T2-canonical-tenancy-model.json`.
- `agent-hub dispatch ready` reports 208 warnings despite 0 errors; inspect the full JSON when choosing a WorkOrder because warnings can indicate dependency, scope, or review-state caveats.
- Claude and Gemini are disabled in `.harness/config.json`; any cross-runtime peer review requires an explicit runtime enablement decision or an external review path.
- Legacy `.agent/` artifacts are present but retired. Use vNext queues under `agent/state/work-orders/` as authoritative state.

## Stop Conditions

- Do not implement tenant-management runtime/UI behavior until the T2 tenancy RFC gate is resolved.
- Do not force WorkOrders from `review` to `ready` unless the operator explicitly accepts the review decision.
- Do not repair unrelated dispatch warnings during an implementation slice; record them and keep the scope narrow.
- Do not edit forbidden paths from a WorkOrder, even for small fixes.

## Cost vs Claim

This handoff is an L1 operational artifact. It does not resolve the T2 review gate or implement any remaining WorkOrder. Its value is queue truth, dependency ordering, and a concrete resume path anchored to passing schema/reconcile evidence.

## What I Might Be Wrong About

- The phrase "remaining threads" is interpreted here as remaining vNext WorkOrders and review gates, not legacy `.agent/` session handoffs.
- The recommended order is dependency-based from the current WorkOrder objectives and scope, not a product-owner priority decision.
- The dispatch warning count was summarized, not individually triaged; a future executor should inspect `.harness/runs/remaining-threads-handoff-20260608-0646/dispatch-ready.json` before claiming work.
