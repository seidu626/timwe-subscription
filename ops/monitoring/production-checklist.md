# Platform Ops Production Checklist

Use this checklist before turning on the tenant/channel monitoring stack in a shared or production-like environment.

## Secret Hygiene

- [ ] `GRAFANA_ADMIN_PASSWORD` is set in the environment and is not committed as a literal value.
- [ ] `NOTIFICATION_WORKER_METRICS_ADDR` is set explicitly if the default bind address is not acceptable for the environment.
- [ ] Credential material is provided through environment variables or secret refs, not checked-in passwords or API keys.
- [ ] Documentation examples use redacted or placeholder values only.

## Observability Labels

- [ ] Tenant and channel metrics use bounded labels only: `tenant_id`, `channel_id`, `worker`, and `status`.
- [ ] No metric label carries `msisdn`, `click_id`, `token`, `secret`, or another high-cardinality / PII value.
- [ ] Dashboard panels use `topk` or aggregated views where label cardinality could grow.

## Scrape and Dashboard Readiness

- [ ] Prometheus scrapes the notification worker metrics endpoint.
- [ ] Grafana loads the tenant/channel operations dashboard without provisioning errors.
- [ ] The dashboard shows dispatch success, retry, and failure views for the worker.

## Failure-Mode Checks

- [ ] Missing or invalid secret references fail closed in the credential resolver tests.
- [ ] Unsafe label values are rejected by the observability label tests.
- [ ] The worker metrics endpoint is available before the dashboard is considered ready.

## Useful Verification Commands

```bash
go test ./services/notification/internal/observability ./services/notification/internal/dispatcher
go test ./services/subscription-external/internal/service -run 'TestEnvProviderCredentialResolver|Test.*TenantRouting'
jq empty ops/monitoring/grafana/provisioning/dashboards/dashboard-tenant-channel-ops.json
```
