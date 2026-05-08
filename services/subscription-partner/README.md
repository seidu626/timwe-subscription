# Subscription Partner Service

`subscription-partner` is the inbound partner-facing service.

## Responsibility

- Owns inbound TIMWE event notifications sent to this platform.
- Persists incoming notification events for downstream processing and audit.
- Triggers internal callbacks for charge/renewal events when required.

## What this service is not

- It does not send outbound TIMWE opt-in/opt-out/confirm/status requests.
- Outbound subscription actions are owned by `subscription-external`.

## Main API surface (current ownership)

- `POST /api/v1/webhooks/timwe/notification` - receive TIMWE event notifications.
- Existing legacy `/api/v1/subscription/*` routes may still exist for compatibility but are not the canonical outbound TIMWE integration path.

## Related service

- See `services/subscription-external/README.md` for outbound TIMWE request flow ownership.
