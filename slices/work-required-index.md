# Work Required Index

This index translates the roadmap into executable platform work by repository layer. It is meant to be the quick intake view before creating backlog issues or implementation branches.

## Foundation

| Work | Primary slices | Primary surfaces |
| --- | --- | --- |
| Tenant claim, role, trust boundary, and service-to-service auth contract | TMP-018 | `common/auth/auth0jwt`, admin middleware, `krakend`, request context |
| Tenant model, status, membership, service-account identity, and audit fields | TMP-001 | `common/auth`, acquisition admin middleware, DB migrations |
| Tenant context propagation through trusted gateway/auth boundary | TMP-001, TMP-012 | `krakend`, admin middleware, service request context |
| Safe tenant/channel logging, metrics, traces, and worker health baseline | TMP-020 | HTTP middleware, Prometheus metrics, worker instrumentation |
| Tenant-scoped product, userbase, activity-log repositories | TMP-002 | `services/acquisition-api/internal/repository`, existing admin endpoints |
| Tenant source of truth and isolation ADR | TMP-001, TMP-011 | `docs/`, migrations, deployment runbooks |

## Channel Platform

| Work | Primary slices | Primary surfaces |
| --- | --- | --- |
| Channel catalog with provider, country/operator, capability, status, callback config | TMP-003 | acquisition admin API, channel tables, admin portal |
| Credential reference binding, redaction, activation, rotation posture | TMP-004, TMP-015 | secret adapter, config, channel credential API |
| Capability enforcement for opt-in, confirm, opt-out, status, MT, charge, callbacks, postbacks | TMP-003, TMP-007, TMP-017 | acquisition, subscription-external, notification, billing |
| Partner/channel onboarding contracts and fixtures | TMP-016 | docs, OpenAPI/Postman, sandbox fixtures |

## Acquisition And Landing

| Work | Primary slices | Primary surfaces |
| --- | --- | --- |
| Tenant campaign binding and per-tenant slug uniqueness | TMP-005 | campaign domain, repository, migrations, admin API |
| Tenant-scoped campaign asset keys and public-safe asset DTOs | TMP-019 | campaign asset service, MinIO/S3, presigned URL endpoint |
| Deterministic public tenant routing for landing, campaign read, HE, and callbacks | TMP-012 | `landing-web`, `acquisition-api`, `krakend`, Nginx |
| Tenant-scoped transaction lifecycle, consent, attribution, HE, next action | TMP-006 | transaction service/repository, TIMWE client, landing API proxy |
| Legacy default-tenant compatibility for existing Ghana/TIMWE routes | TMP-006, TMP-012, TMP-011 | route resolver, migration, smoke tests |

## Subscription, Billing, Notification, And Cadence

| Work | Primary slices | Primary surfaces |
| --- | --- | --- |
| Tenant/channel outbound routing for opt-in, confirm, opt-out, status, MT, charge | TMP-007 | `subscription-external`, TIMWE client, partner APIs |
| Inbound callback correlation and replay/quarantine behavior | TMP-013 | `subscription-partner`, notification endpoints, acquisition callbacks |
| Notification event tenant filtering and MT worker tenant/channel dispatch | TMP-008, TMP-013 | `services/notification`, notification-worker, message outbox |
| Cadence series/content/rules/message state tenant scoping | TMP-008 | `services/cadence-engine`, cadence migrations |
| Billing/charge ownership decision and tenant charge proof | TMP-017 | `services/subscription-external`, `services/billing`, reports |
| Renewal and charging-failure tenant scoping | TMP-007, TMP-017, TMP-010 | subscription-external renewal/charging failure handlers |

## Postbacks, Reporting, And Operations

| Work | Primary slices | Primary surfaces |
| --- | --- | --- |
| Tenant/provider postback templates, outbox, retry, DLQ, admin retry | TMP-009 | acquisition postback domain, postback-dispatcher |
| Tenant/channel KPI, funnel, campaign, transaction, queue, charge, and failure reports | TMP-010 | acquisition reports, subscription monitoring, Grafana |
| Secret hygiene, safe config, tenant/channel metric labels, runbooks | TMP-015 | compose/config/docs/ops monitoring |
| Migration backfill and constraints for legacy global data | TMP-011 | migrations, SQL verification, rollback runbook |

## Admin Portal

| Work | Primary slices | Primary surfaces |
| --- | --- | --- |
| Tenant workspace shell, tenant selector for platform operators, tenant assignment denial | TMP-014 | `frontend/webspa-admin`, Auth0 callback, API clients |
| Tenant-aware campaign/product/userbase/cadence/postback/report views | TMP-014 with TMP-002/TMP-005/TMP-008/TMP-009/TMP-010 | admin frontend modules |
| UI handling for 403/404 tenant denials and disabled tenants | TMP-014 | route guards, interceptors, empty states |

## Release Readiness

| Work | Primary slices | Primary surfaces |
| --- | --- | --- |
| Slice value-gate reports with named test evidence | All TMP slices | `slices/<id>/value-gate-report.md` |
| Claim/auth, observability, asset namespace, and public route contracts settled before acquisition rollout | TMP-018, TMP-020, TMP-019, TMP-012 | ADRs, middleware, storage config, route resolver |
| Tenant platform migration dry run and rollback proof | TMP-011 | migration scripts, runbooks |
| Partner onboarding validation pack | TMP-016 | docs and sandbox fixtures |
| Production deploy checklist: secrets, metrics, health, default tenant, gateway trust | TMP-015, TMP-011 | ops docs and deployment config |
