import { apiRequest } from './client';
import type { FeedItem, FeedResponse } from './types';

export function getFeed(): Promise<FeedResponse> {
  return apiRequest<FeedResponse>('/v1/app/feed');
}

export function getFeedItem(id: string): Promise<FeedItem> {
  return apiRequest<FeedItem>(`/v1/app/feed/items/${encodeURIComponent(id)}`);
}

export function markFeedItemRead(id: string): Promise<void> {
  return apiRequest<void>(`/v1/app/feed/items/${encodeURIComponent(id)}/read`, {
    method: 'POST',
  });
}
