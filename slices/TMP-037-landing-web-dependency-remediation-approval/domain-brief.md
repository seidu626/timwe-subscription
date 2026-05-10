# TMP-037 Domain Brief

- Actor: platform-operator
- Business outcome: Landing-web dependency vulnerability remediation has an explicit approval gate before breaking Next/PostCSS upgrades are attempted.
- Domain invariant: full-system verification must not claim end-to-end readiness while this blocker remains unresolved.
- Entrypoint: services/landing-web/package.json and package-lock.json
- Trigger: Verifier runs npm audit after landing-web build passes.
- Risk: Dependency changes required explicit user approval by repo policy. Approval was recorded on 2026-05-10 from the operator auto-proceed directive. The proposed remediation is breaking and requires UI regression verification.

## Story Craft

The story is concrete and testable: npm audit reports Next/PostCSS advisories and npm audit fix proposes a breaking Next upgrade to next@16.2.6. The expected outcome is: Dependency upgrade scope, risk, and UI regression proof are approved before package manifests or lockfiles change.

## Roadmap To Slices

This is a follow-up slice under TMP-021. It records the smallest independently verifiable approval gate without implementing the package changes directly; the remediation itself belongs in a bounded implementation slice.
