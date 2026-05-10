export interface UserbaseRecord {
  id: number;
  msisdn: string;
  type: string;
}

export interface UserbaseListResponse {
  records: UserbaseRecord[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface UserbaseFilters {
  page?: number;
  page_size?: number;
  msisdn?: string;
  type?: string;
}

export interface UpsertUserbaseRequest {
  msisdn: string;
  type: string;
  performed_by?: string;
}

export interface UserbaseImportJob {
  id: string;
  filename: string;
  status: 'PROCESSING' | 'COMPLETED' | 'FAILED';
  total_rows: number;
  success_rows: number;
  failed_rows: number;
  started_at: string;
  completed_at?: string;
  created_by?: string;
}

export interface UserbaseImportError {
  id: number;
  job_id: string;
  row_number: number;
  raw_row: string;
  error_message: string;
}

export interface UserbaseImportListResponse {
  jobs: UserbaseImportJob[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface UserbaseImportDetailResponse {
  job: UserbaseImportJob;
  errors: UserbaseImportError[];
  total_errors: number;
}

export interface UserbaseImportUploadResponse {
  job: UserbaseImportJob;
  errors: UserbaseImportError[];
}
