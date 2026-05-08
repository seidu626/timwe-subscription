/**
 * Header Enrichment (HE) domain types and API contracts.
 * Single source of truth for HE identity and bootstrap token exchange (DDD, DRY).
 */

/** Source of HE identity (matches acquisition-api HESource) */
export type HESource = 'REAL' | 'SIMULATED' | 'NONE'

/**
 * HE identity headers used between middleware/API routes.
 * These are the canonical header names (lowercase for HTTP/2 compatibility).
 */
export const HE_HEADERS = {
  SOURCE: 'x-he-source',
  MSISDN: 'x-he-msisdn',
  OPERATOR: 'x-he-operator',
  MCC: 'x-he-mcc',
  MNC: 'x-he-mnc',
} as const

/**
 * Response from acquisition-api POST /v1/he/token/exchange
 * Contract: backend returns snake_case fields (see he_bootstrap_handler.HandleTokenExchange)
 */
export interface BootstrapExchangeResponse {
  msisdn: string
  operator_id?: string
  mcc?: string
  mnc?: string
  source: HESource
  campaign?: string
}

/** Type guard: ensure API response is valid BootstrapExchangeResponse */
export function isBootstrapExchangeResponse(
  value: unknown
): value is BootstrapExchangeResponse {
  if (value === null || typeof value !== 'object') return false
  const o = value as Record<string, unknown>
  return (
    typeof o.msisdn === 'string' &&
    o.msisdn.length > 0 &&
    typeof o.source === 'string'
  )
}

/** Header name for bootstrap token (client → server) */
export const HE_BOOTSTRAP_TOKEN_HEADER = 'x-he-bootstrap-token'
