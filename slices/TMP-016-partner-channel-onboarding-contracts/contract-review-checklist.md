# TMP-016 Contract Review Checklist

- [x] Contract version is explicit: `tenant-channel-v1.0.0`.
- [x] Partner-facing onboarding doc names endpoints, auth, tenant/channel identity, retries, idempotency, errors, callbacks, postbacks, and legacy mapping.
- [x] Credential exchange runbook avoids literal secrets and uses secret references.
- [x] Callback signing algorithm, headers, canonical payload, timestamp window, and error codes are documented.
- [x] Fixture bundle includes supported opt-in, charge, callback, and postback examples.
- [x] Fixture bundle includes missing-signature and unsupported-capability negative examples.
- [x] Local fixture validator is runnable without production credentials.
- [x] No runtime service code, migrations, frontend files, or Makefile changes were made for this slice.
