# Subscription reporting domain brief

The tenant operations admin uses `/operations` to inspect current subscription state and TIMWE charge totals. The existing `/dashboard` and `/reports` pages describe acquisition transactions; their estimated revenue is a different measure.

The local `subscriptions` table owns observed subscription state. TIMWE's authenticated Charges Consolidated report owns its daily successful billing count and revenue. The report contains no subscriber identifiers, charge attempts, failures, currency, or channel key. Its current-day row can change during the day.

Invariants:

- A dashboard request resolves its tenant from authenticated server context, never a client-selected tenant identifier.
- A TIMWE row is assigned only to the configured tenant, shortcode, and numeric product ID that were sent in the report request.
- A failed or malformed fetch preserves the last successful import and marks it stale; zero means a successful empty report.
- Provider money is a decimal string with an unknown currency until an operator confirms a currency code.
- A provider aggregate is tenant scoped. Channel-specific requests cannot display it as a channel total.
- The importer logs no Basic Auth credentials, full report URL, or subscriber data.

Known gaps: TIMWE has not supplied an event-level feed, a currency definition, or a finalization rule for daily rows. The CSV export was observed through an authenticated browser session on 26 September 2026; it uses `StartDate`, `EndDate`, `Shortcode`, numeric `Product`, and `rs:Format=CSV`.
