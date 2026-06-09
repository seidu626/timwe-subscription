# Just Command Reference

This repository uses `just` for project command automation.

## Discover Commands

```bash
just --list
```

## Common Commands

```bash
just dev
just stop
just restart
just status
just build
just test
just health
just logs
just compose-up
just compose-down
```

## Database Operations

```bash
just db-migrate-tenant-platform-dry-run
just db-migrate-tenant-platform
just db-migrate-nrg-subscriptions-transactions-dry-run
just db-migrate-nrg-subscriptions-transactions
just db-exec-sql services/acquisition-api/migrations/update_ghana_lp_copy_msisdn_format.sql
```

## Docker And Deploy

```bash
just docker-build-all
just docker-push-all
just docker-release-all
just deploy-all
```

Recipes are grouped in the justfile with `[group(...)]` attributes, so `just --list` is the canonical command catalog.

Service start recipes use fixed ports because the web clients and gateway configs reference those ports directly. If a configured port is already in use, the recipe fails with the listener details instead of starting that service on a different port.
