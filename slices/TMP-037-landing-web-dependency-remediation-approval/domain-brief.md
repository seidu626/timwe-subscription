# TMP-037 Domain Brief

- Actor: platform-operator
- Business outcome: Landing-web dependency vulnerability remediation has an explicit approval gate before breaking Next/PostCSS upgrades are attempted.
- Domain invariant: full-system verification must not claim end-to-end readiness while this blocker remains unresolved.
- Entrypoint: services/landing-web/package.json and package-lock.json
- Trigger: Verifier runs npm audit after landing-web build passes.
- Risk: Dependency changes require explicit user approval by repo policy. The proposed remediation is breaking and requires UI regression verification.

## Story Craft

The story is concrete and testable: npm audit reports Next/PostCSS advisories and npm audit fix proposes a breaking Next upgrade to next@16.2.6. The expected outcome is: Dependency upgrade scope, risk, and UI regression proof are approved before package manifests or lockfiles change.

## Roadmap To Slices

This is a blocked follow-up slice under TMP-021. It records the smallest independently verifiable blocker without implementing approval-gated changes.
