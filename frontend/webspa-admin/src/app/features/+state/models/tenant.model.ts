export type TenantStatus = 'ACTIVE' | 'INACTIVE';
export type TenantMemberStatus = 'ACTIVE' | 'INACTIVE';
export type TenantMemberRole = 'TENANT_ADMIN' | 'TENANT_VIEWER';
export type ChannelStatus = 'ACTIVE' | 'INACTIVE';
export type ChannelCredentialStatus = 'ACTIVE' | 'INACTIVE' | 'REVOKED';

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

export interface AdminChannel {
  channel_id: string;
  tenant_id: string;
  channel_key: string;
  provider: string;
  country: string;
  operator?: string;
  capabilities: string[];
  status: ChannelStatus;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChannelListResponse {
  channels: AdminChannel[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface ChannelFilters {
  page?: number;
  page_size?: number;
  provider?: string;
  country?: string;
  enabled?: boolean;
}

export interface ChannelCreatePayload {
  provider: string;
  country: string;
  operator?: string;
  capabilities: string[];
  enabled?: boolean;
  performed_by?: string;
}

export interface AdminChannelCredential {
  credential_id: string;
  tenant_id: string;
  channel_id: string;
  purpose: string;
  version: number;
  status: ChannelCredentialStatus;
  redacted_display: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
  activated_at?: string;
  deactivated_at?: string;
}

export interface ChannelCredentialListResponse {
  credentials: AdminChannelCredential[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface ChannelCredentialPayload {
  purpose?: string;
  secret_ref?: string;
  redacted_display?: string;
  performed_by?: string;
}

export interface RevokeCredentialResponse {
  credential: AdminChannelCredential;
  was_only_active: boolean;
}
