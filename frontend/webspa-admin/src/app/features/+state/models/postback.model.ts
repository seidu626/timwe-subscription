// postback.model.ts

export type PostbackStatus = 'PENDING' | 'PROCESSING' | 'SUCCESS' | 'FAILED' | 'DLQ';

export type PostbackEvent = 'subscribed' | 'failed' | 'cancelled' | 'conversion';

export interface PostbackOutbox {
  id: string;
  transaction_id: string;
  event: PostbackEvent;
  provider: string;
  url_template_rendered: string;
  http_method: string;
  headers: string; // JSON string
  body?: string;
  attempt_count: number;
  max_attempts: number;
  next_retry_at?: string;
  status: PostbackStatus;
  created_at: string;
  updated_at: string;
}

export interface PostbackAttempt {
  id: string;
  outbox_id: string;
  attempt_number: number;
  http_status?: number;
  response_body?: string;
  error_message?: string;
  duration_ms?: number;
  created_at: string;
}

export interface PostbackLookupResponse {
  transaction_id: string;
  postbacks: PostbackOutbox[];
  attempts: PostbackAttempt[];
}

export interface PostbackLookupRequest {
  transaction_id: string;
}

export interface PostbackStats {
  pending: number;
  processing: number;
  success: number;
  failed: number;
  dlq: number;
  total: number;
  alert: boolean;
}

export interface PostbackStatusResponse {
  status?: PostbackStatus;
  count?: number;
  limit?: number;
  offset?: number;
  postbacks: PostbackOutbox[];
}

export interface PostbackStatusApiResponse {
  status?: PostbackStatus;
  count?: number;
  limit?: number;
  offset?: number;
  items?: PostbackOutbox[];
  postbacks?: PostbackOutbox[];
}

export interface BulkRequeueResponse {
  requeued: number;
  limit?: number;
  offset?: number;
}
