export type MSISDNRegion = 'GH' | 'NG';
export type VerificationStatus = 'PENDING' | 'VERIFIED' | 'INVALID' | 'ERROR';

export interface MSISDNCatalogRecord {
  id: number;
  tenant_id: string;
  msisdn: string;
  region: MSISDNRegion;
  telco: string;
  verification_status: VerificationStatus;
  dnd: boolean;
  invalid: boolean;
  invalid_reason?: string;
  source: 'BATCH_GENERATED' | 'DND_IMPORT' | 'MANUAL' | string;
  generation_batch_id?: string;
  verification_provider?: string;
  verification_reference?: string;
  verification_attempts: number;
  verified_at?: string;
  last_verification_at?: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface MSISDNCatalogListResponse {
  records: MSISDNCatalogRecord[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface MSISDNCatalogStats {
  total: number;
  ready: number;
  verified: number;
  pending: number;
  dnd: number;
  invalid: number;
  errors: number;
}

export interface MSISDNCatalogFilters {
  page?: number;
  page_size?: number;
  q?: string;
  region?: string;
  telco?: string;
  status?: string;
  dnd?: boolean;
  invalid?: boolean;
}

export interface DNDImportResponse {
  imported: number;
  rejected: number;
  errors?: string[];
}

export interface VerificationSummary {
  requested: number;
  verified: number;
  invalid: number;
  errors: number;
}
