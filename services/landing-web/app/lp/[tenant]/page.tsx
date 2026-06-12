import React from 'react'
import type { Metadata } from 'next'
import { getTenantAliasForCampaignSlug } from '@/app/lib/campaign-aliases'
import LandingPageClient from './LandingPageClient'

export async function generateMetadata(
  { params }: { params: Promise<{ tenant: string }> }
): Promise<Metadata> {
  const { tenant: slug } = await params
  const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'
  const tenantAlias = getTenantAliasForCampaignSlug(slug)
  const campaignPath = tenantAlias
    ? `${ACQUISITION_API_URL}/v1/campaigns/${encodeURIComponent(tenantAlias)}/${encodeURIComponent(slug)}`
    : `${ACQUISITION_API_URL}/v1/campaigns/${encodeURIComponent(slug)}`

  try {
    const response = await fetch(campaignPath, {
      next: { revalidate: 3600 }
    })
    
    if (!response.ok) return {}
    
    const campaign = await response.json()
    const text = campaign.lp_copy?.en
    const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || 'https://subscribe.nouveauriche.com'

    return {
      title: text?.heroTitle || 'Nouveauriche Subscription',
      description: text?.heDescription || 'Subscribe to premium mobile services.',
      openGraph: {
        title: text?.heroTitle,
        description: text?.heDescription,
        url: tenantAlias ? `${siteUrl}/lp/${tenantAlias}/${slug}` : `${siteUrl}/lp/${slug}`,
        siteName: 'Nouveauriche',
        type: 'website',
        images: campaign.og_image ? [{ url: campaign.og_image }] : [],
      },
      twitter: {
        card: 'summary_large_image',
        title: text?.heroTitle,
        description: text?.heDescription,
        images: campaign.og_image ? [campaign.og_image] : [],
      },
    }
  } catch (error) {
    console.error('Failed to generate metadata:', error)
    return {}
  }
}

export default function Page() {
  return <LandingPageClient />
}
