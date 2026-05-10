export interface AdminActivityLog {
  id: string;
  entity_type: string;
  entity_id: string;
  action: string;
  actor?: string;
  request_id?: string;
  before_json?: string;
  after_json?: string;
  metadata?: string;
  created_at: string;
}

export interface ActivityLogFilters {
  page?: number;
  page_size?: number;
  entity_type?: string;
  action?: string;
  actor?: string;
  from?: string;
  to?: string;
}

export interface ActivityLogListResponse {
  items: AdminActivityLog[];
  total_count: number;
  page: number;
  page_size: number;
}
