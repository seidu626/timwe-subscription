import { apiRequest } from './client';
import type { CatalogResponse } from './types';

export function getCatalog(tenant: string, country: string): Promise<CatalogResponse> {
  const params = new URLSearchParams({ tenant, country });
  return apiRequest<CatalogResponse>(`/v1/app/catalog?${params.toString()}`, { auth: false });
}
