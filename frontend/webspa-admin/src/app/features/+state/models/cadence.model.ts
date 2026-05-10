export interface CadenceSeries {
  id: number;
  partner_role_id: number;
  product_id: number;
  name: string;
  mode: 'SEQUENTIAL' | 'POOL' | string;
  content_version: number;
  is_active: boolean;
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
  is_active: boolean;
  created_at?: string;
}

export interface CadenceCsvImportResult {
  dry_run: boolean;
  series_count: number;
  row_count: number;
  upserted?: number;
  deactivated?: number;
  errors?: Array<{ line: number; error: string }>;
}

