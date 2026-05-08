import { NextRequest, NextResponse } from 'next/server'
import {
  HE_HEADER_SOURCE,
  HE_HEADER_MSISDN,
  HE_HEADER_OPERATOR,
  HE_HEADER_MCC,
  HE_HEADER_MNC,
} from '@/middleware'
import {
  HE_BOOTSTRAP_TOKEN_HEADER,
  type BootstrapExchangeResponse,
} from '@/lib/he-types'
import { exchangeBootstrapToken } from '@/lib/he-token-exchange'

const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'

/**
 * Build backend request headers with HE identity from bootstrap exchange or middleware.
 * Priority: bootstrap token exchange (server-side) > middleware HE headers (simulation).
 */
function buildBackendHeaders(request: NextRequest): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  const bootstrapToken = request.headers.get(HE_BOOTSTRAP_TOKEN_HEADER)

  if (bootstrapToken) {
    // Synchronous exchange is done in POST; we only set headers here after exchange
    // So this function is used after we have the exchange result
    return headers
  }

  const heSource = request.headers.get(HE_HEADER_SOURCE)
  const heMsisdn = request.headers.get(HE_HEADER_MSISDN)
  if (heSource && heMsisdn) {
    headers[HE_HEADER_SOURCE] = heSource
    headers[HE_HEADER_MSISDN] = heMsisdn
    const heOperator = request.headers.get(HE_HEADER_OPERATOR)
    const heMcc = request.headers.get(HE_HEADER_MCC)
    const heMnc = request.headers.get(HE_HEADER_MNC)
    if (heOperator) headers[HE_HEADER_OPERATOR] = heOperator
    if (heMcc) headers[HE_HEADER_MCC] = heMcc
    if (heMnc) headers[HE_HEADER_MNC] = heMnc
  }

  return headers
}

/**
 * Apply HE identity from bootstrap exchange result to headers.
 */
function applyBootstrapIdentity(
  headers: Record<string, string>,
  identity: BootstrapExchangeResponse
): void {
  headers[HE_HEADER_SOURCE] = identity.source
  headers[HE_HEADER_MSISDN] = identity.msisdn
  if (identity.operator_id) headers[HE_HEADER_OPERATOR] = identity.operator_id
  if (identity.mcc) headers[HE_HEADER_MCC] = identity.mcc
  if (identity.mnc) headers[HE_HEADER_MNC] = identity.mnc
}

export async function POST(request: NextRequest) {
  let body: unknown

  try {
    body = await request.json()
  } catch {
    return NextResponse.json(
      { error: 'Invalid JSON body' },
      { status: 400 }
    )
  }

  const backendHeaders = buildBackendHeaders(request)

  const bootstrapToken = request.headers.get(HE_BOOTSTRAP_TOKEN_HEADER)
  if (bootstrapToken) {
    const outcome = await exchangeBootstrapToken(ACQUISITION_API_URL, bootstrapToken)
    if (outcome.success) {
      applyBootstrapIdentity(backendHeaders, outcome.identity)
    }
  }

  const response = await fetch(`${ACQUISITION_API_URL}/v1/acquisition/transactions`, {
    method: 'POST',
    headers: backendHeaders,
    body: JSON.stringify(body),
  })

  const data = await parseUpstreamResponse(response)

  if (!response.ok) {
    console.error('Upstream transaction error:', JSON.stringify(data))
    return NextResponse.json(sanitizeErrorResponse(data), { status: response.status })
  }

  return NextResponse.json(data)
}

async function parseUpstreamResponse(response: Response): Promise<unknown> {
  const raw = await response.text()

  if (!raw) {
    return response.ok ? {} : { error: 'Something went wrong. Please try again.' }
  }

  try {
    return JSON.parse(raw)
  } catch {
    return { error: raw }
  }
}

function sanitizeErrorResponse(data: unknown): Record<string, unknown> {
  const safe: Record<string, unknown> = { error: 'Something went wrong. Please try again.' }

  if (data && typeof data === 'object') {
    const obj = data as Record<string, unknown>
    if (typeof obj.transaction_id === 'string') safe.transaction_id = obj.transaction_id
    if (typeof obj.status === 'string') safe.status = obj.status

    const msg = typeof obj.error === 'string' ? obj.error
              : typeof obj.message === 'string' ? obj.message
              : null
    if (msg && !isInternalMessage(msg)) {
      safe.error = msg
    }
  }

  return safe
}

function isInternalMessage(msg: string): boolean {
  const lower = msg.toLowerCase()
  return (
    lower.includes('internal_error') ||
    lower.includes('internal server error') ||
    lower.includes('generic_error_code') ||
    lower.includes('status code:') ||
    lower.includes('mt response') ||
    lower.includes('timwe') ||
    lower.includes('circuit breaker') ||
    lower.includes('request failed') ||
    lower.includes('marshal') ||
    lower.includes('auth key') ||
    lower.includes('partnerrole') ||
    lower.includes('acquisition api')
  )
}
