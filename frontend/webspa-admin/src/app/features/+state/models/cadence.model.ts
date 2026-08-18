export type CadenceDeliveryChannel = 'USER_PREF' | 'SMS' | 'PUSH';

export type CadenceContentKind = 'TEXT' | 'LINK';

export interface CadenceSeries {
  id: number;
  partner_role_id: number;
  product_id: number;
  name: string;
  mode: 'SEQUENTIAL' | 'POOL' | string;
  content_version: number;
  is_active: boolean;
  delivery_channel: CadenceDeliveryChannel | string;
  created_at?: string;
}

export interface CadenceScheduleRule {
  series_id: number;
  rule_kind: 'DAILY' | 'WEEKLY' | 'EVERY_N_DAYS' | string;
  preferred_time: string; // HH:MM:SS preferred from backend
  days_of_week: number;
  n_days: number;
  send_start_time: string;
  send_end_time: string;
  timezone: string;
  max_per_day: number;
  catchup_mode: 'SEND' | 'SKIP' | 'THROTTLE' | string;
}

export interface CadenceContentItem {
  id: number;
  series_id: number;
  content_version: number;
  seq_no: number;
  message_text: string;
  content_kind: CadenceContentKind | string;
  link_url?: string;
  cta_label?: string;
  is_active: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface CadenceContentImpact {
  series_id: number;
  content_version: number;
  is_live: boolean;
  active_states: number;
  pending_jobs: number;
}

export interface CadenceCloneResult {
  status: string;
  series_id: number;
  from_version: number;
  to_version: number;
  items_copied: number;
}

export interface CadenceCsvImportResult {
  dry_run: boolean;
  series_count: number;
  row_count: number;
  upserted?: number;
  deactivated?: number;
  errors?: Array<{ line: number; error: string }>;
}


export interface CadenceSeriesHealth {
  series_id: number;
  name: string;
  is_active: boolean;
  delivery_channel: string;
  product_id: number;
  partner_role_id: number;
  active_states: number;
  paused_states: number;
  stopped_states: number;
  next_due_at?: string | null;
  last_sent_at?: string | null;
  sent_24h: number;
  failed_24h: number;
  sent_7d: number;
  failed_7d: number;
  sent_total: number;
  failed_total: number;
  last_error?: string | null;
  last_failed_at?: string | null;
}

export interface CadencePreviewOccurrence {
  n: number;
  send_at: string;
  seq_no?: number;
  message_text?: string;
  content_kind?: string;
  link_url?: string;
  cta_label?: string;
  ends_series?: boolean;
}

export interface CadenceSeriesPreview {
  series_id: number;
  mode: string;
  content_version: number;
  timezone: string;
  pool_size: number;
  occurrences: CadencePreviewOccurrence[];
}
