# Full-System Verification Matrix

## Current Verification Refresh: 2026-05-10

Verdict:
- Local release-verification slices are complete in isolated branch `agent/codex/fullsystem-20260510-045911`.
- Previous blockers for webspa-admin reproducibility, compose runtime schema provisioning, landing-web dependency remediation, and local-main strategy have been resolved by TMP-046, TMP-045, TMP-047, and TMP-038 respectively.
- No destructive merge/reset was performed against the primary checkout. The recorded strategy is to preserve primary local `main` and use this origin/main-derived worktree branch as the current release verification surface.

Current command evidence:

| Area | Command | Result |
|---|---|---|
| Control plane | `agent-supervisor` preflight with worktree-local config | passed; only stale superseded rows `TMP-011-repair-1` and `TMP-015-repair-1` remain |
| Classifier | `hvc check agent/backlog/issues/*.md --fail-on block` | passed |
| Local service builds | `make build-all-local` | passed |
| Common tests | `cd common && go test ./...` | passed |
| Service tests | `go test ./...` in subscription-external, subscription-partner, billing, notification, acquisition-api, postback-dispatcher, and cadence-engine | passed |
| Landing web audit/build | `cd services/landing-web && npm audit --audit-level=moderate && npm run build` | passed; zero audit vulnerabilities |
| Landing web runtime | standalone Next server on port 3138 plus `curl /` | passed with HTTP 200 |
| Admin frontend | `cd frontend/webspa-admin && npm run build` | passed with existing SCSS/selector warnings |
| Admin tests | `CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false` | passed, `TOTAL: 84 SUCCESS` |
| Compose config | `docker compose --env-file .env.example -f docker-compose.yml config --quiet` | passed |
| Compose runtime smoke | temporary `shared-network`, `docker compose --project-name timwe-codex-fullsystem --env-file .env.example -f docker-compose.yml up --build -d database redis db-bootstrap acquisition-api notification-worker cadence-engine postback-dispatcher` | passed for bootstrap and startup; all selected services were `Up` after db-bootstrap completion |

Runtime caveats:
- Acquisition-api logs a Redis fallback warning because its runtime config still attempts `127.0.0.1:6379`; service startup continues with in-memory fallback.
- Next 16 logs a deprecation warning for `middleware.ts` in favor of `proxy`; build and runtime smoke pass.
- Primary local `/home/xper626/workspace/apps/timwe-subscription` still diverges from `origin/main`; TMP-038 records the non-destructive strategy rather than mutating that checkout.
- Production deploy and live TIMWE/Auth0/provider credential flows were not in scope for this local full-system verification.

TMP-042 refresh note, 2026-06-08:
- `docs/agent/release-decision-packet-2026-05-09.md` was refreshed against the current evidence above.
- No sibling worker result capsules were present in `.harness/runs/parallel-TMP042-20260608-0742/`; this refresh is bounded to the scoped context pack, this verification matrix, and TMP-042 state files.
- The historical command and failure ledgers below are retained for traceability. Current release-decision status is summarized in the refreshed packet and in the blocked-check/risk sections at the end of this document.

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
| subscription-partner | backend API | `services/subscription-partner/go.mod`, `cmd/main.go` | `make build-local-subscription`, `make build-all-local` | `go test ./...` | `make start-subscription` | `/health` | Redis, Postgres, shared common module | passed |
| billing | backend API | `services/billing/go.mod`, `cmd/main.go` | readonly module-mode build | `go test ./...` | `make start-billing` | `/health` | Postgres for live flows | passed |
| notification API | backend API | `services/notification/go.mod`, `cmd/main.go` | `make build-local-notification`, `make build-all-local` | `go test ./...` | `make start-notification` | `/health` | Postgres, Redis, auth common module | passed |
| notification worker | worker | `services/notification/cmd/notification-worker/main.go` | `make build-local-notification-worker`, `make build-all-local` | covered by notification package tests | compose service | Prometheus metrics handler | Postgres, MT endpoint config, message outbox schema | passed in current matrix with live-flow caveats |
| acquisition-api | backend API | `services/acquisition-api/go.mod`, `cmd/main.go`, migrations | readonly module-mode build; compose image build | `go test ./...` | `make start-acquisition-api` / compose service | service routes and compose dependency health | Postgres, MinIO, TIMWE/Auth0 config for live flows | passed in current matrix with Redis fallback caveat |
| cadence-engine | worker/admin HTTP | `services/cadence-engine/go.mod`, `cmd/cadence-engine/main.go` | readonly module-mode build | `go test ./...` | `make start-cadence-engine` | admin HTTP on `:8091` | Postgres | passed |
| postback-dispatcher | worker | `services/postback-dispatcher/go.mod`, `cmd/main.go` | readonly module-mode build | `go test ./...` | compose service | worker starts against DB | Postgres, postback outbox schema | passed in current matrix with live-flow caveats |
| landing-web | Next.js frontend | `services/landing-web/package.json`, `app/**` | `npm run build` | build/typecheck; no route tests present | `npm run dev` / `npm start` | Next route build output | Node 24.15.0, npm 11.12.1, acquisition API at runtime | fixed |
| webspa-admin | admin frontend gitlink plus nested checkout | `frontend/webspa-admin` gitlink; admin checkout | `npm run build` | ChromeHeadless tests | unavailable from TMP-042 docs-only refresh | rerun on release candidate | current matrix records build/test pass after TMP-046 | passed in current matrix |
| KrakenD gateway | gateway | `krakend/**`, `Makefile` | `make docker-build-krakend` | `make krakend-query-forwarding-check` | compose service | `krakend check` not run; query-forwarding check run | Docker/Podman, KrakenD image | partially verified |
| docker compose dev stack | local integration stack | `docker-compose.yml` | `docker compose --env-file .env.example -f docker-compose.yml config` | config render plus selected compose smoke | `make compose-up` | selected service startup after `db-bootstrap` | real env/provider values and external network for live flows | locally verified with caveats |
| tenant migration runner | migration/ops script | `scripts/db-migrate-tenant-platform.sh` | `bash -n scripts/db-migrate-tenant-platform.sh` | `make -n db-migrate-tenant-platform-dry-run` | `make db-migrate-tenant-platform-dry-run` | dry-run output against DB | Postgres credentials | partially verified |

## Feature Inventory

| Feature | Evidence of implementation | Owning service/component | Public interface | Critical invariants | Verification method | Status |
|---|---|---|---|---|---|---|
| tenant context and service auth contract | `common/auth/auth0jwt`, `common/auth/tenantctx`, TMP-018 reports | common | JWT claims and trusted service headers | tenant identity cannot be forged; replay nonce rejected | `go test ./...` in common | fixed |
| tenant admin management scope | `services/acquisition-api/internal/handler`, TMP-002 report | acquisition-api | `/v1/admin/products`, `/v1/admin/userbase` | tenant filtering and authorization | `go test ./...` in acquisition-api | passed |
| channel catalog and credential binding | acquisition migrations/handlers, TMP-003/TMP-004 reports | acquisition-api | `/v1/admin/channels`, credential binding routes | credential references only, no secret exposure | acquisition-api tests plus docs review | partially verified |
| tenant campaign binding and public routing | acquisition migrations/handlers, landing-web routes | acquisition-api, landing-web | `/v1/admin/campaigns`, `/lp/:slug`, `/lp/:tenant/:slug` | overlapping slugs resolve deterministically | acquisition-api tests; landing-web build | fixed |
| tenant acquisition flow | acquisition transaction handlers, landing-web flow | acquisition-api, landing-web | `/v1/acquisition/transactions`, confirm route | consent, HE, attribution, tenant/campaign match | acquisition-api tests; landing-web build | partially verified |
| subscription routing by tenant channel | subscription-external and subscription-partner services | subscription-external, subscription-partner | subscription external/admin and partner endpoints | no global credentials when tenant/channel required | subscription-external and subscription-partner tests pass; canonical local build passes | passed |
| notification and cadence routing | notification tests, cadence tests, TMP-008 report | notification, cadence-engine | notification list, cadence admin HTTP | tenant/channel context preserved | notification and cadence tests pass; canonical local build passes | passed |
| postback attribution routing | acquisition postback admin and dispatcher | acquisition-api, postback-dispatcher | postback admin routes and dispatcher worker | tenant/provider-specific recovery | acquisition and dispatcher tests | passed |
| tenant reporting operations | acquisition reporting handlers, TMP-010 report | acquisition-api | reporting endpoints | tenant/channel filters avoid leakage | acquisition-api tests | passed |
| billing charge ownership | TMP-017 decision, billing service | subscription-external, billing | charge endpoints and billing routes | single owner for tenant charge state | billing tests pass; subscription external tests pass | passed |
| tenant asset namespacing | acquisition storage config and handlers, TMP-019 report | acquisition-api | campaign asset presign route | tenant-scoped object keys | acquisition-api tests | passed |
| observability baseline | notification observability tests, compose monitoring | notification, ops monitoring | metrics, logs, dashboards | safe bounded labels, no PII labels | notification observability tests and worker build pass; live compose monitoring remains env-blocked | partially verified |
| partner onboarding contracts | docs and examples under TMP-016 | docs/examples | onboarding document and fixture validator | versioned tenant/channel contract | prior evidence plus HVC | passed |

## Environment Readiness

| Requirement | Source | Available? | Action |
|---|---|---|---|
| Go toolchain | `go.mod` files | yes: `go1.26.2-X:nodwarf5` | Used for Go tests/builds. |
| Node/npm | `services/landing-web/package.json` | yes: Node `v24.15.0`, npm `11.12.1` | Ran `npm ci`, `npm run build`, `npm audit`. |
| Docker/Compose | `docker-compose.yml` | yes: Podman Docker emulation, Compose `5.1.3` | Rendered compose config with `.env.example`; temporary empty Docker auth files allowed anonymous builder-image pulls without changing credentials; bounded smoke now reaches app startup and records service-specific runtime blockers. |
| Postgres/Redis live dependencies | compose and service configs | no/unknown | Mark live runtime and DB migration checks blocked unless env vars and local stack are provided. |
| TIMWE/Auth0/provider credentials | compose/service configs | no | External-provider flows marked blocked or partially verified by tests only. |
| webspa-admin submodule content | gitlink `frontend/webspa-admin` | yes in current matrix | The 2026-05-10 refresh records the previous reproducibility blocker as resolved by TMP-046; `npm run build` passed and ChromeHeadless tests reported `TOTAL: 84 SUCCESS`. Rerun on the release candidate branch before release. |
| landing-web dependencies | `package-lock.json` | yes after `npm ci` | Build passed; audit reports unresolved Next/PostCSS vulnerabilities. |

## Service Verification Matrix

| Service/component | Build | Unit tests | Integration tests | Migrations | Runtime start | Health/smoke | Status | Evidence |
|---|---|---|---|---|---|---|---|---|
| common | fixed | fixed | not run | n/a | n/a | n/a | fixed | TMP-023 made `cd common && go test ./...` pass. |
| subscription-external | passed | passed | not run live | migrations discovered | not run | not run live | passed | `go test ./...` passed; readonly build passed; canonical make built this service before failing later. |
| subscription-partner | passed | passed | not run live | n/a | not run | not run live | passed | Current `go test ./...` passed and `make build-all-local` built this service. |
| billing | passed | passed | not run live | n/a | not run | not run live | passed | `go test ./...` passed; readonly build passed. |
| notification API | passed | passed | not run live | n/a | not run | not run live | passed | Current `go test ./...` passed and `make build-all-local` built this service. |
| notification worker | passed | passed | selected compose startup passed | message outbox provisioning covered by current matrix | passed | passed with caveats | locally verified with caveats | Current refresh records selected compose smoke passed after TMP-045/db-bootstrap; production/live provider flows remain out of scope. |
| acquisition-api | passed | passed | selected compose startup passed | schema provisioning covered by current matrix | passed | passed with Redis fallback caveat | locally verified with caveats | Current refresh records selected compose smoke passed after TMP-045/db-bootstrap; acquisition-api still logs Redis fallback to in-memory mode. |
| cadence-engine | passed | passed | not run live | n/a | not run | not run live | passed | `go test ./...` passed; readonly build passed. |
| postback-dispatcher | passed | passed | selected compose startup passed | postback outbox provisioning covered by current matrix | passed | passed with caveats | locally verified with caveats | Current refresh records selected compose smoke passed after TMP-045/db-bootstrap; production/live provider flows remain out of scope. |
| landing-web | fixed | build/typecheck passed | not run live | n/a | not run | route build output passed | fixed | Initial `npm run build` failed; TMP-022 patch made `npm run build` pass. |
| webspa-admin | passed in current matrix | passed in current matrix | not rerun by TMP-042 | n/a | not rerun by TMP-042 | not rerun by TMP-042 | passed in current matrix | Current refresh records `npm run build` passed and ChromeHeadless tests reported `TOTAL: 84 SUCCESS` after TMP-046. |
| docker compose dev stack | passed in selected smoke | n/a | selected smoke passed after db-bootstrap | n/a | selected services stayed `Up` | selected startup passed | locally verified with caveats | Current refresh records compose config passed and selected compose smoke passed after TMP-045/db-bootstrap; production deploy and live TIMWE/Auth0/provider flows remain out of scope. |

## Feature Verification Matrix

| Feature | Verification path | Command/check | Expected signal | Actual result | Status | Evidence |
|---|---|---|---|---|---|---|
| tenant context and auth | common package tests | `go test ./...` in `common` | all auth tests pass | pass after TMP-023 | fixed | `TestMiddlewareRejectsReplayNonce` now rejects replay. |
| admin and acquisition APIs | Go tests | `go test ./...` in `services/acquisition-api` | pass | pass | passed | acquisition-api package tests passed. |
| subscription external tenant routing | Go tests | `go test ./...` in `services/subscription-external` | pass | pass | passed | service/domain/handler/repository/worker tests passed. |
| subscription partner routes | Go tests | `go test ./...` | pass | pass | passed | current default tests passed. |
| notification tenant/cadence routing | Go tests | `go test ./...` | pass | pass | passed | current default tests passed. |
| landing public tenant routing | Next build | `npm run build` | pass and route output includes legacy and tenant-qualified routes | pass after TMP-022 | fixed | build lists `/lp/[tenant]` and `/lp/[tenant]/[slug]`. |
| tenant migration dry-run entrypoint | shell/make checks | `bash -n`; `make -n db-migrate-tenant-platform-dry-run` | syntax and target resolve | pass | partially verified | DB-backed dry-run blocked by missing Postgres env. |
| KrakenD query forwarding | script check | `make krakend-query-forwarding-check` | pass | pass | passed | check passed against `krakend/krakend.json`. |
| compose runtime stack | compose render plus bounded smoke | `docker compose --env-file .env.example -f docker-compose.yml config`; bounded `docker compose ... up -d --build ...` smoke | config renders; selected smoke reaches app startup after db-bootstrap | config pass; selected compose smoke passed in current refresh | locally verified with caveats | live TIMWE/Auth0/provider flows and production deploy proof remain out of scope. |

## Commands Run

| Time/order | Command | Purpose | Result | Evidence/log summary |
|---|---|---|---|---|
| 1 | `git status --short --branch` | Session-start safety and git state | passed | Initial session snapshot: main checkout had no uncommitted files; local `main` was ahead 51 and behind `origin/main` by 2. Current divergence is refreshed in command 44 and the blocked-check row. |
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
| 28 | `cd services/subscription-partner && go test ./...` | Re-run subscription-partner default tests on current `origin/main` | passed | All packages passed or had no test files. |
| 29 | `cd services/notification && go test ./...` | Re-run notification default tests on current `origin/main` | passed | Dispatcher, handler, observability, repository, service, and transport tests passed. |
| 30 | `make build-all-local` | Re-run canonical local service build on current `origin/main` | passed | subscription-external, subscription-partner, billing, notification API, notification worker, acquisition-api, and cadence-engine built successfully. |
| 31 | `make clean` plus `git restore --source=HEAD -- services/notification/notification-worker` | Remove generated build artifacts before evidence-only commit | passed | Worktree returned to evidence-only changes. |
| 32 | `docker compose --env-file .env.example -f docker-compose.yml config` | Verify compose renders from safe placeholder env scaffold | passed | Config rendered without relying on checked-in subscription DB credential material. |
| 33 | `rg -n 'APP_DATABASE_POSTGRESQL_HOST=139\|APP_DATABASE_POSTGRESQL_PASSWORD=[^$]' docker-compose.yml \|\| true` | Confirm previous hardcoded subscription DB host/password patterns are absent | passed | No matches. |
| 34 | bounded `docker compose --project-name timwe_smoke_024125 --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-smoke-override.yml up -d --build ...` | Attempt runtime smoke after TMP-028 while avoiding unrelated Redis host-port conflict and creating missing `shared-network` temporarily | blocked | Compose failed before app containers started because local Docker/Podman registry auth could not pull `docker.io/library/golang:1.24-alpine`; cleanup removed the temporary network and override file. |
| 35 | `docker pull docker.io/library/golang:1.24-alpine` | Reproduce compose builder-image pull failure outside compose | blocked | Same local Docker/Podman registry auth failure reproduced; no credential repair attempted. |
| 36 | `DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker pull docker.io/library/golang:1.24-alpine` | Test anonymous builder-image pull without mutating local Docker credentials | passed | Temporary empty auth files bypassed the local auth/tooling blocker. |
| 37 | `DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker compose --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml build acquisition-api` | Verify TMP-030 acquisition-api compose image build | passed | Image build completed after repo-root context and readonly module-mode Dockerfile fix. |
| 38 | bounded full `docker compose ... up -d --build ...` smoke with temporary auth, Redis override, and temporary `shared-network` | Re-run compose startup after acquisition image build fix | partially passed | subscription `/health`, notification API `/health`, cadence `/health`, and landing-web `/` returned 200. acquisition-api `/health` returned no response because the container exited; notification-worker and postback-dispatcher also exited/retried on runtime config blockers. |
| 39 | targeted acquisition-api runtime probe with database, redis, minio, minio-init, and acquisition-api | Isolate acquisition runtime failure after image build passed | blocked | Acquisition API connected to DB, then exited with admin schema bootstrap failure: migration `add_admin_management_tables.sql` expects relation `products`. |
| 40 | `docker compose --env-file .env.example -f docker-compose.yml config` | Verify TMP-031 compose env renders | passed | Compose config rendered with notification-worker DB env. |
| 41 | targeted `docker compose --project-name timwe_tmp031 --env-file .env.example -f docker-compose.yml up -d --build database notification-worker` | Verify notification-worker DB env/startup | passed with downstream schema warning | Worker state was `running`, logged `notification worker started`, and started metrics at `:9103`; dispatcher then logged missing `message_outbox`, a separate schema provisioning blocker. |
| 42 | `docker compose --env-file .env.example -f docker-compose.yml config` | Verify TMP-032 compose env renders | passed | Compose config rendered with postback-dispatcher DB env aliases. |
| 43 | targeted `docker compose --project-name timwe_tmp032 --env-file .env.example -f docker-compose.yml up -d --build database postback-dispatcher` | Verify postback-dispatcher DB env/startup | passed with downstream schema warning | Dispatcher state was `running`, logged database connection established, and started polling; polling then logged missing `postback_outbox`, a separate schema provisioning blocker. |
| 44 | `git -C /home/xper626/workspace/apps/timwe-subscription status --short --branch --untracked-files=all`; `git -C /home/xper626/workspace/apps/timwe-subscription rev-list --left-right --count main...origin/main`; `gh pr list --state open --json number,title,headRefName,url` | Refresh TMP-038 local-main integration evidence without merging or resetting | blocked | At that refresh, primary checkout was clean but `main...origin/main` was ahead 51 and behind 32; primary head was `ab22b15f7c8f6ea8df951a04f3201027c00de06e`, `origin/main` was `5a6e89aa0e762ccd84d23ba3e6a691320d334517`, merge-base was `b86522933b13108dd7165f0f91618a59c378d5bc`, and there were no open PRs. |
| 45 | `git fetch --prune origin`; `git rev-parse origin/main`; `git rev-list --left-right --count main...origin/main`; `gh pr list --state open`; `gh issue list --state open` | Record a TMP-038 local-main evidence snapshot after release evidence updates | blocked | At the 2026-05-09T08:44:16Z refresh, primary checkout was clean but `main...origin/main` was ahead 51 and behind 38; primary head was `ab22b15f7c8f6ea8df951a04f3201027c00de06e`, `origin/main` was `bad5fe156f876938ff10895a5a330178c95bb8de`, and there were no open PRs or issues. Rerun the command for the current count before an integration decision. |

## Failure Ledger

| Failure | Command/check | Root cause | Patch | Re-verification | Status |
|---|---|---|---|---|---|
| Local branch cannot fast-forward or cleanly merge `origin/main` | `git merge --no-edit origin/main` | Local `main` and `origin/main` contain divergent overlapping history with add/add conflicts, including generated/vendor files and slice evidence. | Created clean PR branch from `origin/main` and cherry-picked only the verified audit/repair commit. | Clean branch has no oversized blobs and re-ran manifest, HVC, common tests, and landing-web build. | fixed for PR branch; local main integration still blocked |
| landing-web production build failed | `npm run build` | Next.js App Router rejected sibling dynamic segment names `[slug]` and `[tenant]` at the same route level. | TMP-022 renamed single-segment dynamic folders to `[tenant]` and mapped absent `slug` to the single segment. | `npm run build` passed. | fixed |
| common package fails | `go test ./...` in `common` | Generator API drift, postgres test signature drift, and nonce replay test clock mismatch. | TMP-023 excluded tool-only generator helper, updated postgres tests, and aligned nonce store test clock. | `cd common && go test ./...` passed. | fixed |
| notification package stale failure | `go test ./...` | Historical dependency/vendor failure no longer reproduces on current `origin/main`. | No source change; TMP-027 retired the stale blocker in evidence. | `cd services/notification && go test ./...` and `make build-all-local` passed. | fixed |
| subscription-partner stale canonical failure | `go test ./...`, `make build-all-local` | Historical vendor-mode failure no longer reproduces on current `origin/main`. | No source change; TMP-027 retired the stale blocker in evidence. | `cd services/subscription-partner && go test ./...` and `make build-all-local` passed. | fixed |
| compose smoke fails before app startup | bounded `docker compose ... up -d --build ...`; `docker pull docker.io/library/golang:1.24-alpine` | Local Docker/Podman registry auth cannot pull the Go builder image. | No credential or dependency change made; TMP-029 records evidence and cleanup. | Direct image pull reproduced the same blocker. | blocked by local tooling/auth |
| acquisition-api compose image build fails | bounded `docker compose ... build acquisition-api` | Service-only Docker build context cannot see `../../common`, and Dockerfile forces `-mod=vendor` without a service-local vendor tree. | TMP-030 uses repo-root build context, explicit Dockerfile path, copies `common` and service files into matching paths, and builds with `-mod=readonly`. | Isolated-auth compose build for acquisition-api passed. | fixed |
| acquisition-api exits during compose runtime | targeted acquisition-api runtime probe | Admin schema migration `add_admin_management_tables.sql` expects relation `products` before the runtime schema has created it. | No schema change in TMP-030; recorded as downstream defect candidate. | Targeted probe reproduced the blocker after image build passed. | blocked |
| notification-worker exits during DB ping | targeted notification-worker compose smoke | Worker compose env only overrode DB host, leaving incomplete/default local DB connection settings for a process that pings DB at startup. | TMP-031 passes DB port, user, password, database, and `sslmode=disable` into notification-worker compose env. | Targeted smoke left worker running and logged worker/metrics startup. | fixed |
| notification-worker dispatcher logs missing outbox | targeted notification-worker compose smoke | Empty compose DB has not applied subscription-external message cadence migration that creates `message_outbox`. | No schema change in TMP-031; recorded as downstream schema provisioning blocker. | Worker startup passed, then dispatcher logged missing relation. | blocked |
| postback-dispatcher uses localhost DB in compose | targeted postback-dispatcher compose smoke | Compose used `DATABASE_POSTGRESQL_*` env names, but `common/config` reads `DB_POSTGRESQL_*` or `APP_DATABASE_POSTGRESQL_*`, so defaults were used. | TMP-032 adds `DB_POSTGRESQL_*` aliases and `DB_POSTGRESQL_SSL_MODE=disable` to postback-dispatcher compose env. | Targeted smoke logged database connection established and dispatcher startup. | fixed |
| postback-dispatcher logs missing postback outbox | targeted postback-dispatcher compose smoke | Empty compose DB has not applied postback outbox migrations. | No schema change in TMP-032; recorded as downstream schema provisioning blocker. | Dispatcher startup passed, then polling logged missing relation. | blocked |

## Blocked Checks

| Check | Reason | Exact command or requirement to unblock |
|---|---|---|
| Production release/deploy decision | Production deploy and live TIMWE/Auth0/provider credential flows were not in scope for the local full-system verification refresh. TMP-042 is not authorized to make that decision. | Release owner approval, live or sandbox credential readiness, deploy/runbook evidence, and provider-flow proof on the chosen release candidate. |
| Verify primary checkout as the release surface | The current matrix records a non-destructive TMP-038 strategy: preserve primary local `main` and use an origin/main-derived worktree branch as the verification surface. | Maintainer decision if release must run from `/home/xper626/workspace/apps/timwe-subscription` primary `main`; no merge/reset/archive is approved by TMP-042. |
| Release-candidate rerun after branch or env changes | Local full-system verification was recorded on `agent/codex/fullsystem-20260510-045911`. Branch, compose, migration, env, provider, or credential changes can invalidate that proof. | Rerun the full-system matrix on the final release candidate with the exact env/provider scope the release owner chooses. |
| Historical local-history branch publish | Original local-history branches may carry oversized local-only blobs according to the historical ledger. | Do not push those branches. Use a clean origin-derived verification or integration branch. |

## Remaining Risks

- A local verification pass is not a production release decision.
- Live TIMWE/Auth0/provider flows, deploy runbooks, and production credentials still need explicit release-owner evidence.
- The primary checkout remains a separate integration concern if it must be the release surface; TMP-042 does not merge, reset, archive, or rewrite it.
- Redis fallback and Next middleware deprecation warnings are caveats to review before release hardening, even though local build/runtime smoke passed.
- This TMP-042 refresh did not rerun code, compose, admin, or browser checks; it refreshed the packet from scoped existing evidence only.

## Gaps for /slice-plan

| Feature/service | Evidence of incompleteness | Suggested slice class | Notes |
|---|---|---|---|
| production/live provider proof | current matrix excludes production deploy and live TIMWE/Auth0/provider credential flows | operational_slice | Requires release-owner scope, credentials, and runbook approval before execution. |
| primary checkout release-surface decision | current strategy preserves local `main` and uses an origin/main-derived verification branch | operational_slice | Only needed if the maintainer requires primary checkout integration before release. |
| release-candidate rerun | verification branch is recorded as `agent/codex/fullsystem-20260510-045911` | operational_slice | Rerun the full-system matrix if a different branch or env becomes the release candidate. |
