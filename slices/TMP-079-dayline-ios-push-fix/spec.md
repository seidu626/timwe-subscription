# TMP-079: Dayline iOS push fix

## Problem

Push notifications never work on iOS builds of dayline-mobile:

1. `app.json` has no `expo-notifications` config plugin and no `googleServicesFile`
   entries, so native FCM config is absent from prebuilds. No
   `GoogleService-Info.plist` or `google-services.json` exists in the repo (they
   are operator-supplied Firebase artifacts and must not be fabricated).
2. `usePushRegistration` swallows every failure in a bare `catch {}`, so a user
   whose device never registers gets no signal anywhere in the UI, and the
   `registerDevice` mutation is fire-and-forget (`mutate`), so backend rejection
   is invisible too.
3. On iOS `getDevicePushTokenAsync` returns a raw APNs token that is sent as
   `fcm_token`; the deviation is documented but there is no operator guide for
   completing the FCM setup.

## Goals

- Dynamic Expo config (`app.config.js`) that registers the `expo-notifications`
  plugin and wires `ios.googleServicesFile` / `android.googleServicesFile`
  conditionally, only when the operator has dropped the Firebase files into the
  project root, so a missing file never breaks `expo prebuild`.
- Rework `usePushRegistration` to record the registration outcome
  (registered / permission denied / failed with detail) in a small module-level
  store exposed via a `usePushRegistrationStatus` hook, and await the register
  mutation so backend failures are captured.
- Surface the denied/failed state as a banner on the Notifications screen so
  users understand why push delivery is off.
- Operator setup doc (`docs/dayline-push-setup.md`) covering the Firebase
  project, both platform config files, the APNs key upload, and the iOS
  APNs-vs-FCM token caveat.

## Non-goals

- Shipping Firebase credentials or fabricated plist/json config files.
- Physical-device push smoke testing (no iOS device/credentials in this
  environment); recorded as the remaining verification gap.
- Backend sender changes (APNs vs FCM token handling server-side).

## Verification

- `cd frontend/dayline-mobile && npx tsc --noEmit` clean.
- `node -e "require('./app.config.js')"` style sanity: config function returns
  plugins including expo-notifications and omits googleServicesFile when the
  files are absent.
