## Performance Ledger

- Symptom: `make dev` appears to pause at `Starting Admin Panel (Angular)...` before the Angular dev server starts.
- Metric: admin startup wall-clock time and dependency-setup latency.
- Baseline: in a clean isolated worktree with no `frontend/webspa-admin/node_modules`, `cd frontend/webspa-admin && npm install --silent` was still running after more than 5 minutes and was terminated. This proved the user-visible pause occurs before Angular serve starts.
- Target: repeated local starts with current dependencies should skip npm install and reach Angular serve readiness in under 20 seconds on the measured machine.
- Hypotheses:
  - Highest: unconditional npm install blocks every `dev-admin` invocation before Angular starts.
  - Secondary: readiness parsing depends on a `localhost:<port>` line that Angular may not emit before the Makefile checks.
- Measurement method: shell wall-clock timing with `date +%s%3N`, `make -n dev-admin`, and `make WEBSPA_ADMIN_PORT=4420 dev-admin` using existing installed admin dependencies.
- Bottleneck evidence: the clean dependency install exceeded 5 minutes; the patched warm dependency check completed in 1 ms and printed `Admin Panel dependencies current; skipping npm install.`
- Optimization: `dev-admin` now installs dependencies only when `node_modules` is missing, `node_modules/.package-lock.json` is missing, or `package-lock.json` is newer than the installed tree. Readiness now checks the actual listening port instead of requiring a `localhost:<port>` log line.
- After measurement: `make WEBSPA_ADMIN_PORT=4420 dev-admin` returned `rc=0 elapsed_ms=8185` and the Angular log included `Local: http://localhost:4420/`.
- Correctness verification: `make -n dev-admin` parsed successfully; HVC accepted TMP-059; admin dev-server smoke reached Angular local URL output on port 4420.
- Remaining risks: first-run startup still depends on npm registry/cache speed because dependencies must be installed when missing or stale.
