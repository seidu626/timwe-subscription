/**
 * Server-side HE bootstrap token exchange.
 * Single responsibility: call acquisition-api token exchange and return typed result.
 * Used by API routes only (not exposed to client).
 */

import {
  type BootstrapExchangeResponse,
  isBootstrapExchangeResponse,
} from './he-types'

export interface HeTokenExchangeResult {
  success: true
  identity: BootstrapExchangeResponse
}

export interface HeTokenExchangeFailure {
  success: false
  reason: 'network' | 'invalid_response' | 'expired_or_used'
  status?: number
}

export type HeTokenExchangeOutcome = HeTokenExchangeResult | HeTokenExchangeFailure

/**
 * Exchange HE bootstrap token for identity via acquisition-api.
 * Returns typed outcome; does not throw.
 */
export async function exchangeBootstrapToken(
  baseUrl: string,
  token: string
): Promise<HeTokenExchangeOutcome> {
  const url = `${baseUrl.replace(/\/$/, '')}/v1/he/token/exchange?token=${encodeURIComponent(token)}`

  let response: Response
  try {
    response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    })
  } catch {
    return { success: false, reason: 'network' }
  }

  if (!response.ok) {
    return {
      success: false,
      reason: response.status === 404 ? 'expired_or_used' : 'invalid_response',
      status: response.status,
    }
  }

  let data: unknown
  try {
    data = await response.json()
  } catch {
    return { success: false, reason: 'invalid_response', status: response.status }
  }

  if (!isBootstrapExchangeResponse(data)) {
    return { success: false, reason: 'invalid_response', status: response.status }
  }

  return { success: true, identity: data }
}
