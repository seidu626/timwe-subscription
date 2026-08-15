import { apiRequest } from './client';
import type { MarketplaceResponse } from './types';

// The marketplace view: every tenant's storefront, grouped per tenant.
// (GET /v1/app/catalog with a tenant arg still returns a flat per-tenant
// list, but the app browses the whole marketplace.)
export function getMarketplace(country: string): Promise<MarketplaceResponse> {
  const params = new URLSearchParams({ country });
  return apiRequest<MarketplaceResponse>(`/v1/app/catalog?${params.toString()}`, { auth: false });
}
