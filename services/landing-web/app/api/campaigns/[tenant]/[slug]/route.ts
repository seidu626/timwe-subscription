import { NextRequest, NextResponse } from 'next/server'

const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'

export async function GET(
  request: NextRequest,
  { params }: { params: { tenant: string; slug: string } }
) {
  const { tenant, slug } = params

  try {
    const response = await fetch(
      `${ACQUISITION_API_URL}/v1/campaigns/${encodeURIComponent(tenant)}/${encodeURIComponent(slug)}`,
      {
        headers: {
          'Content-Type': 'application/json',
        },
        next: { revalidate: 60 },
      }
    )

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Campaign not found' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Failed to fetch tenant campaign:', error)
    return NextResponse.json(
      { error: 'Failed to fetch campaign' },
      { status: 500 }
    )
  }
}
