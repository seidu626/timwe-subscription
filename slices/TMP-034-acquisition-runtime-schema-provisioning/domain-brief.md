# TMP-034 Domain Brief

- Actor: platform-operator
- Business outcome: Acquisition API starts in the compose runtime with base products/userbase schema available before admin migrations run.
- Domain invariant: full-system verification must not claim end-to-end readiness while this blocker remains unresolved.
- Entrypoint: docker compose acquisition-api runtime startup
- Trigger: Verifier runs bounded compose runtime smoke after TMP-030 image build fix.
- Risk: Schema/migration provisioning is approval-gated by repo risk boundaries. The failing relation products/userbase path requires schema ownership and migration-order decision before implementation.

## Story Craft

The story is concrete and testable: Acquisition API exits during admin schema bootstrap because add_admin_management_tables.sql expects relation products in the empty compose DB. The expected outcome is: The compose DB schema provisioning path creates or migrates products and userbase before add_admin_management_tables.sql runs, so acquisition-api reaches health checks.

## Roadmap To Slices

This is a blocked follow-up slice under TMP-021. It records the smallest independently verifiable blocker without implementing approval-gated changes.
