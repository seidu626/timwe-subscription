# Session Capsule: 20260510-2323

Task: `T-TMP-049`
Status: `done`

## Summary

Fixed acquisition-api startup schema bootstrap by removing all legacy foreign keys that reference public.campaigns(slug) before dropping campaigns_slug_key.

## Completed Work

- Replaced a hard-coded acquisition_transactions FK drop with catalog-driven removal of every FK that references public.campaigns(slug).
- Kept the migration explicit and bounded by avoiding CASCADE.
- Added a repository migration guard test covering FK discovery, DDL ordering, no CASCADE, and tenant acquisition indexes.
- Recorded TMP-049 issue, work order, slice manifest entry, domain brief, story, and value-gate evidence.

## Unfinished Work


## Next Tasks

