# TMP-085: Dayline app themes (light/dark/system) + UX polish

## Problem

The app hardcodes a single light Material palette (`src/theme/tokens.ts
colors`) imported statically by every screen and component. There is no dark
mode, no user preference, and product titles imported from carrier catalogs
render in ALL CAPS (shouting) on every surface.

## Approach

- `tokens.ts`: split the palette into `lightColors` (current values) and
  `darkColors` (Material 3 dark counterparts derived from the same roles;
  fixed roles stay theme-invariant). Remove the bare `colors` export so any
  unconverted import fails to compile. Add a `primarySoft` token to replace
  hardcoded `rgba(15,110,82,0.12)` overlays.
- New `src/theme/ThemeContext.tsx`: `ThemePreference = system|light|dark`
  persisted via the existing secureStorage wrapper (same pattern as
  SettingsContext); resolves against the OS scheme; exposes
  `{ preference, setPreference, scheme, colors }`.
- Root layout wraps the app in ThemeProvider and drives the status bar and
  navigation background from the resolved scheme.
- Every consumer converts to `const createStyles = (colors: ThemeColors) =>
  StyleSheet.create(...)` + `useTheme()` + `useMemo` per component.
- Profile gains an Appearance setting (System / Light / Dark).
- UX polish: `formatProductName` sentence-cases ALL-CAPS catalog names at
  render time (ProductRow, featured cards, product detail, confirm, success,
  subscriptions).

## Non-goals

- Per-tenant brand-color theming of whole screens (branding accents shipped
  in TMP-084).
- Theme-aware artwork or server-driven theming.

## Verification

- `npx tsc --noEmit` clean; `npx expo lint` no new issues.
- Manual: toggle Appearance in Profile; APK smoke after rebuild.
