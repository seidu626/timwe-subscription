# TMP-017 Claude Review Notes

## Initial blocker review

Claude reviewed the TMP-017 diff and found three blockers:

- Ownership persistence happened after the provider charge and returned an API error on local DB failure, inviting caller retry and possible double charge.
- Generated idempotency keys were permanent for tenant/channel/product/subscriber/context, which could suppress legitimate future renewals.
- Legacy KrakenD billing routes were removed without compatibility replacements.

## Remediation

- `RequestCharge` now resolves idempotency before the provider call and `sendChargeRequest` forwards it as `external-tx-id`.
- Partner charge handler uses a supplied `idempotencyKey` or forwarded `external-tx-id` before falling back.
- Generated fallback idempotency includes a minute bucket so it does not permanently suppress future renewal attempts.
- If the provider charge succeeds but ownership persistence fails, the API logs the ownership error and returns provider success instead of causing a retryable double charge.
- Legacy KrakenD charge/MT routes remain available but retarget to `subscription-external` canonical partner endpoints.

## Re-review attempt

Two bounded Claude re-review attempts timed out without output after the remediation. Local regression gates and focused tests were run after applying the fixes.
