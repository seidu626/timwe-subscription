# Tenant Channel Onboarding Fixtures

These fixtures support `docs/tenant-channel-onboarding.md` for contract version `tenant-channel-v1.0.0`.

Run:

```bash
examples/tenant-channel-onboarding/validate-fixtures.sh
```

The validator checks that examples include tenant/channel/partner identity, mutation idempotency keys, accepted callback signing headers, a missing-signature negative case, an unsupported-capability negative case, and a conversion postback example.
