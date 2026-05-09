# Full-System Verification Matrix

## Objective

Goal:
- Build every discovered service or runnable component where the local environment supports it.
- Verify every discovered implemented feature through source evidence, tests, smoke checks, or runtime evidence.
- Reconcile harness, slice, git, and evidence state before making any completion claim.

Definition of done:
- Service inventory complete.
- Feature inventory complete.
- Verification matrix created before implementation fixes.
- Build, test, control-plane, runtime, health, and migration checks executed where possible.
- Every failure has a root cause, fix, blocked reason, or follow-up slice candidate.
- Final report includes commands, evidence, changed files, and unresolved risks.

Scope exclusions:
- Missing or planned-only features are not implemented inside this audit slice.
- External systems are mocked, sandboxed, or marked blocked.
- Production deploy, dependency changes, and schema rewrites are out of scope.

## Service Inventory

| Service/component | Type | Source files/manifests | Build command | Test command | Start command | Health/smoke check | Dependencies | Status |
|---|---|---|---|---|---|---|---|---|
| common | shared Go library | `common/go.mod`, `common/**` | `go test ./...` | `go test ./...` | not applicable | package tests | Go 1.26.2 local toolchain | fixed |
| subscription-external | backend API | `services/subscription-external/go.mod`, `cmd/main.go`, migrations | `go build -o /tmp/agent-build-timwe-20260509/subscription-external cmd/main.go` | `go test ./...` | `make start-subscription-external` | `/health`, `/metrics` via scripts | Postgres, Redis, TIMWE credentials for live flows | passed |
| subscription-partner | backend API | `services/subscription-partner/go.mod`, `cmd/main.go` | `make build-local-subscription`, readonly module-mode build | `go test ./...`, readonly module-mode test | `make start-subscription` | `/health` | Redis, Postgres, shared common module | failed |
| billing | backend API | `services/billing/go.mod`, `cmd/main.go` | readonly module-mode build | `go test ./...` | `make start-billing` | `/health` | Postgres for live flows | passed |
| notification API | backend API | `services/notification/go.mod`, `cmd/main.go` | readonly module-mode build | `go test ./...`, readonly module-mode test | `make start-notification` | `/health` | Postgres, Redis, auth common module | failed |
| notification worker | worker | `services/notification/cmd/notification-worker/main.go` | readonly module-mode build | covered by notification package tests where buildable | `./notification-worker` | Prometheus metrics handler | Postgres, MT endpoint config | passed |
| acquisition-api | backend API | `services/acquisition-api/go.mod`, `cmd/main.go`, migrations | readonly module-mode build | `go test ./...` | `make start-acquisition-api` | service routes and compose dependency health | Postgres, MinIO, TIMWE/Auth0 config for live flows | passed |
| cadence-engine | worker/admin HTTP | `services/cadence-engine/go.mod`, `cmd/cadence-engine/main.go` | readonly module-mode build | `go test ./...` | `make start-cadence-engine` | admin HTTP on `:8091` | Postgres | passed |
| postback-dispatcher | worker | `services/postback-dispatcher/go.mod`, `cmd/main.go` | readonly module-mode build | `go test ./...` | compose service | worker starts against DB | Postgres | passed |
| landing-web | Next.js frontend | `services/landing-web/package.json`, `app/**` | `npm run build` | build/typecheck; no route tests present | `npm run dev` / `npm start` | Next route build output | Node 24.15.0, npm 11.12.1, acquisition API at runtime | fixed |
| webspa-admin | admin frontend gitlink | `frontend/webspa-admin` gitlink | unavailable | unavailable | unavailable | unavailable | pinned gitlink commit unavailable from configured submodule remote | blocked |
| KrakenD gateway | gateway | `krakend/**`, `Makefile` | `make docker-build-krakend` | `make krakend-query-forwarding-check` | compose service | `krakend check` not run; query-forwarding check run | Docker/Podman, KrakenD image | partially verified |
| docker compose dev stack | local integration stack | `docker-compose.yml` | `docker compose -f docker-compose.yml config` | config render | `make compose-up` | compose healthchecks | required env vars and external network | blocked |
| tenant migration runner | migration/ops script | `scripts/db-migrate-tenant-platform.sh` | `bash -n scripts/db-migrate-tenant-platform.sh` | `make -n db-migrate-tenant-platform-dry-run` | `make db-migrate-tenant-platform-dry-run` | dry-run output against DB | Postgres credentials | partially verified |

## Feature Inventory

| Feature | Evidence of implementation | Owning service/component | Public interface | Critical invariants | Verification method | Status |
|---|---|---|---|---|---|---|
| tenant context and service auth contract | `common/auth/auth0jwt`, `common/auth/tenantctx`, TMP-018 reports | common | JWT claims and trusted service headers | tenant identity cannot be forged; replay nonce rejected | `go test ./...` in common | fixed |
| tenant admin management scope | `services/acquisition-api/internal/handler`, TMP-002 report | acquisition-api | `/v1/admin/products`, `/v1/admin/userbase` | tenant filtering and authorization | `go test ./...` in acquisition-api | passed |
| channel catalog and credential binding | acquisition migrations/handlers, TMP-003/TMP-004 reports | acquisition-api | `/v1/admin/channels`, credential binding routes | credential references only, no secret exposure | acquisition-api tests plus docs review | partially verified |
| tenant campaign binding and public routing | acquisition migrations/handlers, landing-web routes | acquisition-api, landing-web | `/v1/admin/campaigns`, `/lp/:slug`, `/lp/:tenant/:slug` | overlapping slugs resolve deterministically | acquisition-api tests; landing-web build | fixed |
| tenant acquisition flow | acquisition transaction handlers, landing-web flow | acquisition-api, landing-web | `/v1/acquisition/transactions`, confirm route | consent, HE, attribution, tenant/campaign match | acquisition-api tests; landing-web build | partially verified |
| subscription routing by tenant channel | subscription-external and subscription-partner services | subscription-external, subscription-partner | subscription external/admin and partner endpoints | no global credentials when tenant/channel required | subscription-external tests pass; subscription-partner default tests fail | failed |
| notification and cadence routing | notification tests, cadence tests, TMP-008 report | notification, cadence-engine | notification list, cadence admin HTTP | tenant/channel context preserved | cadence tests pass; notification default tests fail | failed |
| postback attribution routing | acquisition postback admin and dispatcher | acquisition-api, postback-dispatcher | postback admin routes and dispatcher worker | tenant/provider-specific recovery | acquisition and dispatcher tests | passed |
| tenant reporting operations | acquisition reporting handlers, TMP-010 report | acquisition-api | reporting endpoints | tenant/channel filters avoid leakage | acquisition-api tests | passed |
| billing charge ownership | TMP-017 decision, billing service | subscription-external, billing | charge endpoints and billing routes | single owner for tenant charge state | billing tests pass; subscription external tests pass | passed |
| tenant asset namespacing | acquisition storage config and handlers, TMP-019 report | acquisition-api | campaign asset presign route | tenant-scoped object keys | acquisition-api tests | passed |
| observability baseline | notification observability tests, compose monitoring | notification, ops monitoring | metrics, logs, dashboards | safe bounded labels, no PII labels | notification blocked by module sums/vendor; worker build passes | failed |
| partner onboarding contracts | docs and examples under TMP-016 | docs/examples | onboarding document and fixture validator | versioned tenant/channel contract | prior evidence plus HVC | passed |

## Environment Readiness

| Requirement | Source | Available? | Action |
|---|---|---|---|
| Go toolchain | `go.mod` files | yes: `go1.26.2-X:nodwarf5` | Used for Go tests/builds. |
| Node/npm | `services/landing-web/package.json` | yes: Node `v24.15.0`, npm `11.12.1` | Ran `npm ci`, `npm run build`, `npm audit`. |
| Docker/Compose | `docker-compose.yml` | yes: Podman Docker emulation, Compose `5.1.3` | Rendered compose config; did not start stack because required env/network/secrets are incomplete. |
| Postgres/Redis live dependencies | compose and service configs | no/unknown | Mark live runtime and DB migration checks blocked unless env vars and local stack are provided. |
| TIMWE/Auth0/provider credentials | compose/service configs | no | External-provider flows marked blocked or partially verified by tests only. |
| webspa-admin submodule content | gitlink `frontend/webspa-admin` | no | Blocked: `.gitmodules` maps the path, but `git submodule update --init --recursive frontend/webspa-admin` cannot fetch pinned commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a` from the configured remote. |
| landing-web dependencies | `package-lock.json` | yes after `npm ci` | Build passed; audit reports unresolved Next/PostCSS vulnerabilities. |

## Service Verification Matrix

| Service/component | Build | Unit tests | Integration tests | Migrations | Runtime start | Health/smoke | Status | Evidence |
|---|---|---|---|---|---|---|---|---|
| common | fixed | fixed | not run | n/a | n/a | n/a | fixed | TMP-023 made `cd common && go test ./...` pass. |
| subscription-external | passed | passed | not run live | migrations discovered | not run | not run live | passed | `go test ./...` passed; readonly build passed; canonical make built this service before failing later. |
| subscription-partner | failed canonical, passed readonly | failed canonical, passed readonly | not run live | n/a | not run | not run live | failed | Default vendor mode cannot resolve `common/cache`; `GOFLAGS=-mod=readonly go test ./...` passed. |
| billing | passed | passed | not run live | n/a | not run | not run live | passed | `go test ./...` passed; readonly build passed. |
| notification API | failed | failed | not run live | n/a | not run | not run live | failed | Vendor modules inconsistent; readonly mode lacks go.sum entries for Auth0/JWT deps. |
| notification worker | passed | blocked by module test failures | not run live | n/a | not run | metrics not run live | passed | readonly build for worker passed. |
| acquisition-api | passed | passed | not run live | SQL migrations discovered | not run | not run live | passed | `go test ./...` passed; readonly build passed. |
| cadence-engine | passed | passed | not run live | n/a | not run | not run live | passed | `go test ./...` passed; readonly build passed. |
| postback-dispatcher | passed | passed | not run live | n/a | not run | not run live | passed | `go test ./...` passed; readonly build passed. |
| landing-web | fixed | build/typecheck passed | not run live | n/a | not run | route build output passed | fixed | Initial `npm run build` failed; TMP-022 patch made `npm run build` pass. |
| webspa-admin | blocked | blocked | blocked | n/a | blocked | blocked | blocked | gitlink exists and `.gitmodules` maps it, but the configured remote does not contain pinned commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`. |
| docker compose dev stack | config rendered with warnings | n/a | blocked | n/a | not run | blocked | blocked | `docker compose config` renders but required env vars are blank and one secret-shaped DB credential appears in config output. |

## Feature Verification Matrix

| Feature | Verification path | Command/check | Expected signal | Actual result | Status | Evidence |
|---|---|---|---|---|---|---|
| tenant context and auth | common package tests | `go test ./...` in `common` | all auth tests pass | pass after TMP-023 | fixed | `TestMiddlewareRejectsReplayNonce` now rejects replay. |
| admin and acquisition APIs | Go tests | `go test ./...` in `services/acquisition-api` | pass | pass | passed | acquisition-api package tests passed. |
| subscription external tenant routing | Go tests | `go test ./...` in `services/subscription-external` | pass | pass | passed | service/domain/handler/repository/worker tests passed. |
| subscription partner routes | Go tests | `go test ./...`; `GOFLAGS=-mod=readonly go test ./...` | canonical pass | canonical failed; readonly passed | failed | vendor mode cannot resolve common packages. |
| notification tenant/cadence routing | Go tests | `go test ./...`; `GOFLAGS=-mod=readonly go test ./...` | pass | failed | failed | inconsistent vendor and missing go.sum auth deps. |
| landing public tenant routing | Next build | `npm run build` | pass and route output includes legacy and tenant-qualified routes | pass after TMP-022 | fixed | build lists `/lp/[tenant]` and `/lp/[tenant]/[slug]`. |
| tenant migration dry-run entrypoint | shell/make checks | `bash -n`; `make -n db-migrate-tenant-platform-dry-run` | syntax and target resolve | pass | partially verified | DB-backed dry-run blocked by missing Postgres env. |
| KrakenD query forwarding | script check | `make krakend-query-forwarding-check` | pass | pass | passed | check passed against `krakend/krakend.json`. |
| compose runtime stack | compose render | `docker compose -f docker-compose.yml config` | usable config with required env | rendered with blank env warnings | blocked | missing env vars and secret-shaped config risk. |

## Commands Run

| Time/order | Command | Purpose | Result | Evidence/log summary |
|---|---|---|---|---|
| 1 | `git status --short --branch` | Session-start safety and git state | passed | Main checkout had no uncommitted files; local `main` was ahead 51 and behind `origin/main` by 2. |
| 2 | `git worktree add ../worktrees/codex-full-system-verify-20260509-005155 -b agent/codex/full-system-verify-20260509-005155` | Isolate non-read-only audit work | passed | Created isolated verification branch from local `main`. |
| 3 | `context-cycle save` then `context-cycle restore` | Loop entry checkpoint | passed | Snapshot `20260509-005205` restored for this worktree. |
| 4 | `agent-supervisor --config .harness/config.json preflight` | Control-plane drift check | passed with warning | Non-repairable stale superseded ledger rows: `TMP-011-repair-1`, `TMP-015-repair-1`; no schedulable stale rows. |
| 5 | `agent-harness list` | Harness task state | passed | TMP-011, TMP-014, TMP-015, TMP-016 were `done`. |
| 6 | `hvc check agent/backlog/issues/*.md --fail-on block` | Classifier gate | passed | Existing four issues had no blockers; TMP-015 had review broadness signal only. |
| 7 | `agent-supervisor --config .harness/config.json list-tasks` | Supervisor queue state | passed | No ready tasks; four done tasks and two superseded repair rows. |
| 8 | `git merge --no-edit origin/main` | Probe whether remote reconciliation could be applied cleanly to isolated branch | failed | Add/add conflicts across workflows, issues, vendored files, and value-gate reports; merge was aborted. |
| 9 | `go test ./...` in `common` | Shared package tests | failed, then fixed | Initial openAPI generator compile error, postgres test signature mismatch, nonce replay test failure; TMP-023 made the command pass. |
| 10 | `go test ./...` in service modules | Service unit tests | mixed | subscription-external, billing, acquisition-api, cadence-engine, postback-dispatcher passed; subscription-partner and notification failed under default vendor mode. |
| 11 | `GOFLAGS=-mod=readonly go test ./...` | Separate vendor drift from code failures | mixed | subscription-partner passed; common still failed; notification failed on missing go.sum auth dependencies. |
| 12 | `make build-all-local` | Canonical build | failed | subscription-external built, then subscription-partner failed under vendor mode. |
| 13 | readonly module-mode `go build` per service | Compile service binaries without writing repo artifacts | mixed | all checked services passed except notification API; notification worker passed. |
| 14 | `npm ci` in `services/landing-web` | Install locked frontend deps | passed with audit warning | 30 packages installed; npm reported 1 moderate and 1 high vulnerability. |
| 15 | `npm run build` in `services/landing-web` | Frontend production build | failed, then fixed | Initially failed on dynamic segment conflict; TMP-022 patch made build pass. |
| 16 | `npm audit --audit-level=moderate` | Supply-chain check | failed | Next/PostCSS advisories; fix requires breaking `next@16.2.6`, so no dependency change made. |
| 17 | `docker compose -f docker-compose.yml config` | Compose config render | blocked | Renders, but many required env vars blank; output includes a secret-shaped DB credential in service env. |
| 18 | `make krakend-query-forwarding-check` | Gateway config check | passed | Query forwarding check passed. |
| 19 | `bash -n scripts/db-migrate-tenant-platform.sh` | Migration script syntax | passed | Shell syntax valid. |
| 20 | `make -n db-migrate-tenant-platform-dry-run` | Migration target resolution | passed | Target resolves to the migration script dry-run. |
| 21 | `git push --progress -u origin HEAD` from `agent/codex/full-system-verify-20260509-005155` | Publish original isolated verification branch | blocked | Push transferred past 200 MiB because the branch carried 52 commits from local `main`, including a 332 MB dump and generated binaries absent from `origin/main`; push was terminated for oversized history risk. |
| 22 | `git worktree add ../worktrees/codex-full-system-verify-pr-20260509-0129 -b agent/codex/full-system-verify-pr-20260509-0129 origin/main` | Create clean PR branch from remote source of truth | passed | New branch starts at `origin/main` commit `791ae9d`. |
| 23 | `git cherry-pick -x 5984863` | Move verified audit and repair commit onto clean PR branch | passed | Produced clean branch commit `356c449` before this evidence reconciliation. |
| 24 | `git rev-list --objects origin/main..HEAD \| git cat-file --batch-check=... \| sort -k3 -nr \| head` | Confirm clean branch has no oversized objects | passed | Largest new blob is `docs/agent/full-system-verification-2026-05-09.md` at 21,832 bytes. |
| 25 | `jq empty slices/manifest.json && hvc check agent/backlog/issues/*.md --fail-on block && slice-harness sync --dry-run` | Re-run manifest, classifier, and slice drift gates on clean branch | passed | HVC allowed TMP-021/022/023 and `slice-harness sync --dry-run` reported no drift. |
| 26 | `cd common && go test ./...` | Re-run common package test repair on clean branch | passed | All common packages passed or had no test files. |
| 27 | `cd services/landing-web && npm ci && npm run build` | Re-run landing-web dependency install and production build on clean branch | passed with audit warning | Build passed and routes include `/api/campaigns/[tenant]`, `/api/campaigns/[tenant]/[slug]`, `/lp/[tenant]`, and `/lp/[tenant]/[slug]`; npm still reports 1 moderate and 1 high vulnerability. |

## Failure Ledger

| Failure | Command/check | Root cause | Patch | Re-verification | Status |
|---|---|---|---|---|---|
| Local branch cannot fast-forward or cleanly merge `origin/main` | `git merge --no-edit origin/main` | Local `main` and `origin/main` contain divergent overlapping history with add/add conflicts, including generated/vendor files and slice evidence. | Created clean PR branch from `origin/main` and cherry-picked only the verified audit/repair commit. | Clean branch has no oversized blobs and re-ran manifest, HVC, common tests, and landing-web build. | fixed for PR branch; local main integration still blocked |
| landing-web production build failed | `npm run build` | Next.js App Router rejected sibling dynamic segment names `[slug]` and `[tenant]` at the same route level. | TMP-022 renamed single-segment dynamic folders to `[tenant]` and mapped absent `slug` to the single segment. | `npm run build` passed. | fixed |
| common package fails | `go test ./...` in `common` | Generator API drift, postgres test signature drift, and nonce replay test clock mismatch. | TMP-023 excluded tool-only generator helper, updated postgres tests, and aligned nonce store test clock. | `cd common && go test ./...` passed. | fixed |
| notification package fails | `go test ./...` and readonly mode | Vendor directory is inconsistent with go.mod, and readonly module mode lacks auth dependency checksums. | Dependency/vendor remediation requires approval because it touches module metadata/vendor state. | Not fixed. | blocked |
| subscription-partner canonical test/build fails | `go test ./...`, `make build-all-local` | Vendor mode cannot resolve shared `common/cache` import. | Pending dependency/vendor strategy; readonly mode passes. | `GOFLAGS=-mod=readonly go test ./...` passed. | blocked |

## Blocked Checks

| Check | Reason | Exact command or requirement to unblock |
|---|---|---|
| Verify latest `origin/main` and local `main` as one integrated state | Local and remote main histories conflict heavily. | Human-directed integration strategy for `main...origin/main`; the clean PR branch intentionally uses `origin/main` as source of truth. |
| webspa-admin build and UI runtime | `frontend/webspa-admin` is a gitlink; `.gitmodules` maps it, but `git submodule update --init --recursive frontend/webspa-admin` fails because the configured remote does not contain pinned commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`. | Publish the pinned admin commit to an accessible remote, repoint the gitlink to an available commit after review, or replace the gitlink strategy before running admin build/UI checks. |
| compose runtime start | Required env vars are blank in rendered config; starting would run with missing DB, TIMWE, Auth0/JWT, notification worker, and pgAdmin settings. | Provide local `.env` or export required variables, then run `docker compose up` and service health checks. |
| dependency vulnerability remediation | `npm audit` fix requires breaking Next upgrade to `next@16.2.6`. | Explicit approval for dependency upgrade and follow-up UI regression check. |
| notification vendor/go.sum repair | Fix likely touches `services/notification/go.sum` and/or vendor tree. | Explicit approval for dependency/vendor metadata changes. |
| original local-history branch publish | Original branch carries a 332 MB dump and generated binaries from local-only history. | Do not push that branch. Use clean branch `agent/codex/full-system-verify-pr-20260509-0129` instead. |

## Remaining Risks

- Local and remote `main` diverge with overlapping histories.
- Compose config contains blank required env vars and a secret-shaped checked-in service credential; do not treat compose runtime as production-safe until config is sanitized.
- Build success is not enough for tenant feature verification; several live flows remain blocked by missing local infrastructure and credentials.
- Admin frontend cannot be verified from this checkout because the pinned gitlink commit is unavailable from the configured submodule remote.

## Gaps for /slice-plan

| Feature/service | Evidence of incompleteness | Suggested slice class | Notes |
|---|---|---|---|
| notification API module | default and readonly tests fail | vertical_defect_slice / bounded_enabler | Requires dependency/vendor metadata strategy before code fixes. |
| subscription-partner canonical vendor mode | default build/test fails but readonly mode passes | vertical_defect_slice / bounded_enabler | Decide whether canonical builds should use module mode or vendor tree should include shared common package/deps. |
| webspa-admin | pinned gitlink commit is unavailable from the configured submodule remote | operational_slice | Decide whether to publish the admin commit, repoint the gitlink, or replace the gitlink strategy before UI verification. |
| compose runtime | missing env values and secret-shaped config | operational_slice | Provide safe local env and remove checked-in secret-shaped compose value. |
