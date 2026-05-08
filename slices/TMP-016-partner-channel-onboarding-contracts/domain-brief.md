# TMP-016 Domain Brief: Partner Channel Onboarding Contracts

Post-hoc reconciliation note: this domain brief was added after implementation to align the shipped slice artifacts with the domain-grounding contract. It summarizes existing `slice.yaml`, issue, docs, fixtures, checklist, and value-gate evidence; it does not introduce new runtime scope.

## Actors

- API-integrated partner: follows tenant/channel API, callback, credential, sandbox, and postback contracts to integrate a channel. Source: `slices/TMP-016-partner-channel-onboarding-contracts/slice.yaml`.
- Platform operator: reviews and accepts partner contract evidence before onboarding a tenant channel. Source: `slices/TMP-016-partner-channel-onboarding-contracts/contract-review-checklist.md`.
- Tenant channel implementer: uses the contract pack to preserve tenant/channel identity across runtime slices. Source: `docs/tenant-channel-onboarding.md`.
- Callback sender: emits signed callback events that must include tenant/channel identity and replay/idempotency fields. Source: `examples/tenant-channel-onboarding/contract-fixtures.json`.

## Ubiquitous Language

- Contract version: explicit `tenant-channel-v1.0.0` version shared by docs and fixtures. Source: `docs/tenant-channel-onboarding.md`.
- Tenant/channel identity: required identifiers carried through API requests, callbacks, credentials, sandbox payloads, and postbacks. Source: `examples/tenant-channel-onboarding/contract-fixtures.json`.
- Capability: channel-supported operation such as opt-in, confirm, MT, charge, callback, or postback. Source: `docs/tenant-channel-onboarding.md`.
- Unsupported capability: documented failure when a partner requests an operation the channel does not support. Source: `examples/tenant-channel-onboarding/contract-fixtures.json`.
- Callback signature: HMAC-SHA256 signing contract with timestamp/event identity to prevent unsigned or replayed callbacks. Source: `docs/tenant-channel-onboarding.md`.
- Credential redaction: contract rule that partners exchange credential references and never commit raw secret material. Source: `docs/tenant-channel-onboarding.md`.

## Domain Invariants

- Partner-facing examples must include tenant/channel identity; no successful example may omit tenant/channel context.
- Contract versioning must be explicit and consistent across docs, fixtures, and review checklist.
- Callback examples must include timestamp, event id, and signature guidance.
- Unsupported channel capability requests must fail with documented errors rather than silently routing to a global/default provider.
- Legacy TIMWE mapping ambiguity must be documented with safe fallback behavior.

## Failure Modes

- Breaking contract change: endpoint or payload field changes without versioning -> review checklist flags the change.
- Missing callback signature: fixture omits signature -> sandbox validator rejects it.
- Unsupported capability requested: non-charge-capable channel receives charge request -> documented capability error.
- Missing tenant/channel identity: fixture or contract omits required identity -> validator/review fails.
- Raw credential in example: redaction guidance or review fails because secret material appears in contract artifacts.

## User Journey

1. API-integrated partner receives `docs/tenant-channel-onboarding.md`.
2. Partner reads version, identity, endpoint, auth, credential, callback, retry, idempotency, error, and postback contracts.
3. Partner runs `examples/tenant-channel-onboarding/validate-fixtures.sh` against sandbox fixtures.
4. Platform operator reviews `contract-review-checklist.md` and confirms supported/unsupported examples and legacy ambiguity are covered.
5. Runtime implementers preserve this contract in later channel, routing, callback, and postback slices.

Failure journeys:

1. Partner sends unsigned callback -> contract fixture documents rejection.
2. Partner requests unsupported charge capability -> fixture documents capability error.
3. Contract version is omitted or mismatched -> review checklist blocks release.

## Open Questions

- This slice is a bounded contract enabler; live partner credential provisioning and backend route rewrites are intentionally out of scope.
- Legal/commercial partner agreements are not represented here; this is the technical integration contract only.
