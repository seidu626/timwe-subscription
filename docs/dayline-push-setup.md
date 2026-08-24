# Dayline push notification setup (operator guide)

The Dayline mobile app registers each signed-in device's push token with the
backend (`POST /v1/app/devices`, field `fcm_token`). For push to actually work
in native builds, the operator must supply Firebase configuration files that
are NOT checked into this repo. `frontend/dayline-mobile/app.config.js` picks
them up automatically when they exist and skips them when they do not, so a
missing file never breaks `expo prebuild`.

## One-time Firebase setup

1. Create (or reuse) a Firebase project for Dayline.
2. Add an Android app with package name matching the Expo Android package,
   download `google-services.json`, and place it at
   `frontend/dayline-mobile/google-services.json`.
3. Add an iOS app with the bundle identifier matching the Expo iOS bundle id,
   download `GoogleService-Info.plist`, and place it at
   `frontend/dayline-mobile/GoogleService-Info.plist`.
4. iOS only: in Firebase console under Project settings > Cloud Messaging,
   upload an APNs authentication key (.p8) from the Apple Developer account
   (Keys > create a key with the Apple Push Notifications service enabled).
   Without this, FCM cannot deliver to iOS devices even with the plist in
   place.
5. Rebuild the native app (`npx expo prebuild --clean` then platform build, or
   a fresh EAS build). Config changes never apply to an already-installed
   binary.

Keep both files out of git; they are per-environment credentials.

## iOS token caveat

`expo-notifications`' `getDevicePushTokenAsync()` returns the FCM registration
token on Android. On iOS it returns the raw APNs device token; FCM can still
deliver to it only when the `GoogleService-Info.plist` config above is compiled
into the build so Firebase swizzles APNs registration. If the backend sender
sees iOS tokens that look like 64+ hex characters rather than FCM's
colon-separated format, the iOS build was made without the plist: redo steps
3-5.

## In-app failure surfacing

Registration is best-effort and never blocks sign-in, but the outcome is
recorded (`usePushRegistrationStatus`): the Notifications screen shows a banner
when the user denied the permission or when token fetch/backend registration
failed, including the failure detail. SMS delivery is unaffected either way.
