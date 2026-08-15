import { useQuery } from '@tanstack/react-query';

import { getMarketplace } from '@/api/catalog';
import { COUNTRY_CODE } from '@/config';
import { queryKeys } from './queryKeys';

export function useMarketplace() {
  return useQuery({
    queryKey: queryKeys.marketplace(COUNTRY_CODE),
    queryFn: () => getMarketplace(COUNTRY_CODE),
    select: (data) => data.tenants,
  });
}

// Campaign slugs are globally unique (enforced by a DB unique index), so a
// slug alone identifies a product anywhere in the marketplace.
export function useCatalogProduct(slug: string | undefined) {
  const marketplace = useMarketplace();
  const product = slug
    ? marketplace.data?.flatMap((tenant) => tenant.products).find((item) => item.slug === slug)
    : undefined;
  return { ...marketplace, product };
}

export function useMarketplaceTenant(tenantKey: string | undefined) {
  const marketplace = useMarketplace();
  const tenant = tenantKey ? marketplace.data?.find((item) => item.tenant_key === tenantKey) : undefined;
  return { ...marketplace, tenant };
}
