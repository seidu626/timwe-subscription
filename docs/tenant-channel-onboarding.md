# Tenant Channel Onboarding Contract Pack

Contract version: `tenant-channel-v1.0.0`

This pack gives API-integrated partners the stable tenant/channel contract for opt-in, confirmation, mobile-terminated messaging, charging, callbacks, and conversion postbacks. It is documentation-only and does not provision live credentials.

## Identity Model

Every partner request must carry both tenant and channel identity.

| Field | Required | Example | Notes |
| --- | --- | --- | --- |
| `tenant_key` | Yes | `legacy-default` | Stable tenant slug assigned during onboarding. |
| `channel_key` | Yes | `web-gh-mobplus` | Stable channel slug under the tenant. |
| `partner_key` | Yes | `mobplus` | Partner identifier used for audit and routing. |
| `capability` | Yes | `optin`, `confirm`, `mt`, `charge`, `callback`, `postback` | Must be enabled for the tenant channel before use. |
| `idempotency_key` | Mutations only | `tenant-channel-v1.0.0:optin:partner-click-001` | Stable for retries of the same logical action. |

Tenant/channel identity must come from the trusted onboarding contract or gateway/auth context. A request that can succeed without tenant/channel context is invalid.

## API Shape

The public contract is versioned by document version and by the `/v1` URL segment. Breaking changes require a new document version and fixture set.

| Operation | Method and Path | Required Capability | Idempotency |
| --- | --- | --- | --- |
| Opt-in | `POST /api/external/v1/{tenant_key}/{channel_key}/subscriptions/optin` | `optin` | Required |
| Confirm | `POST /api/external/v1/{tenant_key}/{channel_key}/subscriptions/confirm` | `confirm` | Required |
| Mobile terminated message | `POST /api/external/v1/{tenant_key}/{channel_key}/mt` | `mt` | Required |
| Charge | `POST /api/external/v1/{tenant_key}/{channel_key}/charges` | `charge` | Required |
| Callback receive | `POST /api/external/v1/{tenant_key}/{channel_key}/callbacks/{event_type}` | `callback` | Event id required |
| Conversion postback delivery | Partner-defined HTTPS URL from channel config | `postback` | `postback_id` required |

Recommended request headers:

```http
Authorization: Bearer <partner-access-token>
Content-Type: application/json
Idempotency-Key: tenant-channel-v1.0.0:optin:partner-click-001
X-Timwe-Contract-Version: tenant-channel-v1.0.0
```

## Credential Exchange

Credentials are exchanged out of band through the operator-approved secret channel for the tenant. Do not send credentials in tickets, examples, or screenshots.

The onboarding record must include:

- `tenant_key`
- `channel_key`
- `partner_key`
- enabled capabilities
- sandbox base URL
- callback shared secret reference
- postback target URL template
- credential rotation owner

Credential values in documentation must be placeholders such as `<partner-access-token>` or `<callback-shared-secret-ref>`.

## Callback Signing

Callbacks use HMAC-SHA256 with a timestamped canonical payload.

Required headers:

```http
X-Timwe-Timestamp: 2026-05-08T10:00:00Z
X-Timwe-Event-Id: evt_01JXAMPLE000000000000000
X-Timwe-Signature: sha256=<hex-hmac>
X-Timwe-Contract-Version: tenant-channel-v1.0.0
```

Signature input:

```text
<timestamp>.<event_id>.<raw_request_body>
```

Validation rules:

- reject missing signature, timestamp, or event id with `SIGNATURE_REQUIRED`
- reject timestamps outside a five-minute clock-skew window with `SIGNATURE_EXPIRED`
- reject HMAC mismatch with `SIGNATURE_INVALID`
- treat duplicate `event_id` as idempotent replay and return the original outcome

## Retry and Idempotency

Partners may retry network failures with the same `Idempotency-Key`.

| Case | Expected Result |
| --- | --- |
| Same key and same body | Return original result. |
| Same key and different body | Reject with `IDEMPOTENCY_CONFLICT`. |
| Missing key on mutation | Reject with `IDEMPOTENCY_KEY_REQUIRED`. |
| Duplicate callback event id | Return original callback outcome. |

## Errors

| Code | HTTP Status | Meaning |
| --- | --- | --- |
| `TENANT_CHANNEL_REQUIRED` | 400 | Missing tenant or channel identity. |
| `TENANT_CHANNEL_NOT_FOUND` | 404 | Tenant/channel pair is unknown or disabled. |
| `CAPABILITY_NOT_ENABLED` | 409 | Requested capability is not enabled for the channel. |
| `SIGNATURE_REQUIRED` | 401 | Callback signature headers are missing. |
| `SIGNATURE_INVALID` | 401 | Callback HMAC does not match the canonical payload. |
| `IDEMPOTENCY_KEY_REQUIRED` | 400 | Mutation request omitted the idempotency key. |
| `IDEMPOTENCY_CONFLICT` | 409 | Idempotency key was reused with a different body. |

## Postback Contract

Postbacks are delivered from the platform to the partner URL configured on the tenant channel. The payload must include tenant/channel identity and a stable postback id.

```json
{
  "contract_version": "tenant-channel-v1.0.0",
  "tenant_key": "legacy-default",
  "channel_key": "web-gh-mobplus",
  "partner_key": "mobplus",
  "postback_id": "pb_01JXAMPLE000000000000000",
  "event_type": "conversion.charged",
  "click_id": "partner-click-001",
  "msisdn_hash": "sha256:example-redacted",
  "amount": "1.00",
  "currency": "GHS",
  "occurred_at": "2026-05-08T10:00:00Z"
}
```

Do not include raw MSISDN values in postback examples.

## Legacy TIMWE Mapping

| Legacy Field | Tenant/Channel Contract Field | Posture |
| --- | --- | --- |
| `realm` | `tenant_key` | Supported as compatibility input only when mapped explicitly. |
| `channel` or `entry_channel` | `channel_key` | Supported for current TIMWE-style integrations. |
| `partnerRole` | `partner_key` | Mapped during onboarding. |
| `txid`, `tracker`, or `click_id` | `click_id` | Preserve original partner value. |
| unsigned callback | signed callback headers | Deprecated; sandbox must reject missing signatures. |

Ambiguous legacy mappings must fail closed. Operators must choose a single tenant/channel mapping before enabling production traffic.

## Sandbox Fixtures

Fixtures live in `examples/tenant-channel-onboarding/contract-fixtures.json`.

Run the local fixture smoke:

```bash
examples/tenant-channel-onboarding/validate-fixtures.sh
```

The fixture bundle includes supported opt-in, charge, callback, and postback examples plus negative examples for missing callback signatures and unsupported charge capability.
