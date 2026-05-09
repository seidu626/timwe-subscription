# TMP-040 Notes

- The primary checkout contains a nested `frontend/webspa-admin` repository at `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- That commit is `feat: add tenant workspace admin guardrails`.
- Local admin verification passed:
  - `npm run build`
  - `CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false`
- Environment observed:
  - Node `v24.15.0`; Angular reports this Node version as unsupported.
  - npm `11.12.1`.
  - Google Chrome `148.0.7778.96`.
- Clean source-truth submodule initialization still fails with `fatal: remote error: upload-pack: not our ref 2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- TMP-026 therefore remains blocked for reproducible clean clones even though local admin code-health evidence now exists.
