# TMP-052 Follow-up Slices

## Backbone

1. Platform operator proves acquisition/admin tables have canonical tenant ownership.
2. Platform operator proves subscription/cadence tables have canonical tenant ownership.
3. Service owners collapse runtime nullable tenant matches into canonical tenant-aware paths.
4. Platform operator applies forward-only NOT NULL and legacy-index cleanup migrations after runtime proof.

## Emitted Slices

| Slice | Class | Depends on | Actor | Outcome |
|---|---|---|---|---|
| TMP-053 | bounded_enabler | TMP-052 | platform-operator | acquisition/admin tenant nullable rows are proven safe for canonical enforcement |
| TMP-054 | bounded_enabler | TMP-052 | platform-operator | subscription/cadence tenant nullable rows are proven safe for canonical enforcement |
| TMP-055 | vertical_defect_slice | TMP-053, TMP-054 | platform-operator | runtime nullable tenant matches and legacy partial indexes collapse into tenant-aware canonical paths |

## Value Gate

- TMP-053 and TMP-054 must be read-only proof slices. They may run SQL count checks and migration dry-runs but must not mutate a remote database.
- TMP-055 may add forward migrations and runtime code changes only after TMP-053 and TMP-054 produce zero-null proof or explicit blocker evidence.
- Existing migrations should not be rewritten for already-applied production history; cleanup must be forward-only.
