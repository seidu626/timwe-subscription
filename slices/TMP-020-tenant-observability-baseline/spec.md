# TMP-020 Spec: Tenant Observability Baseline

## Story

As an operations analyst, I want tenant/channel health signals in logs, metrics, and health responses so that I can diagnose tenant issues without exposing PII or confusing one tenant's issue with another.

## Scope

- Add safe tenant/channel label helpers in the modules touched by this slice.
- Add cadence admin HTTP logging middleware that emits safe tenant/channel/request id fields and avoids query/body/header dumps.
- Add notification worker dispatch outcome metric/log fields with tenant/channel/worker/status labels.
- Remove full request logging from notification request paths touched by tenant routing.
- Advertise observability status on notification and cadence health endpoints.

## Acceptance Criteria

1. HTTP request logging includes safe tenant/channel/request id when context is resolved and `unknown` tenant/trust source for early denials.
2. Notification worker dispatch metrics include bounded tenant/channel/worker/status labels.
3. PII and secret label keys such as `msisdn`, `click_id`, `authorization`, `token`, and `secret` are rejected.
4. Notification worker logs do not include MSISDN, message text, body, click id, raw headers, or secrets.
5. Notification service no longer logs full `fasthttp.Request.String()` values for notification and unknown-route paths.
6. Notification and cadence health endpoints report observability status without tenant-specific values.

## Out of Scope

- Full distributed tracing deployment.
- Grafana dashboard redesign.
- Broad cleanup of older acquisition, subscription-external, and postback logs.
- Direct Prometheus scrape server for the notification worker process.
