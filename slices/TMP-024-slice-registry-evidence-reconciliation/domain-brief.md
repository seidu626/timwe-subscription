# TMP-024 Domain Brief

- Actor: platform-operator
- Business outcome: Release-readiness reports and control-plane dashboards show truthful slice states and evidence paths for already accepted TMP-022 and TMP-023 work.
- Domain invariant: slice registry metadata must point to the evidence that actually proves the corresponding slice, and must not borrow another slice's DoD report.
- Entrypoint: `slices/manifest.json`
- Trigger: Operator inspects shipped full-system verification state.
- Risk: Incorrect manifest evidence can make a fixed build/test slice appear unverified, or make one slice look verified by another slice's report.

## Story Craft

The story is concrete and testable: TMP-022 and TMP-023 manifest rows can be queried and their state, automated checks, and DoD paths must match their accepted value-gate evidence.

## Roadmap To Slices

TMP-024 is a metadata reconciliation slice under TMP-021. It does not implement product behavior; it fixes the registry contract used by release-readiness tooling.
