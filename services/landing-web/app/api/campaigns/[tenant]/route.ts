import { NextRequest, NextResponse } from 'next/server'

const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ tenant: string }> }
) {
  const { tenant: slug } = await params

  try {
    const response = await fetch(`${ACQUISITION_API_URL}/v1/campaigns/${encodeURIComponent(slug)}`, {
      headers: {
        'Content-Type': 'application/json',
      },
      // Don't cache campaign data for too long in development
      next: { revalidate: 60 },
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Campaign not found' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Failed to fetch campaign:', error)
    return NextResponse.json(
      { error: 'Failed to fetch campaign' },
      { status: 500 }
    )
  }
}
