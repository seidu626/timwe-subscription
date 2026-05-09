# TMP-021 Domain Brief

- Actor: platform-operator
- Business outcome: Operator has an evidence-backed release-readiness matrix for every discovered runnable component and implemented tenant-platform feature.
- Domain invariant: full-system verification must distinguish passed, fixed, blocked, failed, not applicable, and not implemented states; build success alone must not imply feature readiness.
- Entrypoint: `docs/agent/full-system-verification-2026-05-09.md`
- Trigger: Operator requests end-to-end release verification.
- Risk: Release-readiness can be overstated if blocked runtime checks, missing submodules, dependency approval gates, or local/remote branch divergence are hidden.

## Story Craft

The story is concrete and testable: the platform operator opens the release matrix and sees service inventory, feature inventory, command evidence, and blocker rows. The expected outcome is a truthful readiness report, not a product implementation.

## Roadmap To Slices

TMP-021 is the parent operational verification slice. Concrete defects and approval gates are split into narrower follow-up slices such as TMP-026 and TMP-034 through TMP-038.
