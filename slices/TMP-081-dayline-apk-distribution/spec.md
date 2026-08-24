# TMP-081: Dayline APK distribution for testing

## Problem

dayline-mobile has never been packaged for testers. The API base URL comes
from `EXPO_PUBLIC_API_BASE_URL` at bundle time and no production value is
recorded anywhere, so any ad-hoc build would point at a local instance or at
nothing. There is no repeatable way to rebundle and produce an APK, and no
place testers can download one.

## Goals

- `frontend/dayline-mobile/.env.production`: committed production bundle
  config pointing the app at the deployed gateway
  (`https://api.nouveauricheglobalgroup.com`, smoke-verified serving
  `/v1/app/*`).
- `frontend/dayline-mobile/scripts/build-apk.sh`: on-demand rebundle + APK
  build. Idempotent: installs node_modules when missing, runs
  `expo prebuild --platform android` when the native project is missing (or
  `--clean` is passed), then `gradlew assembleRelease`, and copies the result
  to `dist/dayline-<version>-<gitsha>.apk`. `--publish` uploads to the
  droplet and refreshes the stable `dayline.apk` download name.
- Hosting: bind-mount `~/services/nouveauricheglobalgroup/downloads` into the
  webspa-admin nginx container (`/usr/share/nginx/html/downloads`, read-only)
  so the APK is served at
  `https://admin.nouveauricheglobalgroup.com/downloads/dayline.apk` over the
  existing TLS vhost. Mirror the mount in `docker-compose.prod-do.yml`.
- Actually build and publish one APK so testers have a working link.

## Non-goals

- Play Store / EAS distribution, iOS builds, release signing (the Expo
  template signs release builds with the debug keystore, which is what
  sideload testing needs).
- Exposing MinIO publicly (port 9100 reachability is unverified from outside
  and the admin vhost already terminates TLS).

## Verification

- `cd frontend/dayline-mobile && npx tsc --noEmit` clean.
- Build script produces an APK; `apksigner verify` or install-time check.
- `curl -I https://admin.nouveauricheglobalgroup.com/downloads/dayline.apk`
  returns 200 with a non-trivial Content-Length.
