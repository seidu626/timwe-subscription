import { NextRequest, NextResponse } from 'next/server'
import { getTenantAliasForCampaignSlug } from '@/app/lib/campaign-aliases'

const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ tenant: string }> }
) {
  const { tenant: slug } = await params
  const tenantAlias = getTenantAliasForCampaignSlug(slug)

  try {
    const campaignPath = tenantAlias
      ? `${ACQUISITION_API_URL}/v1/campaigns/${encodeURIComponent(tenantAlias)}/${encodeURIComponent(slug)}`
      : `${ACQUISITION_API_URL}/v1/campaigns/${encodeURIComponent(slug)}`
    const response = await fetch(campaignPath, {
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
