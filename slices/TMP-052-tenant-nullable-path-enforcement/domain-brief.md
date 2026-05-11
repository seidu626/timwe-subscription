# TMP-052 Domain Brief: Tenant Nullable Path Enforcement

## Actors

- Platform operator: audits tenant nullable paths before enforcing canonical `nrg` tenant ownership.
- End subscriber: enters acquisition and subscription flows that must resolve a tenant-owned campaign, subscription, or cadence state.
- Cadence worker: selects due subscription messages and missing message states without crossing tenant ownership.
- Canonical tenant: `nrg`, the default owner for pre-tenant rows after TMP-050.

## Ubiquitous Language

- Tenant nullable path: code, migration, script, or documentation that permits `tenant_id IS NULL` as an ownership state.
- Canonical ownership: every existing tenantless row is assigned to tenant `nrg`.
- Runtime nullable match: an active query predicate that treats NULL tenant ownership as a valid match.
- Enforcement migration: a schema change that converts tenant-owned tables from nullable tenant columns to `NOT NULL`.
- Prune classification: `delete_now`, `collapse_into_canonical`, `keep_as_permanent_capability`, or `needs_human_decision`.

## Domain Invariants

- Existing tenantless production data must migrate to `nrg` before NOT NULL constraints are applied.
- Active runtime paths must not use NULL tenant ownership as a long-lived compatibility model.
- Read-only dry-run and verification commands may inspect `tenant_id IS NULL`; that is migration observability, not production ownership.
- NOT NULL enforcement requires runtime proof for every touched table group.
- Tenant matching must preserve tenant isolation; NULL tenant joins must not allow cross-tenant reads.

## Failure Modes

- Acquisition campaign lookup: slug-only public lookup can keep resolving only `tenant_id IS NULL` campaigns after canonical backfill, making `nrg`-owned campaigns invisible.
- Acquisition reporting: nullable campaign join predicates can match transactions against tenantless campaigns after canonical ownership is expected.
- Cadence due selection: nullable tenant predicates can match message state, subscription, and series rows across incomplete ownership.
- Subscription/cadence schema enforcement: applying NOT NULL without live row counts can fail migrations or hide unmigrated data.
- Migration tooling: deleting the `tenant_id IS NULL` dry-run predicate before enforcement would remove the operator's only local proof that backfill completed.

## User Journey

1. Platform operator runs the required nullable-path audit.
2. The audit classifies active runtime nullable paths separately from migration/docs proof paths.
3. The operator receives follow-up slices for acquisition, subscription/cadence, and final NOT NULL enforcement.
4. Later implementation slices remove runtime nullable matches only after table groups have proof that canonical `nrg` ownership is complete.

## Open Questions

- Live production row counts are not available in this slice; no remote database mutation or NOT NULL enforcement was attempted.
- Some non-tenant compatibility paths remain outside the grep evidence scope and should be pruned by separate service-specific slices.
