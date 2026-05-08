# Existing Feature And Service Inventory

Source: repository intake on 2026-05-08 from service READMEs, docker-compose, routers, handlers, migrations, and task docs.

## Service Surface

| Service | Current role | Evidence paths | Tenant/platform implication |
| --- | --- | --- | --- |
| `services/acquisition-api` | Web acquisition control plane: campaigns, landing analytics, transactions, telco callbacks, header enrichment bootstrap, admin reporting, product/userbase admin, postback admin. | `services/acquisition-api/README.md`, `services/acquisition-api/internal/transport/router.go`, `services/acquisition-api/internal/service/*`, `services/acquisition-api/migrations/*` | Becomes the first tenant-aware workflow owner for campaign, product, userbase, transaction, attribution, and reports. |
| `services/landing-web` | Next.js landing-page runtime and API proxy for campaigns, transactions, analytics, and header-enrichment simulation. | `services/landing-web/app/lp/[slug]/*`, `services/landing-web/app/api/*`, `services/landing-web/README.md` | Needs tenant/campaign public route strategy and tenant-safe campaign asset loading. |
| `services/subscription-external` | Outbound TIMWE integration: opt-in, confirm, opt-out, status, MT/charge, batch, backfill, resubscribe, renewal, charging failures, monitoring. | `services/subscription-external/README.md`, `services/subscription-external/internal/transport/router.go`, `services/subscription-external/internal/handler/*` | Must route outbound actions by tenant/channel credentials rather than global TIMWE settings. |
| `services/subscription-partner` | Inbound partner/TIMWE notification service and legacy subscription/product endpoints. | `services/subscription-partner/README.md`, `services/subscription-partner/internal/transport/router.go` | Inbound events need tenant/channel correlation before notifications, renewals, and postbacks are triggered. |
| `services/notification` | Notification service plus worker for MO, MT delivery notifications, user opt-in/renewed/opt-out/charge events, and MT dispatch. | `services/notification/internal/transport/router.go`, `services/notification/internal/dispatcher/dispatcher.go`, `docker-compose.yml` | Notification list, event ingestion, and worker dispatch must carry tenant/channel context and idempotency. |
| `services/cadence-engine` | Cadence series/content/rule admin API plus scheduler, planner, advancer, and backfill. | `services/cadence-engine/internal/adminhttp/server.go`, `services/cadence-engine/internal/planner/*`, `services/cadence-engine/internal/advancer/*` | Cadence series and message state must be tenant scoped and channel aware. |
| `services/postback-dispatcher` | Async conversion postback outbox dispatcher with retry, circuit breaker, attempt logs, and DLQ. | `services/postback-dispatcher/README.md`, `services/postback-dispatcher/internal/*` | Postback templates, outbox rows, retries, and DLQ admin actions must be tenant/provider scoped. |
| `services/billing` | Billing transaction API and saga/circuit-breaker logic, currently disabled in `docker-compose.yml`. | `services/billing/internal/transport/router.go`, `docker-compose.yml` | Either revive as tenant/channel billing component or keep explicitly out of the first tenant platform slice. |
| `krakend` | API gateway for subscription, notification, billing, acquisition, and landing web. | `krakend/krakend.json`, `krakend/config/*`, `docker-compose.yml` | Natural place to normalize tenant context from host/path/header/JWT before services process requests. |
| `ops` / `config` / `scripts` | Nginx, monitoring, deployment, DB/bootstrap scripts, and environment examples. | `ops/**`, `config/**`, `scripts/**`, `docs/environment-variables.md` | Tenant launch requires deploy/runbook changes, secret hygiene, monitoring labels, and migration playbooks. |
| `frontend/webspa-admin` | Admin frontend directory present in the repo tree. | `frontend/webspa-admin` | Ownership and current completeness need confirmation before assigning UI work; roadmap keeps UI redesign out of early slices. |

## Existing Feature Groups

1. Campaign management: create/list/update/enable/clone campaigns, postback rules, campaign assets, public campaign reads.
2. Web acquisition: transaction create/confirm/status, next-action decisioning, consent/compliance checks, pending transaction reuse, telco callbacks.
3. Attribution and analytics: landing events, click-out handling, provider attribution, charge-success handling, conversion postback generation.
4. Admin management: products, userbase records/imports, activity logs, campaign administration.
5. Reporting: KPIs, funnel, campaign performance, CSV export, time series, transaction stats.
6. Header enrichment: bootstrap, campaign bootstrap, token exchange, landing-web simulator routes.
7. Outbound subscription operations: TIMWE opt-in, confirm, opt-out, status, MT, charge, batch, backfill, resubscribe.
8. Inbound partner notifications: TIMWE notification webhook persistence and downstream callback triggers.
9. Notifications: notification list, MO/MT delivery notifications, opt-in/renewed/opt-out/charge events, worker MT dispatch.
10. Renewal and charging operations: renewal worker control, priority retry, churn candidates, manual renewal, charging failure summaries and health.
11. Cadence: cadence series CRUD, rule/content management, CSV import, publish, scheduler/planner/advancer/backfill.
12. Postbacks: outbox dispatch, retry/backoff, circuit breaker, attempt logs, DLQ requeue and status views.
13. Gateway and routing: KrakenD gateway plus Nginx/header-enrichment bootstrap configuration.
14. Platform operations: Docker Compose deployments, Postgres, Redis, MinIO campaign assets, pgAdmin, Prometheus/Grafana monitoring.

## Cross-Cutting Gaps For Tenant Multi-Channel Platform

1. Tenant identity is not a first-class invariant across the data model, request context, reports, workers, or outbox rows.
2. Channel configuration exists implicitly as hard-coded realm/channel/provider paths and env vars, not as tenant-owned catalog data.
3. Provider credentials are mostly global environment/config values; tenant/channel secret references need a backend and redacted admin API.
4. Public routing needs one tenant resolution policy across landing pages, campaign reads, transaction starts, callbacks, and gateway forwarding.
5. Workers need tenant/channel claim and idempotency keys so cadence, notifications, renewals, and postbacks cannot cross tenant boundaries.
6. Migration work must backfill legacy global rows into a default tenant before constraints can be enforced.
7. Reports and monitoring must distinguish tenant-scoped views from platform-wide operator views.
8. Configuration contains sensitive environment values in local compose files; tenant credential work should include secret hygiene and rotation posture.
