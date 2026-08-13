/**
 * Runtime configuration. All values come from EXPO_PUBLIC_* env vars so they
 * are baked into the client bundle per Expo's public-env convention (see
 * README.md for the full list and how to set them per white-label tenant).
 */

export const API_BASE_URL = process.env.EXPO_PUBLIC_API_BASE_URL ?? '';

// Dayline ships as a white-label shell per tenant/product family. The tenant
// key and default country are build-time env vars rather than in-app
// settings because the contract's auth/catalog routes require them on every
// unauthenticated call (otp request/verify, catalog browse).
export const TENANT_KEY = process.env.EXPO_PUBLIC_TENANT_KEY ?? 'careerify';
export const COUNTRY_CODE = process.env.EXPO_PUBLIC_COUNTRY ?? 'GH';

// Optional support/legal links surfaced from Profile settings. Left unset by
// default; the settings rows render a disabled state rather than opening a
// broken/placeholder link when these are not configured for a tenant.
export const SUPPORT_URL = process.env.EXPO_PUBLIC_SUPPORT_URL ?? '';
export const TERMS_URL = process.env.EXPO_PUBLIC_TERMS_URL ?? '';
export const PRIVACY_URL = process.env.EXPO_PUBLIC_PRIVACY_URL ?? '';
