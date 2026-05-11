# Session Capsule: codex-postback-tenant-schema-20260511-011214

Task: `TMP-056`
Status: `done`

## Summary

Aligned acquisition-api service-local startup bootstrap with the canonical postback migration path so the in-process dispatcher does not poll tenant-aware postback_outbox columns before they exist.

## Completed Work

- Created TMP-056 defect slice after reviewing TMP-036/TMP-045 prior postback schema work.
- Added acquisition-api postback table and tenant-routing migrations to the service-local startup bootstrap list.
- Added repository tests for bootstrap order and tenant-routing migration content.
- Recorded domain grounding, slice story/spec, and value-gate evidence.

## Unfinished Work


## Next Tasks

