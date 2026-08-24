export const queryKeys = {
  marketplace: (country: string) => ['marketplace', country] as const,
  subscriptions: ['subscriptions'] as const,
  feed: ['feed'] as const,
  feedItem: (id: string) => ['feed', 'item', id] as const,
  notificationPrefs: ['notification-prefs'] as const,
};
