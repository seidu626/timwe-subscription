# TMP-006 Value Gate Report

Verdict: PASS

## Slice

Tenant landing submissions create acquisition transactions under the correct tenant campaign and isolate reuse/throttle/idempotency checks by tenant.

## Review Gates Applied

- Domain grounding applied: actors, invariants, failure modes, and TMP-007/TMP-013 deferrals are captured in `domain-brief.md`.
- Story craft applied: OTP confirmation acceptance was aligned to transaction tenant ownership; provider channel routing is deferred to TMP-007.
- Parallel critique applied: tenant id persistence, click-id isolation, pending reuse isolation, confirm campaign fallback, and migration order were addressed.
- Value gate applied after full acquisition-api verification.

## Acceptance Coverage

- Create tenant transaction: `CreateTransactionRequest` accepts `tenant_key`; service resolves `(tenant_key, campaign_slug)` and persists `tenant_id`.
- Confirm OTP: confirm recovers tenant campaign context for tenant-owned transactions before falling back to legacy slug lookup.
- Tenant campaign mismatch: tenant-key lookup fails without falling back to slug-only campaign lookup.
- Missing consent: existing consent validation remains before provider opt-in.
- Dependency failure: existing TIMWE error handling and status behavior are preserved.
- Pending reuse isolation: tenant campaigns call tenant-scoped pending reuse query.
- Click-id isolation: tenant campaigns call tenant-scoped click-id idempotency query.
- Throttle isolation: tenant campaigns call tenant-scoped throttle query.

## Verification

Commands run:

```bash
cd services/acquisition-api && go test ./...
cd services/landing-web && if [ -d node_modules ]; then npx tsc --noEmit; else echo 'node_modules missing; skipping tsc'; fi
```

Results:

- Full acquisition-api suite passed.
- landing-web TypeScript check could not run because `node_modules` is not installed in this worktree.

## Test Quality

The repository does not contain `scripts/scan-test-quality.sh`; manual checks were applied. New repository tests assert tenant id persistence on transaction create, and existing transaction service tests continue to prove legacy unscoped flow compatibility.

## Gaps Deferred

- Tenant channel credential/provider routing is deferred to TMP-007.
- Inbound charge/callback tenant correlation is deferred to TMP-013.
- Full transaction read DTO tenant exposure is deferred until admin/reporting surfaces require it.
