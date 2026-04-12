import { NextRequest, NextResponse } from 'next/server'

const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'

export async function POST(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const { id } = params

  try {
    const body = await request.json()

    const response = await fetch(
      `${ACQUISITION_API_URL}/v1/acquisition/transactions/${id}/confirm`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      }
    )

    const data = await parseUpstreamResponse(response)

    if (!response.ok) {
      console.error('Upstream confirm error:', JSON.stringify(data))
      return NextResponse.json(
        sanitizeErrorResponse(data),
        { status: response.status }
      )
    }

    return NextResponse.json(data)
  } catch (error) {
    console.error('Failed to confirm transaction:', error)
    return NextResponse.json(
      { error: 'Something went wrong. Please try again.' },
      { status: 500 }
    )
  }
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

    // Preserve status/transaction_id if present (non-sensitive)
    if (typeof obj.transaction_id === 'string') safe.transaction_id = obj.transaction_id
    if (typeof obj.status === 'string') safe.status = obj.status
    if (obj.payload && typeof obj.payload === 'object') {
      const payload = obj.payload as Record<string, unknown>
      if (typeof payload.message === 'string' && !isInternalMessage(payload.message)) {
        safe.error = payload.message
      }
    }

    // If the top-level error/message is user-safe, use it
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
    lower.includes('partnerrole')
  )
}
