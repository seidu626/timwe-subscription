/**
 * Header Enrichment (HE) Simulation Utilities
 *
 * SECURITY: This module is for staging/local testing ONLY.
 * Production environments MUST have HE_SIMULATION_ENABLED=false.
 *
 * See docs/timwe-he-simulation-e2e.md for full specification.
 */

import * as jose from 'jose'

// HE identity source types
export type HESource = 'REAL' | 'SIMULATED' | 'NONE'

// HE identity payload (stored in JWT and passed downstream)
export interface HEIdentity {
  msisdn: string
  operatorId?: string
  mcc?: string
  mnc?: string
  country?: string
  source: HESource
}

// JWT payload structure for simulation token
export interface HESimPayload {
  msisdn: string
  operatorId?: string
  mcc?: string
  mnc?: string
  country?: string
  iat: number
  exp: number
}

// Configuration for HE simulation
export interface HESimConfig {
  enabled: boolean
  secret: string
  cookieName: string
  ttlSeconds: number
  cookieSecure: boolean
  allowedMsisdns: string[]
}

export interface HESimulationInput {
  msisdn?: string | null
  operator?: string | null
  mcc?: string | null
  mnc?: string | null
  redirect?: string | null
}

export interface HESimulationScenario {
  identity: Omit<HEIdentity, 'source'>
  redirect: string
  maskedMsisdn: string
}

// Ghana operator definitions (from docs/ghana-header-enrichment-parameters.md)
export const GHANA_OPERATORS = {
  MTN: { mcc: '620', mnc: '01', name: 'MTN Ghana' },
  TELECEL: { mcc: '620', mnc: '02', name: 'Telecel Ghana (ex-Vodafone)' },
  AT_03: { mcc: '620', mnc: '03', name: 'AT Ghana (ex-AirtelTigo)' },
  AT_06: { mcc: '620', mnc: '06', name: 'AT Ghana (ex-AirtelTigo)' },
} as const

type GhanaOperatorKey = keyof typeof GHANA_OPERATORS

// Candidate MSISDN headers to check (in order of preference)
export const MSISDN_HEADERS = [
  'x-msisdn',
  'x-up-calling-line-id',
  'x_wap_network_client_msisdn',
] as const

/**
 * Load HE simulation configuration from environment variables
 */
export function getHESimConfig(): HESimConfig {
  const cookieSecure = process.env.HE_SIM_COOKIE_SECURE
  const ttlSeconds = Number.parseInt(process.env.HE_SIM_TTL_SECONDS || '180', 10)

  return {
    enabled: process.env.HE_SIMULATION_ENABLED === 'true',
    secret: process.env.HE_SIM_SECRET || '',
    cookieName: process.env.HE_SIM_COOKIE_NAME || 'he_sim_token',
    ttlSeconds: Number.isFinite(ttlSeconds) && ttlSeconds > 0 ? ttlSeconds : 180,
    cookieSecure: cookieSecure ? cookieSecure === 'true' : process.env.NODE_ENV === 'production',
    allowedMsisdns: parseCsv(process.env.HE_SIM_ALLOWED_MSISDNS).map(normalizeMsisdn),
  }
}

export function getDefaultHESimulationInput(): HESimulationInput {
  return {
    msisdn: process.env.HE_SIM_DEFAULT_MSISDN || '',
    operator: process.env.HE_SIM_DEFAULT_OPERATOR || 'MTN',
    mcc: process.env.HE_SIM_DEFAULT_MCC || '',
    mnc: process.env.HE_SIM_DEFAULT_MNC || '',
    redirect: process.env.HE_SIM_DEFAULT_REDIRECT || '/',
  }
}

/**
 * Normalize MSISDN by removing whitespace and leading '+'
 */
export function normalizeMsisdn(msisdn: string): string {
  return msisdn.replace(/\s+/g, '').replace(/^\+/, '')
}

/**
 * Validate MSISDN format (basic check for numeric string)
 */
export function isValidMsisdn(msisdn: string): boolean {
  const normalized = normalizeMsisdn(msisdn)
  return /^\d{9,15}$/.test(normalized)
}

export function maskMsisdnForDisplay(msisdn: string): string {
  const normalized = normalizeMsisdn(msisdn)
  if (normalized.length <= 6) {
    return '*'.repeat(normalized.length)
  }

  return `${normalized.slice(0, 5)}${'*'.repeat(normalized.length - 8)}${normalized.slice(-3)}`
}

export function resolveHESimulationScenario(
  input: HESimulationInput,
  config: HESimConfig = getHESimConfig()
): HESimulationScenario {
  const defaults = getDefaultHESimulationInput()
  const msisdn = normalizeMsisdn(cleanString(input.msisdn) || cleanString(defaults.msisdn))

  if (!msisdn || !isValidMsisdn(msisdn)) {
    throw new Error('Invalid MSISDN format')
  }

  if (config.allowedMsisdns.length > 0 && !config.allowedMsisdns.includes(msisdn)) {
    throw new Error('MSISDN is not allowed for HE simulation')
  }

  const operator = cleanString(input.operator) || cleanString(defaults.operator) || 'MTN'
  const operatorKey = normalizeOperatorKey(operator)
  const isCustomOperator = operatorKey === 'CUSTOM'
  let mcc = cleanString(input.mcc) || cleanString(defaults.mcc)
  let mnc = cleanString(input.mnc) || cleanString(defaults.mnc)
  let operatorId = operator

  if (isCustomOperator) {
    mcc ||= '620'
    mnc ||= '01'
    operatorId = `${mcc}-${mnc}`
  } else {
    const opConfig = GHANA_OPERATORS[operatorKey as GhanaOperatorKey]
    if (!opConfig) {
      throw new Error('Unsupported HE operator')
    }
    mcc = opConfig.mcc
    mnc = opConfig.mnc
    operatorId = opConfig.name
  }

  if (!/^\d{3}$/.test(mcc) || !/^\d{2,3}$/.test(mnc)) {
    throw new Error('Invalid MCC/MNC format')
  }

  return {
    identity: {
      msisdn,
      operatorId,
      mcc,
      mnc,
      country: 'GH',
    },
    redirect: normalizeLocalRedirect(cleanString(input.redirect) || cleanString(defaults.redirect) || '/'),
    maskedMsisdn: maskMsisdnForDisplay(msisdn),
  }
}

function cleanString(value: string | null | undefined): string {
  return typeof value === 'string' ? value.trim() : ''
}

function parseCsv(value: string | undefined): string[] {
  return (value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function normalizeOperatorKey(operator: string): string {
  return operator.trim().toUpperCase().replace(/[\s-]+/g, '_')
}

function normalizeLocalRedirect(redirect: string): string {
  const trimmed = redirect.trim()
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) {
    return '/'
  }
  return trimmed
}

/**
 * Create a signed JWT token for HE simulation
 */
export async function createSimulationToken(
  identity: Omit<HEIdentity, 'source'>,
  config: HESimConfig
): Promise<string> {
  if (!config.secret) {
    throw new Error('HE_SIM_SECRET is not configured')
  }

  const secret = new TextEncoder().encode(config.secret)
  const now = Math.floor(Date.now() / 1000)

  const token = await new jose.SignJWT({
    msisdn: normalizeMsisdn(identity.msisdn),
    operatorId: identity.operatorId || undefined,
    mcc: identity.mcc || undefined,
    mnc: identity.mnc || undefined,
    country: identity.country || undefined,
  })
    .setProtectedHeader({ alg: 'HS256' })
    .setIssuedAt(now)
    .setExpirationTime(now + config.ttlSeconds)
    .sign(secret)

  return token
}

/**
 * Verify and decode a simulation token
 * Returns null if token is invalid or expired
 */
export async function verifySimulationToken(
  token: string,
  config: HESimConfig
): Promise<HEIdentity | null> {
  if (!config.secret || !token) {
    return null
  }

  try {
    const secret = new TextEncoder().encode(config.secret)
    const { payload } = await jose.jwtVerify(token, secret, {
      algorithms: ['HS256'],
    })

    const simPayload = payload as unknown as HESimPayload

    if (!simPayload.msisdn) {
      return null
    }

    return {
      msisdn: normalizeMsisdn(simPayload.msisdn),
      operatorId: simPayload.operatorId || undefined,
      mcc: simPayload.mcc || undefined,
      mnc: simPayload.mnc || undefined,
      country: simPayload.country || undefined,
      source: 'SIMULATED',
    }
  } catch {
    // Token is invalid or expired
    return null
  }
}

/**
 * Extract real HE identity from request headers
 * Checks candidate MSISDN headers in order of preference
 */
export function extractRealHEIdentity(
  headers: Headers
): HEIdentity | null {
  // Try each candidate header
  for (const headerName of MSISDN_HEADERS) {
    const msisdn = headers.get(headerName)
    if (msisdn) {
      const normalized = normalizeMsisdn(msisdn)
      if (isValidMsisdn(normalized)) {
        return {
          msisdn: normalized,
          operatorId: headers.get('x-operator-id') || undefined,
          mcc: headers.get('x-mcc') || undefined,
          mnc: headers.get('x-mnc') || undefined,
          country: headers.get('x-country') || undefined,
          source: 'REAL',
        }
      }
    }
  }

  return null
}

/**
 * Resolve HE identity from request
 * Priority: Real HE headers > Simulation cookie > None
 */
export async function resolveHEIdentity(
  headers: Headers,
  cookies: { get: (name: string) => { value: string } | undefined }
): Promise<HEIdentity | null> {
  // 1. Always prefer real HE headers
  const realIdentity = extractRealHEIdentity(headers)
  if (realIdentity) {
    return realIdentity
  }

  // 2. Try simulation cookie if enabled
  const config = getHESimConfig()
  if (!config.enabled) {
    return null
  }

  const simCookie = cookies.get(config.cookieName)
  if (simCookie?.value) {
    const simIdentity = await verifySimulationToken(simCookie.value, config)
    if (simIdentity) {
      return simIdentity
    }
  }

  return null
}
