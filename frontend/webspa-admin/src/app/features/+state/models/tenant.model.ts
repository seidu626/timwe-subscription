export type TenantStatus = 'ACTIVE' | 'INACTIVE';

export interface AdminTenant {
  id: string;
  tenant_key: string;
  name: string;
  status: TenantStatus;
  default_country: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface TenantListResponse {
  tenants: AdminTenant[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface TenantFilters {
  page?: number;
  page_size?: number;
  q?: string;
  status?: TenantStatus | '';
}

export interface TenantMutationPayload {
  name?: string;
  status?: TenantStatus;
  default_country?: string;
  metadata?: Record<string, unknown>;
  performed_by?: string;
}

export interface TenantMutationResponse extends AdminTenant {
  audit_log_id: string;
}
