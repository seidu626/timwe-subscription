# TMP-004 Domain Brief: Channel Credential Binding

## Actors

- tenant-admin: binds and inspects redacted credential references for a tenant-owned channel.
- channel routing services: later resolve an active credential reference for provider calls.
- secret backend adapter: accepts raw credential material and returns a durable reference when configured.

## Ubiquitous Language

- Tenant: platform-owned isolation boundary resolved from trusted admin identity.
- Channel: tenant-owned provider/country/operator capability catalog entry.
- Credential reference: opaque pointer to secret material stored outside the application database.
- Redacted display: non-sensitive label returned to admins instead of the raw reference or secret.
- Purpose: credential scope under a channel, defaulting to `provider_api`.
- Version: server-assigned monotonically increasing credential version per tenant/channel/purpose.

## Domain Invariants

- Credential rows store secret references, redacted display, and tenant-scoped fingerprints only; never raw secret values.
- One active credential may exist per tenant/channel/purpose.
- Cross-tenant channel and credential access resolves to not found.
- Inactive channels reject credential binding before a secret backend call.
- Bind and rotate operations are auditable with redacted metadata only.
- Raw credential material requires a configured secret adapter; without one, no active credential row is created.

## Failure Modes

- Bind credential:
  - Invalid input: missing both `secret_ref` and `secret_value`, both supplied, invalid purpose, or disallowed reference prefix returns 400.
  - Missing tenant context: existing tenant resolver returns 403.
  - Cross-tenant channel: tenant-scoped channel lookup returns 404.
  - Inactive channel: service returns conflict with `channel_inactive`.
  - Secret backend unavailable: raw secret bind returns 503 before any credential row is created.
  - Duplicate/double-click bind: active credential with same tenant-scoped fingerprint returns existing version rather than inflating versions.
- List credentials:
  - Cross-tenant channel id returns an empty tenant-scoped list or not found via tenant filters.
  - Response never includes `secret_ref`, `secret_value`, token, password, API key, or fingerprint fields.

## User Journey

1. Tenant admin calls `POST /v1/admin/channels/{id}/credentials` with a trusted tenant context and either `secret_ref` or `secret_value`.
2. System verifies the channel belongs to the tenant and is active.
3. If a raw secret was provided, the configured adapter returns an opaque reference; if no adapter exists, the system returns 503.
4. System transactionally inactivates the old active credential for the same purpose, inserts the new active version, and writes an audit log.
5. Tenant admin calls `GET /v1/admin/channels/{id}/credentials` and receives redacted credential metadata only.

Failure journeys:
1. Tenant admin binds to an inactive channel -> 409 `channel_inactive`, no secret backend call.
2. Tenant admin sends raw secret without backend -> 503, response does not echo input.
3. Tenant admin attempts a foreign channel id -> 404 through tenant-scoped lookup.

## Open Questions

- TMP-004 ships the adapter interface but not a production vault implementation. TMP-015 owns platform secret operations and hardening.
- Orphan cleanup after a successful external secret write followed by database failure is not implemented because no delete-capable backend exists yet.
