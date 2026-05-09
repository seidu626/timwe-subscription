# TMP-022 Notes

- prune: approve services/landing-web/app/lp/** allowed defect fix scope for Next.js route conflict.
- prune: approve services/landing-web/app/api/campaigns/** allowed defect fix scope for Next.js route conflict.
- prune: approve agent/backlog/issues/TMP-022-landing-web-dynamic-route-build.md required HVC issue for defect slice.
- prune: approve agent/state/TMP-022.work-order.json required HVC work order.
- prune: approve slices/TMP-022-landing-web-dynamic-route-build/spec.md required defect spec.
- prune: approve slices/TMP-022-landing-web-dynamic-route-build/domain-brief.md required domain grounding.
- prune: approve slices/TMP-022-landing-web-dynamic-route-build/value-gate-report.md required value-gate evidence.

## Implementation Plan

1. Rename single-segment route folders to share the same dynamic segment name as their tenant-qualified sibling.
2. Update legacy route code to treat the compatibility parameter as campaign slug.
3. Run `npm run build`.
