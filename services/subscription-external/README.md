# Subscription External Service

`subscription-external` is the outbound TIMWE integration service.

## Responsibility

- Sends outbound requests to TIMWE APIs for subscription operations:
  - opt-in
  - opt-in confirm
  - opt-out
  - subscription status checks
  - partner MT/charge flows
- Applies resilience and integration controls for external calls (retry/circuit-breaker logic, response validation).

## What this service is not

- It does not own inbound TIMWE event webhook ingestion.
- TIMWE notification webhook ownership belongs to `subscription-partner`.

## Integration intent

- Platform services (including `acquisition-api`) should call `subscription-external` for outbound TIMWE subscription actions.

## Related docs

- Batch/processor operational docs remain in `services/subscription-external/docs/`.
- Service-role counterpart is documented in `services/subscription-partner/README.md`.
