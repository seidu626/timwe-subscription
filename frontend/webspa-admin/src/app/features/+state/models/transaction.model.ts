// transaction.model.ts

export type TransactionStatus = 
  | 'PENDING' 
  | 'ACTION_REQUIRED' 
  | 'CONFIRM_REQUIRED' 
  | 'SUBSCRIBED' 
  | 'CHARGED' 
  | 'FAILED' 
  | 'CANCELLED';

export type TransactionNextAction =
  | 'OPEN_SMS'
  | 'OTP'
  | 'REDIRECT'
  | 'SHOW_INSTRUCTIONS'
  | 'SUBSCRIBED';

export type TransactionHESource = 'REAL' | 'SIMULATED' | 'NONE';

export type PostbackStatus = 'PENDING' | 'PROCESSING' | 'SUCCESS' | 'FAILED' | 'DLQ';

export interface TransactionSummary {
  id: string;
  correlation_id: string;
  campaign_slug: string;
  msisdn: string;
  status: TransactionStatus;
  ad_provider?: string;
  click_id?: string;
  timwe_transaction_id?: string;
  timwe_status?: string;
  conversion_postback_sent: boolean;
  postback_status?: PostbackStatus;
  charged_at?: string;
  created_at: string;
  updated_at: string;
}

export interface TransactionListResponse {
  transactions: TransactionSummary[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface TransactionListFilter {
  campaign_slug?: string;
  status?: string;
  provider?: string;
  start_date?: string;
  end_date?: string;
  sort_by?: string;
  sort_dir?: 'asc' | 'desc';
  page?: number;
  page_size?: number;
}

export interface TransactionStats {
  start_date: string;
  end_date: string;
  total_count: number;
  status_counts: { [key: string]: number };
}

export interface TransactionDetail extends TransactionSummary {
  next_action?: TransactionNextAction;
  next_action_payload?: Record<string, unknown> | null;
  attribution_data?: Record<string, unknown> | null;
  ip_address?: string;
  user_agent?: string;
  consent_required: boolean;
  consent_checked: boolean;
  consent_version?: string;
  consent_timestamp?: string;
  landing_version_hash?: string;
  he_source?: TransactionHESource;
  he_msisdn?: string;
  he_operator?: string;
  offer_product_id?: number;
  pricepoint_id?: number;
  partner_role_id?: number;
  transaction_auth_code?: string;
  charge_payout?: string;
}
