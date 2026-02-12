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

  const data = await response.json()

  if (!response.ok) {
    return NextResponse.json(data, { status: response.status })
  }

  return NextResponse.json(data)
}
