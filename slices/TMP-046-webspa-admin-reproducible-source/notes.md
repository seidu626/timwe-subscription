# TMP-046 Notes

- The pinned submodule commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a` exists in the local nested checkout and contains tenant workspace guardrails, but the public CoreUI remote does not advertise it.
- Repointing to CoreUI `main` would make `git submodule update` reproducible while dropping tenant admin source. This slice preserves the shipped tenant admin behavior by tracking the source directly.
- `npm ci` completed with Node engine and vulnerability warnings. Dependency remediation remains TMP-037/TMP-048 scope, not this source reproducibility slice.
