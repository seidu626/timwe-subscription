import { useQuery } from '@tanstack/react-query';

import { getCatalog } from '@/api/catalog';
import { COUNTRY_CODE, TENANT_KEY } from '@/config';
import { queryKeys } from './queryKeys';

export function useCatalog() {
  return useQuery({
    queryKey: queryKeys.catalog(TENANT_KEY, COUNTRY_CODE),
    queryFn: () => getCatalog(TENANT_KEY, COUNTRY_CODE),
    select: (data) => data.products,
  });
}

export function useCatalogProduct(slug: string | undefined) {
  const catalog = useCatalog();
  const product = slug ? catalog.data?.find((item) => item.slug === slug) : undefined;
  return { ...catalog, product };
}
