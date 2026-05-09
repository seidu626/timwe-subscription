# TMP-031 Notes

- Pre-fix `notification-worker` only overrode the DB host in compose.
- The worker pings the DB at startup, unlike notification API, so incomplete DB env caused the worker to exit during compose smoke.
- The fix passes the same local Postgres user, password, database, port, and `sslmode=disable` values used by other compose services.
- Targeted smoke starts the worker and metrics endpoint.
- After startup, the dispatcher logs `message_outbox` missing because the empty compose database has not applied subscription-external message cadence migrations.
