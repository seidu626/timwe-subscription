// Campaign slug → tenant aliases are no longer required: the acquisition-api
// resolves the owning tenant server-side from the globally-unique campaign slug
// (GET /v1/campaigns/{slug}), and the campaign response carries tenant_key for
// the tenant-scoped opt-in. This override map is kept only as an escape hatch
// for the rare case where a slug must be pinned to a specific tenant; leave it
// empty for normal operation.
const CAMPAIGN_TENANT_ALIASES: Record<string, string> = {}

export function getTenantAliasForCampaignSlug(slug: string): string {
  return CAMPAIGN_TENANT_ALIASES[slug.trim().toLowerCase()] ?? ''
}
