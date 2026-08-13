import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { getFeed, getFeedItem, markFeedItemRead } from '@/api/feed';
import { queryKeys } from './queryKeys';

export function useFeed() {
  return useQuery({
    queryKey: queryKeys.feed,
    queryFn: () => getFeed(),
    select: (data) => data.items,
  });
}

export function useFeedItem(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.feedItem(id ?? ''),
    queryFn: () => getFeedItem(id as string),
    enabled: Boolean(id),
  });
}

export function useMarkFeedItemRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => markFeedItemRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.feed });
    },
  });
}
