# TMP-032 Notes

- Pre-fix compose passed `DATABASE_POSTGRESQL_*` values to postback-dispatcher.
- `services/postback-dispatcher` uses `common/config`, which binds `DB_POSTGRESQL_*`, `APP_DATABASE_POSTGRESQL_*`, and related aliases, but not `DATABASE_POSTGRESQL_*`.
- The compose-only fix keeps the documented `DATABASE_POSTGRESQL_*` values and adds `DB_POSTGRESQL_*` aliases plus `sslmode=disable`.
- Targeted smoke starts the dispatcher, logs database connection, and starts the worker loop.
- After startup, dispatcher logs `postback_outbox` missing because the empty compose database has not applied postback migrations.
