/**
 * HE Bootstrap Token utilities (client-side).
 * Single responsibility: capture, store, and retrieve he_token from URL/sessionStorage.
 * Used after acquisition-api redirect to /lp/:slug?he_token=...
 */

/** Default config; can be overridden for tests or multi-tenant */
export interface HeBootstrapConfig {
  /** URL query param name for the token */
  tokenParam: string
  /** sessionStorage key for the token */
  storageKey: string
}

const DEFAULT_CONFIG: HeBootstrapConfig = {
  tokenParam: 'he_token',
  storageKey: 'he_bootstrap_token',
}

/** Optional logger for debug (no-op in production if not provided) */
export type HeBootstrapLogger = (message: string) => void

let logger: HeBootstrapLogger = () => {}

/**
 * Set a logger for bootstrap events (e.g. debug). Call from app only if needed.
 */
export function setHeBootstrapLogger(fn: HeBootstrapLogger): void {
  logger = fn
}

function getConfig(): HeBootstrapConfig {
  return DEFAULT_CONFIG
}

function getStorage(): Storage | null {
  if (typeof window === 'undefined') return null
  return sessionStorage
}

/**
 * Capture he_token from URL, store in sessionStorage, and strip from URL.
 * Call once on landing page mount.
 * Returns the token if found, null otherwise.
 */
export function captureBootstrapTokenFromUrl(
  config: Partial<HeBootstrapConfig> = {}
): string | null {
  const storage = getStorage()
  if (!storage) return null

  const { tokenParam, storageKey } = { ...getConfig(), ...config }
  const url = new URL(window.location.href)
  const token = url.searchParams.get(tokenParam)

  if (!token) return null

  storage.setItem(storageKey, token)
  url.searchParams.delete(tokenParam)
  window.history.replaceState({}, '', url.toString())
  logger('HE bootstrap token captured and stored')
  return token
}

/**
 * Get stored bootstrap token, or null if none.
 */
export function getBootstrapToken(
  config: Partial<HeBootstrapConfig> = {}
): string | null {
  const storage = getStorage()
  if (!storage) return null
  const { storageKey } = { ...getConfig(), ...config }
  return storage.getItem(storageKey)
}

/**
 * Clear the stored bootstrap token (e.g. after successful use).
 */
export function clearBootstrapToken(
  config: Partial<HeBootstrapConfig> = {}
): void {
  const storage = getStorage()
  if (!storage) return
  const { storageKey } = { ...getConfig(), ...config }
  storage.removeItem(storageKey)
  logger('HE bootstrap token cleared')
}

/**
 * Whether a bootstrap token is currently stored.
 */
export function hasBootstrapToken(config: Partial<HeBootstrapConfig> = {}): boolean {
  return getBootstrapToken(config) !== null
}
