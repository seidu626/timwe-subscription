# Subscription processing PostgreSQL reliability

Repair the PostgreSQL failures observed in the 5,000-record subscription-only run without repeating any provider opt-in or sending application SMS.

Observed evidence: job 71248d2a-5f18-4765-9321-a6124b87fa33 processed rows 5,051–10,050. 4,545 subscriptions were saved, 25 additional TIMWE successes were not persisted, 5 eligibility reads hit PostgreSQL connection exhaustion, and all 26 BLACKLISTED persistence attempts failed because ON CONFLICT did not match live tenant-scoped constraints. Invalid-number persistence succeeded for 277 newly rejected numbers. The worker's sampled error batches lost individual failure details.

Required outcomes:
- Database pools have configurable finite bounds that fit the service resource budget. Batch worker concurrency respects that budget and does not scale unchecked with source size.
- Blacklist writes follow live tenant ownership and matching unique constraints. No global unique index or tenantless write may be introduced as a shortcut.
- Transient local persistence retries are bounded and never repeat the accepted provider request. Permanent errors remain visible and actionable.
- Every failed item emits safe diagnostics; no worker tail is lost. Provider correlation remains available when local persistence fails, without raw MSISDNs or credentials in ordinary logs.
- The existing subscription-only no-SMS, no-renewal, explicit-source, tenant-routing, invalid-number and checkpoint behavior remains intact.
- Recovery of historical provider successes uses only exact identity/correlation evidence, local idempotent persistence and no new opt-ins. Ambiguous masked identities must remain unresolved explicitly.
- Focused regression coverage uses real isolated PostgreSQL for SQL/index and pool behavior where relevant. No production load test, schema change, or unsolicited subscription batch.

Working checkout: subscription-pg-reliability-20260919, based on deployed source 4e8eb57. Preserve main's user-edited configuration and CSV. Parent coordinates deployment or live data repair only after source validation and independent review.
