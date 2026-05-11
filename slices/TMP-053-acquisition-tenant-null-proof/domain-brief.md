# TMP-053 Domain Brief: Acquisition Tenant Nullable Proof

## Actors

- Platform operator: runs read-only checks before tenant NOT NULL enforcement.
- Canonical tenant: `nrg`, the required owner for pre-tenant acquisition/admin rows.
- Acquisition/admin table group: `campaigns`, `acquisition_transactions`, `postback_outbox`, `products`, `userbase`, `userbase_import_jobs`, `userbase_import_errors`, and `admin_activity_logs`.

## Ubiquitous Language

- Tenantless row: a row where `tenant_id IS NULL`.
- Read-only proof: SQL evidence that counts tenantless rows without changing data.
- Credential blocker: explicit evidence that the required database connection variables are unavailable.
- Enforcement readiness: the table group has zero tenantless rows or a blocker that prevents proof collection.

## Domain Invariants

- TMP-053 must not mutate a remote database.
- Tenantless acquisition/admin rows must not be assumed absent without row-count proof.
- Missing credentials are a valid proof blocker only when the missing env/tool is named.
- TMP-055 must not remove nullable runtime paths for this table group until proof or blocker evidence is recorded.

## Failure Modes

- Missing DB env: read-only SQL cannot run because `DB_PASSWORD`/`PGPASSWORD` or equivalent connection material is unavailable.
- Local default unavailable: documented defaults can point at a database that still requires a password.
- Remote documented host unavailable: a documented host can be reachable but reject passwordless access.
- Table drift: future proof SQL can fail if a table is missing from the deployed schema.

## User Journey

1. Platform operator reviews the acquisition/admin table group.
2. Operator checks documented DB connection inputs.
3. Operator runs read-only tenantless row-count SQL if credentials are available.
4. If credentials are unavailable, the slice records blocker evidence without reading secret files.
5. TMP-055 consumes the proof or blocker before enforcement work.

## Open Questions

- The current agent environment does not provide database credentials, so live row counts were not collected in TMP-053.
