const CAMPAIGN_TENANT_ALIASES: Record<string, string> = {
  'gh-airteltigo-careerify-daily-v1': 'careerify',
}

export function getTenantAliasForCampaignSlug(slug: string): string {
  return CAMPAIGN_TENANT_ALIASES[slug.trim().toLowerCase()] ?? ''
}
