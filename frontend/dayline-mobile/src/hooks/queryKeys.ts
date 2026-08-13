export const queryKeys = {
  catalog: (tenant: string, country: string) => ['catalog', tenant, country] as const,
  subscriptions: ['subscriptions'] as const,
  feed: ['feed'] as const,
  feedItem: (id: string) => ['feed', 'item', id] as const,
};
