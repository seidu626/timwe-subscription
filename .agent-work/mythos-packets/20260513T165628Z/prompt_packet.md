# Mythos Execution Packet

## Objective
Resolve the remaining admin tenant-management issues identified in the previous audit: make tenant membership durable and manageable, ensure admins can switch tenants based on real membership/authorization data rather than dev-only bootstrap, and close any tenant-scoping gaps that still affect admin pages.

## Non-goals
- Do not rewrite the tenant platform architecture.
- Do not remove legacy nullable tenant runtime paths without fresh data proof.
- Do not change production secrets or Auth0 tenant configuration directly.
- Do not repair managed `.gitignore` drift from this orchestrator pass.

## Targets
- Admin tenant membership data model and migration.
- Acquisition admin API for tenant members and current workspace discovery.
- Angular admin workspace resolution and tenant member management UI.
- Subscription External admin action tenant scoping if current UI/API bypasses selected tenant context.
- Verification against local build/tests and live schema where safe.

## Assumptions and unknowns
- The local `.env` points at the active shared PostgreSQL schema used by the running services.
- `nrg` is the canonical tenant and is active in live schema.
- Current dev access grants platform scope through bootstrap subject/email; this is not equivalent to durable tenant membership.
- Unknown: whether a partial membership table already exists under another name. Phase 1 must verify before adding schema.

## Status taxonomy
- `verified implemented`: source and command/runtime evidence confirm behavior.
- `fixed in this pass`: patch implemented and verification passed.
- `partially implemented`: some source exists but missing UI/API/runtime proof.
- `configured only`: config grants behavior without durable domain model.
- `blocked`: requires credentials, release approval, or unsafe migration decision.

## Evidence requirements
- Source lines for every implemented route/service/repository/UI claim.
- Migration/schema proof for new membership structures.
- Build/test command output for changed frontend/backend code.
- Live DB query output for applied migrations or existing rows when used.
- Live endpoint/CORS/status checks for changed request paths when services are running.

## Skill chain
- `mythos-agent-orchestrator` front door.
- `mythos-codebase-cartographer` locally through targeted `rg`/file reads.
- `migration-orchestrator` discipline for additive schema only.
- `frontend-skill` discipline for Angular UI changes.
- `mythos-verification-gate` through build/tests/live checks.
- Tier 2 executor: `auto-pilot` style single-runtime implementation loop; no subagent dispatch in this pass.

## Phase decomposition
1. Intake and packet
   - Input evidence: batched discovery.
   - Action: write this packet.
   - Output artifact: this file.
   - Gate: packet exists before implementation.
   - Fallback: stop if repo dirty or harness blocker appears.
2. Data-model discovery
   - Input evidence: schema/migration/source search.
   - Action: identify existing membership model or design smallest additive table.
   - Output artifact: evidence matrix update.
   - Gate: no duplicate table if model already exists.
   - Fallback: classify existing model instead of adding another.
3. Backend membership API
   - Input evidence: admin management patterns.
   - Action: add platform-scoped tenant member list/upsert/delete endpoints and current-workspace discovery if absent.
   - Output artifact: migration, domain/repo/service/handler/routes/tests.
   - Gate: unit tests/build pass.
   - Fallback: narrow to list/upsert only if delete is unsafe.
4. Frontend membership/workspace UI
   - Input evidence: tenant list component and workspace service.
   - Action: expose tenant members on tenant admin page and use backend workspace discovery where possible.
   - Output artifact: Angular services/components/tests.
   - Gate: `npm run build` passes.
   - Fallback: keep dev bootstrap as fallback only.
5. Subscription tenant-scope check
   - Input evidence: subscription admin service, interceptor, routes.
   - Action: patch selected-tenant forwarding/filtering gaps if confirmed.
   - Output artifact: code/tests or explicit no-op proof.
   - Gate: targeted build/test/live endpoint proof.
   - Fallback: evidence-only residual risk if broad rewrite required.
6. Verification and commit
   - Input evidence: changed files.
   - Action: run focused tests/build/live checks, commit.
   - Output artifact: commit hash and final report.
   - Gate: working tree clean except allowed untracked artifacts.
   - Fallback: report exact failing command and leave no false done claim.

## Verification gates
- `go test` for touched backend packages.
- `npm run build` for `frontend/webspa-admin`.
- Live schema query for migration application when a migration is applied.
- Live CORS/status probes for any admin route changed.

## Failure loop
symptom -> ranked hypotheses -> confirming/refuting experiment -> result -> revised hypothesis -> minimal patch or blocked reason -> re-test

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
Direct user request: `$mythos-agent-orchestrator with all the issues identified, think and plan on how to resolve them and proceed with implementation`.

## Compiler assumptions retained
Previous audit found durable user-to-tenant membership management missing, Angular tenant create UI now fixed, live tenant migrations applied, and subscription-external tenant scope only partially verified.

## Compiler unknowns carried forward
Whether membership data model already exists under another name; whether subscription-external admin action history ignores selected tenant headers.
