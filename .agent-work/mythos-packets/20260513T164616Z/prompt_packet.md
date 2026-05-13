# Mythos Execution Packet

## Objective

Produce an evidence-backed admin tenant operating model for `timwe-subscription`: how admins create/manage tenants, switch tenant context, and how tenant identity scopes userbase, products, notifications, subscriptions, campaigns, cadence, billing, postbacks, reports, and related features.

## Non-goals

- Do not implement changes during this audit unless a runtime check exposes a blocker that prevents evidence collection.
- Do not infer implementation from filenames, comments, config, TODOs, or mocks alone.
- Do not assume `nrg` is the whole tenancy model; verify whether it is bootstrap, default, canonical, or one tenant among many.
- Do not expose pasted bearer tokens or secrets in reports.

## Targets

- Admin UI tenant workspace, tenant selection, tenant persistence, API interceptors, and tenant management views.
- Backend admin tenant APIs, Auth0/bootstrap authorization, tenant resolution, and tenant-scoped request headers.
- Tenant-scoped domains: customers/userbase, products, subscriptions, notifications, campaigns, cadence/message series, billing, postbacks, reports, tenant channels/operators/countries.
- Live schema and migrations supporting tenant ownership, tenant channels, and canonical `nrg` backfill.
- Runtime endpoints and logs for representative tenant-scoped pages.

## Assumptions and unknowns

- Assumption: repo root is `/home/xper626/workspace/apps/timwe-subscription`.
- Assumption: admin UI uses `X-Tenant-Key` or `X-Tenant-Id` to carry selected tenant context.
- Assumption: recent commits `dc3ff90`, `61cf258`, and `214e29e` are relevant to the current tenant-admin behavior.
- Unknown: whether normal admins have explicit tenant membership rows or only platform/bootstrap access exists today.
- Unknown: whether every domain has complete tenant filtering at repository level.
- Unknown: whether all live migrations are represented in code-level migration files and history.
- Risk: `.gitignore` managed block drift reported by `align-gitignore --check`; not repaired in this audit.

## Status taxonomy

- `verified implemented`
- `implemented but unverified`
- `partially implemented`
- `configured only`
- `stubbed`
- `not implemented`
- `blocked`
- `not applicable`

## Evidence requirements

- UI evidence: file paths for routes, components, services, interceptors, and storage behavior.
- API evidence: route/handler/repository paths and symbols.
- Auth evidence: Auth0 claim parsing, bootstrap subject/email config, platform scope, tenant scope, and error behavior.
- Schema evidence: tables, tenant columns, tenant-channel relationships, migrations, and live DB checks.
- Runtime evidence: representative curl checks for tenant-scoped endpoints using `X-Tenant-Key: nrg`.
- Test evidence: relevant Go/Angular test or build commands when available.

## Skill chain

`mythos-prompt-compiler` source brief retained -> `mythos-agent-orchestrator` -> local implementation audit using `mythos-implementation-auditor` style evidence discipline -> `mythos-codebase-cartographer` style mapping -> `mythos-evidence-ledger` style matrix -> `mythos-adversarial-gate` style final risk pass.

Tier 2 executor: local direct audit. No subagent fan-out in this run.

## Phase decomposition

1. Admin UI map
   - Input evidence: frontend routes/services/interceptors.
   - Action: identify tenant create/list/switch/load behavior.
   - Output artifact: UI tenant-flow matrix.
   - Gate: each UI claim has file evidence.
   - Fallback: mark unverified if build/runtime evidence is missing.

2. Backend auth and tenant API map
   - Input evidence: admin routers, auth middleware, handlers, services.
   - Action: trace platform and tenant identity from Auth0/header to context.
   - Output artifact: backend auth-flow matrix.
   - Gate: each auth claim has route/symbol evidence.
   - Fallback: mark blocked for credential-only paths.

3. Domain tenant-scope matrix
   - Input evidence: repositories, migrations, live schema.
   - Action: classify each major domain by status taxonomy.
   - Output artifact: feature-by-feature table.
   - Gate: each status has code/schema/runtime evidence.
   - Fallback: downgrade to partially implemented or implemented but unverified.

4. Runtime representative verification
   - Input evidence: running services and known tenant key.
   - Action: test representative admin endpoints with tenant context.
   - Output artifact: command/result ledger.
   - Gate: endpoints return expected status and tenant-scoped data or explicit failure.
   - Fallback: enter hypothesis loop.

5. Adversarial pass
   - Input evidence: matrices and command ledger.
   - Action: look for unscoped queries, UI-only claims, missing migrations, and cross-tenant leakage risks.
   - Output artifact: residual risks and next smallest slice.
   - Gate: no unsupported "verified" claims remain.

## Verification gates

- `git status --short` is clean at audit start.
- All implementation claims are backed by file, command, schema, route, runtime log, or explicit blocked reason.
- Runtime checks avoid exposing secrets in final output.
- Final report follows the final response contract.

## Failure loop

For failures: symptom -> ranked hypotheses -> confirming/refuting command or code inspection -> result -> minimal patch only if evidence collection is blocked -> re-test -> status update.

## Final report contract

1. Status by target
2. Evidence found
3. Evidence missing
4. Commands/checks run
5. Files changed
6. Failures fixed
7. Blocked items
8. Residual risks
9. Next smallest slice

## Compiled prompt source

User asked: as an admin, how does an admin create and manage tenants, switch between tenants, and how does each tenant identify/manage userbase, products, notifications, subscriptions, campaigns, and all other features?

## Compiler assumptions retained

- Treat this as an implementation audit and architecture report.
- Use exact status taxonomy.
- Verify with code, schema, migration, routes, runtime, and tests.

## Compiler unknowns carried forward

- Exact admin membership model.
- Tenant switching persistence model.
- Completeness of tenant scope across every feature.
