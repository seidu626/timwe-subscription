# Tenant Platform Decision Records

These ADR placeholders capture decisions that block or materially shape implementation slices. Each ADR should be filled before its dependent slice claims value-gate PASS.

| ADR | Decision | Blocks or shapes |
| --- | --- | --- |
| ADR-001 | Tenant claim model: Auth0 Organizations, custom tenant claim, or hybrid | TMP-018, TMP-001, TMP-014 |
| ADR-002 | Public tenant routing: host, path, signed token, gateway host map, or hybrid | TMP-012, TMP-005, TMP-006 |
| ADR-003 | Secret backend: external vault/secret manager, encrypted DB reference, staged adapter | TMP-004, TMP-015 |
| ADR-004 | Service-to-service auth: gateway-signed header, HMAC, mTLS, service account JWT | TMP-018, TMP-007, TMP-013 |
| ADR-005 | Tenant isolation model: shared DB/repository enforcement, PostgreSQL RLS, schema-per-tenant, database-per-tenant | TMP-001, TMP-002, TMP-011 |
| ADR-006 | Charge ownership: subscription-external, billing service, or explicit split | TMP-017, TMP-007, TMP-010 |
| ADR-007 | Admin portal delivery posture: API-only first, thin UI per slice, or full workspace rollout | TMP-014 and tenant-admin usability |

Minimum ADR fields:

- Status: proposed, accepted, superseded.
- Context: current repo facts and constraints.
- Decision: one selected path.
- Consequences: migration, security, operations, tests, and compatibility implications.
- Slice impact: exact slice IDs and acceptance criteria affected.
