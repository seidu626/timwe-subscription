export type TenantStatus = 'ACTIVE' | 'INACTIVE';
export type TenantMemberStatus = 'ACTIVE' | 'INACTIVE';
export type TenantMemberRole = 'TENANT_ADMIN' | 'TENANT_VIEWER';

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

export interface TenantCreatePayload extends TenantMutationPayload {
  tenant_key: string;
  name: string;
  status: TenantStatus;
  default_country: string;
}

export interface TenantMutationResponse extends AdminTenant {
  audit_log_id: string;
}

export interface AdminTenantMember {
  id: string;
  tenant_id: string;
  auth0_subject: string;
  email?: string;
  role: TenantMemberRole;
  status: TenantMemberStatus;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface TenantMemberListResponse {
  members: AdminTenantMember[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface TenantMemberPayload {
  auth0_subject: string;
  email?: string;
  role: TenantMemberRole;
  status: TenantMemberStatus;
  performed_by?: string;
}

export interface TenantMemberMutationResponse extends AdminTenantMember {
  audit_log_id: string;
}
