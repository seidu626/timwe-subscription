# Dayline mobile app

Expo (React Native, TypeScript) subscriber app for Dayline, built against the
contract in `docs/dayline-app-api-contract.md`.

## Environment variables

Copy `.env.example` to `.env` and fill in per tenant/deployment:

| Variable | Required | Description |
| --- | --- | --- |
| `EXPO_PUBLIC_API_BASE_URL` | yes | Base URL of the subscription-external API, no trailing slash. |
| `EXPO_PUBLIC_TENANT_KEY` | yes | White-label tenant key sent on unauthenticated auth/catalog routes. Defaults to `careerify`. |
| `EXPO_PUBLIC_COUNTRY` | yes | ISO country code for catalog scoping. Defaults to `GH`. |
| `EXPO_PUBLIC_SUPPORT_URL` | no | Support link surfaced from Profile. Row renders disabled when unset. |
| `EXPO_PUBLIC_TERMS_URL` | no | Terms/privacy link surfaced from Profile. Row renders disabled when unset. |
| `EXPO_PUBLIC_PRIVACY_URL` | no | Fallback if `EXPO_PUBLIC_TERMS_URL` is unset. |

These are `EXPO_PUBLIC_*` so Expo inlines them into the client bundle at build
time — do not put secrets here.

## Run commands

```bash
npm install
npm run start        # Expo dev server (scan QR with Expo Go, or press i/a)
npm run ios          # iOS simulator
npm run android       # Android emulator
npm run web           # Web preview
npm run typecheck     # tsc --noEmit
npm run lint          # expo lint
```

## Architecture notes

- Routing: `expo-router`, file-based under `src/app`. Auth stack
  (`(auth)`) and the signed-in tab navigator (`(tabs)`) are gated with
  `Stack.Protected` in `src/app/_layout.tsx`.
- Server state: `@tanstack/react-query` hooks in `src/hooks/`, backed by a
  typed API client in `src/api/` that mirrors the contract's wire shapes
  exactly (no transform layer).
- Auth/session: JWT stored in `expo-secure-store` (`src/api/session.ts`),
  never `AsyncStorage`.
- Design tokens: ported from the Dayline design references into
  `src/theme/tokens.ts` (colors, type scale, radii, spacing, shadow, focus
  ring). No UI kit, no Tailwind/NativeWind — plain `StyleSheet`.
- Push notifications: `src/hooks/usePushRegistration.ts` requests permission
  and registers the device token with `POST /v1/app/devices` once signed in.

## Known contract-gap deviations

- There is no `GET` for current notification preferences, so the
  Notifications screen defaults every active subscription to `BOTH` locally
  until the subscriber changes it on-device.
- Delivery history on the Notifications screen reuses `GET /v1/app/feed`
  (the contract has no dedicated delivery-history endpoint).
- The contract's device field is named `fcm_token`; on iOS the registered
  token is the native APNs token from `getDevicePushTokenAsync`, not a real
  FCM token, since Expo does not proxy APNs through FCM without additional
  native config.
- Support/terms/privacy links are optional and render a disabled row when
  the corresponding env var is unset, rather than a fake/broken link.
