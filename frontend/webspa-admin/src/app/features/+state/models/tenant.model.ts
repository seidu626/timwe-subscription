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

export interface ChannelUpdatePayload {
  provider?: string;
  country?: string;
  operator?: string;
  capabilities?: string[];
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
  secret_value?: string;
  redacted_display?: string;
  performed_by?: string;
}

/**
 * Credential purposes the platform gives meaning to. A tenant holds at most
 * one ACTIVE credential per purpose, and binding an `otp_api` credential is
 * what moves that tenant's app login from the local OTP lifecycle to the
 * provider's, so this is a mode switch, not an extra layer.
 */
export const CREDENTIAL_PURPOSES: ReadonlyArray<{ value: string; label: string; hint: string }> = [
  {
    value: 'provider_api',
    label: 'Provider API',
    hint: 'Carrier opt-in and billing credentials for this channel.'
  },
  {
    value: 'sms_api',
    label: 'SMS gateway',
    hint: 'Outbound SMS aggregator used to deliver app login codes.'
  },
  {
    value: 'otp_api',
    label: 'Delegated OTP',
    hint: 'Provider mints, sends and checks login codes. Binding this replaces the built-in code delivery for the tenant.'
  }
];

/** Per-field credential blob. Serialized to JSON and sent as secret_value. */
export interface CredentialSecretValue {
  base_url?: string;
  api_key?: string;
  mt_api_key?: string;
  psk?: string;
  partner_service_id?: string;
  partner_role_id?: string;
  realm?: string;
  mcc?: string;
  mnc?: string;
  large_account?: string;
  service_name?: string;
  free_mt_pricepoint_id?: string;
  mo_pricepoint_ids?: string[];
  billing_pricepoint_ids?: string[];
  he_iv_param_spec_key?: string;
}

/** Blob stored for an `sms_api` credential. */
export interface SmsGatewaySecretValue {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body_template?: string;
  sender_id?: string;
  message_template?: string;
  success_field?: string;
  success_value?: string;
}

/** Blob stored for an `otp_api` credential. */
export interface OtpGatewaySecretValue {
  generate_url: string;
  verify_url: string;
  headers?: Record<string, string>;
  sender_id?: string;
  message_template?: string;
  length?: number;
  expiry_minutes?: number;
  medium?: string;
  type?: string;
}

export interface RevokeCredentialResponse {
  credential: AdminChannelCredential;
  was_only_active: boolean;
}
