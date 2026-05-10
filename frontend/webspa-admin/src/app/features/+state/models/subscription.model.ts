// subscription.model.ts
export interface Subscription {
  id: number;                 // Unique identifier for the subscription
  userIdentifier: string;     // MSISDN or user ID
  userIdentifierType: string; // Type of user identifier (e.g., MSISDN)
  productId: number;          // Product ID associated with the subscription
  shortcode: string;          // Shortcode used for the subscription
  entryChannel: string;       // Channel through which the subscription was made (e.g., SMS, WEB, USSD)
  startDate: Date;            // Subscription start date
  endDate: Date;              // Subscription end date or renewal date
  status: string;             // Status of the subscription (e.g., active, inactive, expired)
  transactionUUID: string;    // Transaction ID for tracking the subscription
  mcc: string;                // Mobile Country Code
  mnc: string;                // Mobile Network Code
  tags?: string[];            // Optional tags associated with the subscription
}



export interface SubscriptionPagedResponse {
  pageSize: number;
  page: number;
  data: Subscription[];
  totalCount: number;
  totalPages: number;
}

export type AdminSubscriptionOperation = 'optin' | 'optout' | 'confirm' | 'status';

export interface AdminSubscriptionActionRequest {
  msisdn: string;
  productId: number;
  partnerRoleId?: number;
  userIdentifierType?: string;
  mcc?: string;
  mnc?: string;
  entryChannel?: string;
  largeAccount?: string;
  subKeyword?: string;
  trackingId?: string;
  clientIp?: string;
  campaignUrl?: string;
  controlKeyword?: string;
  controlServiceKeyword?: string;
  subId?: number;
  cancelReason?: number;
  cancelSource?: number;
  transactionAuthCode?: string;
  externalTxId?: string;
  adminRequestId?: string;
  headers?: Record<string, string>;
}

export interface AdminActionCapturedRequest {
  method: string;
  url: string;
  headers: Record<string, string>;
  body: unknown;
  timestamp: string;
}

export interface AdminActionCapturedResponse {
  statusCode: number;
  headers: Record<string, string>;
  body: unknown;
  timestamp?: string;
  durationMs: number;
}

export interface AdminActionDetail {
  id: string;
  operation: AdminSubscriptionOperation;
  msisdn: string;
  productId: number;
  partnerRoleId: number;
  externalTxId?: string;
  adminRequestId?: string;
  request: AdminActionCapturedRequest;
  response: AdminActionCapturedResponse;
  serviceResult?: unknown;
  error?: unknown;
  createdAt: string;
}

export interface AdminActionSummary {
  id: string;
  operation: AdminSubscriptionOperation;
  msisdn: string;
  productId: number;
  partnerRoleId: number;
  externalTxId?: string;
  adminRequestId?: string;
  responseStatusCode: number;
  durationMs: number;
  createdAt: string;
  hasError: boolean;
  errorMessage?: string;
}

export interface AdminActionHistoryResponse {
  data: AdminActionSummary[];
  totalCount: number;
  page: number;
  pageSize: number;
  totalPages: number;
}
