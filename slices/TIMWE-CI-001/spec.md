# TIMWE-CI-001 — Add build/test CI pipeline (Angular, Next.js, Go lanes)

**Class**: operational_slice
**State**: planned
**Layers**: ci, tooling
**Depends on**: (none)

---

## ACCEPTANCE CRITERIA

- PR push triggers `.github/workflows/ci.yml`; all three lanes (Angular, Next.js, Go) run in parallel.
- **Angular lane**: `npm ci` → `ng lint` → `ng test --watch=false --browsers=ChromeHeadlessNoSandbox` → `ng build --configuration=production` passes.
- **Next.js lane**: `npm ci` → `next lint` → `tsc --noEmit` → `next build` passes.
- **Go lane**: matrix over `acquisition-api`, `billing`, `cadence-engine`, `notification`, `postback-dispatcher`, `subscription-external`, `subscription-partner`; each runs `go vet ./...` and `go test ./...`.
- At least one greenfield branch proves all three pass from a clean checkout.

---

## FILES TO TOUCH

| File | Action |
|------|--------|
| `.github/workflows/ci.yml` | new |
| `CODEOWNERS` | new — light ownership: `* @timwe-platform` |
| `services/landing-web/package.json` | add `packageManager` field |
| `frontend/webspa-admin/package.json` | add `packageManager` field |

---

## OUT OF SCOPE

- Modifying `codex-gate-internal.yml`, `codex-review-internal.yml`, `codex-router.yml`, `vendor-sync-check.yml`.
- Refactoring any Go service internals.
- Adding deploy steps or staging promotion.

---

## DEMO

1. Push branch with no code changes beyond the two new files (`.github/workflows/ci.yml`, `CODEOWNERS`).
2. Open PR; confirm Actions tab shows `ci.yml` running all three lanes.
3. All lanes green → slice acceptance met.

---

## RISK NOTES

> Record actuals in `notes.md` once the slice is claimed.

- **HIGH risk**: 55 slices marked done have no CI gate. Expect first-run failures on existing code.
- No root lockfile; each workspace has its own `package-lock.json` — run `npm ci` per-workspace using `working-directory`.
- Chrome must be installed on the runner for Karma headless (`ChromeHeadlessNoSandbox`); add `actions/setup-chrome` or equivalent.
- Go services have separate `go.mod` files — matrix over `services/` subdirectory names, not a monorepo go.mod.
