import { apiRequest } from './client';
import type {
  ConfirmSubscriptionResponse,
  CreateSubscriptionResponse,
  SubscriptionsResponse,
} from './types';

// tenant is the product's owning tenant: the marketplace sells across
// tenants, so the subscription follows the product, not the login tenant.
export function createSubscription(campaignSlug: string, tenant: string): Promise<CreateSubscriptionResponse> {
  return apiRequest<CreateSubscriptionResponse>('/v1/app/subscriptions', {
    method: 'POST',
    body: { campaign_slug: campaignSlug, tenant },
  });
}

export function confirmSubscription(ref: string, pin?: string): Promise<ConfirmSubscriptionResponse> {
  return apiRequest<ConfirmSubscriptionResponse>(
    `/v1/app/subscriptions/${encodeURIComponent(ref)}/confirm`,
    { method: 'POST', body: pin ? { pin } : {} },
  );
}

export function listSubscriptions(): Promise<SubscriptionsResponse> {
  return apiRequest<SubscriptionsResponse>('/v1/app/subscriptions');
}

export function cancelSubscription(ref: string): Promise<void> {
  return apiRequest<void>(`/v1/app/subscriptions/${encodeURIComponent(ref)}`, {
    method: 'DELETE',
  });
}
