# TIMWE subscription health dashboard

The Operations page shows two separate observations for the authenticated tenant:

- **Local subscriptions**: tenant-owned records in `subscriptions`, counted by their current status. This is a point-in-time local database count; one subscriber can hold more than one subscription.
- **TIMWE billing**: successful charges and revenue from the authenticated **Charges Consolidated** report. The importer polls every 15 minutes by default and replaces its rolling 31-day window on each successful import. The provider may revise the current day. The report does not specify a currency or individual subscriber identity, so the dashboard does not infer either.

These figures measure different things. A charge is not a unique active subscriber, and a successful local batch submission is not a TIMWE-confirmed charge. This is a near-real-time report mirror, not a streaming charge feed.
Legacy subscription rows without a tenant ID are excluded from local counts. Confirm the existing tenant backfill before comparing totals.

## Configuration

Apply `services/acquisition-api/migrations/timwe_report_daily.sql` to the target database **before** enabling the importer. The migration creates `timwe_report_syncs`, `timwe_report_daily`, and a tenant/day index for report reads. Use the target environment's normal migration and backup process. Do not put report credentials or the opaque partner token in Git.

Configure the `acquisition-api` service with:

| Variable | Meaning |
| --- | --- |
| `TIMWE_REPORT_URL` | Credential-free HTTPS `reports.timwetech.com/Reportserver/Pages/ReportViewer.aspx?...Charges%20Consolidated...&Partner=<provider token>` URL. Omit per-request dates, shortcode, product and CSV format. |
| `TIMWE_REPORT_USERNAME` | Report server Basic Auth username. |
| `TIMWE_REPORT_PASSWORD` | Report server Basic Auth password. |
| `TIMWE_REPORT_SOURCES` | JSON array mapping each report filter to an active local tenant. Empty disables polling. |
| `TIMWE_REPORT_POLL_INTERVAL` | Duration from `5m` to `24h`; defaults to `15m`. |

Example source mapping, with no secrets:

```json
[{"key":"careerify-6555","tenant_key":"careerify","shortcode":"6555","product_id":32535,"partner_id":1919}]
```

`key` must remain stable. Changing shortcode, product or partner for an existing key is rejected; use a new key and deliberately retire the old one. The importer checks each CSV row against the configured partner, shortcode, and numeric product ID. It rejects report shape changes and does not follow redirects. Only an explicitly configured, credential-free TIMWE HTTPS URL is accepted.

Invalid importer configuration disables the importer and writes an error log; it does not stop the acquisition API. Fix the configuration and restart the service. After a failed poll, the last successful import remains stored and the dashboard shows stale data.

Use a single acquisition API instance for polling where possible. Multiple instances can fetch the same source concurrently; database replacement serializes on the source row and does not add duplicate billings, but it causes redundant provider requests.

## Rollout check

1. Confirm the deployment's tenant key, shortcode, product ID and partner ID from the provider and local catalog. Confirm the report account's authorized scope with TIMWE. Treat the provided Basic Auth password as exposed and rotate it before production use.
2. Back up the target database, apply the migration, deploy the gateway, acquisition API and admin frontend. Keep `TIMWE_REPORT_SOURCES` empty initially.
3. Set the report URL and credentials in the deployment's secret store, set source mapping, then restart the acquisition API. Check logs for `TIMWE report import succeeded` and the source key; logs never include the URL or credentials.
4. Open `/operations` for that tenant. Check the provider's last import time, daily values, and current-day provisional marker against the report. Check that a different tenant sees no provider totals, and that a channel-filtered API request is rejected.
5. A failed import leaves the last good rows in place and marks the source stale. If a source has never succeeded, the provider totals are unavailable (`null`), not zero.

The API is `GET /v1/admin/reports/subscription-health?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD`, through the authenticated admin gateway. Dates are inclusive; up to 31 days are accepted. Optional `productId` and `shortcode` filters are supported. A tenant context is required. Provider revenue is a decimal string and `currency` is `null` until TIMWE confirms it.

## Rollback and limits

To stop polling, clear `TIMWE_REPORT_SOURCES` and restart the acquisition API. The dashboard retains the last imported observations and marks them stale. After preserving an export or backup, the new tables can be removed if the feature is permanently retired. The report does not give attempted charges, failed charges, subscriber-level matches, or a confirmed currency; adding those needs a provider feed or contract.
