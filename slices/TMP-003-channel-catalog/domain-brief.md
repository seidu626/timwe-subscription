# TMP-003 Domain Brief: Tenant Channel Catalog

## Domain Grounding

A channel is a tenant-owned routing option for future acquisition, subscription, cadence, and callback work. It describes provider, country/operator scope, and supported capabilities. It deliberately does not store credentials or provider secret/config material; TMP-004 owns credential binding.

## Actors

- tenant-admin: creates, lists, and disables/enables channel definitions for the current tenant.
- campaign setup: later selects a compatible active channel.
- subscription and notification routing: later resolve capability support before calling providers.

## Story Craft

As a tenant-admin, I want to define channels my tenant can use so that future acquisition and subscription operations route through supported providers.

Primary journey:
1. Trusted admin auth resolves the current tenant.
2. Admin submits provider, country, optional operator, capabilities, and optional enabled state.
3. Service normalizes provider/country/operator, derives the channel key, validates capabilities, and writes a tenant-owned channel.
4. Admin lists only channels for the current tenant.
5. Admin toggles enabled state with a tenant-scoped single update.

## Invariants

- Channel queries and mutations require tenant_id.
- Cross-tenant channel access returns not found.
- `channel_key` is deterministic and unique per tenant.
- Capabilities are a closed set: `optin`, `confirm`, `mt`, `charge`.
- `charge` requires `mt` because charge outcomes depend on message/channel routing.
- Capabilities are deduplicated and returned in deterministic order.
- No credential, secret, token, or metadata JSON is stored in TMP-003.

## Failure Modes Covered

- Unsupported capabilities return `ErrInvalidInput` with `invalid_capability`.
- Duplicate channel key/provider scope maps to admin conflict.
- Missing tenant context follows the existing 403 tenant-context path.
- Cross-tenant enabled toggle maps tenant-scoped no-row to 404.
- Unknown fields in the PATCH enabled contract are rejected.

## Roadmap Link

TMP-003 unlocks TMP-004 credential binding and is a dependency for campaign binding, notification/cadence routing, inbound callback correlation, and subscription routing.
