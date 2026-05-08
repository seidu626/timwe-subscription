# TMP-012 Value Gate Report

Verdict: PASS

## Slice

Public landing and campaign lookup routes preserve tenant context and prevent tenant-bound campaigns from resolving through ambiguous legacy slug routes.

## Review Gates Applied

- Domain grounding applied: public actors, trusted headers, legacy routes, HE bootstrap, and deferred transaction/callback scope are captured in `domain-brief.md`.
- Story craft applied: acceptance was narrowed to explicit path and signed header routing; transaction/callback persistence is deferred to TMP-006/TMP-013.
- Parallel critique applied: legacy slug ambiguity, landing-web tenant path support, forged tenant headers, and HE tenant redirect were implemented.
- Value gate applied after backend verification and landing TypeScript availability check.

## Acceptance Coverage

- Tenant route resolves campaign: acquisition-api `/v1/campaigns/{tenant_key}/{slug}` remains routed to tenant lookup; landing-web adds `/lp/{tenant}/{slug}` and `/api/campaigns/{tenant}/{slug}`.
- Gateway forwards tenant context: public slug lookup accepts tenant headers only through `tenantctx.IdentityFromTrustedRequest`; unsigned tenant headers are rejected.
- Ambiguous campaign slug: legacy `GetBySlug` and `ListEnabled` filter to `tenant_id IS NULL`.
- Forged tenant header: handler tests reject unsigned tenant headers and accept signed tenant key context.
- Disabled tenant: TMP-005 tenant-key repository lookup joins `tenants.status = 'ACTIVE'`.
- Legacy single-tenant route: `/lp/{slug}` and `/api/campaigns/{slug}` remain for unscoped campaigns.
- HE tenant campaign route: `/v1/he/bootstrap/campaign/{tenant}/{slug}` preserves route context for redirect and token storage.

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

The repository does not contain `scripts/scan-test-quality.sh`; manual checks were applied. New tests assert signed/unsigned tenant header behavior, tenant campaign path parsing, HE tenant route validation, and preservation of tenant-aware router dispatch.

## Gaps Deferred

- Host-based tenant mapping and KrakenD config fixture verification are deferred until environment routing details are available.
- Transaction tenant persistence and callback correlation are deferred to TMP-006 and TMP-013.
