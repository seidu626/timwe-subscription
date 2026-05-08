import { NextRequest, NextResponse } from 'next/server'
import { HE_BOOTSTRAP_TOKEN_HEADER } from '@/lib/he-types'

const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'

/**
 * POST /api/analytics/landing
 * Proxies landing page events to acquisition-api for funnel reporting.
 * Forwards X-HE-Bootstrap-Token when present (for future HE enrichment).
 */
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

  if (
    typeof body !== 'object' ||
    body === null ||
    !('event_type' in body) ||
    !('campaign_slug' in body)
  ) {
    return NextResponse.json(
      { error: 'event_type and campaign_slug are required' },
      { status: 400 }
    )
  }

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  const bootstrapToken = request.headers.get(HE_BOOTSTRAP_TOKEN_HEADER)
  if (bootstrapToken) {
    headers[HE_BOOTSTRAP_TOKEN_HEADER] = bootstrapToken
  }

  const response = await fetch(`${ACQUISITION_API_URL}/v1/analytics/landing/events`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
  })

  const data = await response.json()

  if (!response.ok) {
    return NextResponse.json(data, { status: response.status })
  }

  return NextResponse.json(data)
}
