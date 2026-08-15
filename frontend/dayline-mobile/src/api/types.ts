// Shapes mirror docs/dayline-app-api-contract.md exactly (snake_case wire
// fields kept as-is so payloads need no transform layer).

export type FlowType = 'OTP' | 'DOUBLE_OPTIN' | 'AUTO';
export type SubscriptionStatus = 'ACTIVE' | 'PENDING' | 'CANCELLED' | 'FAILED';
export type NextAction = 'OTP' | 'CONFIRM' | 'SUBSCRIBED';
export type NotificationChannel = 'PUSH' | 'SMS' | 'BOTH';
export type DevicePlatform = 'android' | 'ios';

export interface CatalogProduct {
  slug: string;
  tenant: string;
  tenant_name: string;
  name: string;
  tagline: string;
  description: string;
  category: string;
  artwork_url: string | null;
  sample_content: string | null;
  // Omitted on the wire when a campaign has no price; the backend excludes
  // such campaigns from the app catalog, but the type reflects the payload.
  price?: number;
  currency: string;
  billing_cycle: string;
  flow_type: FlowType;
  subscriber_count: number | null;
}

export interface CatalogResponse {
  products: CatalogProduct[];
}

export interface MarketplaceTenant {
  tenant_key: string;
  tenant_name: string;
  products: CatalogProduct[];
}

export interface MarketplaceResponse {
  tenants: MarketplaceTenant[];
}

export interface CreateSubscriptionResponse {
  subscription_ref: string;
  next_action: NextAction;
  message: string;
}

export interface ConfirmSubscriptionResponse {
  status: SubscriptionStatus;
}

export interface Subscription {
  ref: string;
  tenant: string;
  tenant_name: string;
  product_slug: string;
  product_name: string;
  status: SubscriptionStatus;
  price: number;
  currency: string;
  billing_cycle: string;
  next_charge_hint: string | null;
  started_at: string;
}

export interface SubscriptionsResponse {
  subscriptions: Subscription[];
}

export type ContentKind = 'TEXT' | 'LINK';

export interface FeedItem {
  id: string;
  product_slug: string;
  product_name: string;
  title: string;
  body: string;
  published_at: string;
  read: boolean;
  // Optional-tolerant: older payloads omit these fields entirely, which
  // must behave the same as an explicit content_kind of "TEXT".
  content_kind?: ContentKind;
  link_url?: string | null;
  cta_label?: string | null;
}

export interface FeedResponse {
  items: FeedItem[];
}

export interface OtpVerifyResponse {
  token: string;
  expires_in: number;
}
