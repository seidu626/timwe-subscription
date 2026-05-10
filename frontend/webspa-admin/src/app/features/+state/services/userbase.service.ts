import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  UpsertUserbaseRequest,
  UserbaseFilters,
  UserbaseImportDetailResponse,
  UserbaseImportListResponse,
  UserbaseImportUploadResponse,
  UserbaseListResponse,
  UserbaseRecord
} from '../models/userbase.model';

@Injectable({
  providedIn: 'root'
})
export class UserbaseService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/userbase`;

  constructor(private http: HttpClient) {}

  list(filters?: UserbaseFilters): Observable<UserbaseListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.msisdn) {
        params = params.set('msisdn', filters.msisdn);
      }
      if (filters.type) {
        params = params.set('type', filters.type);
      }
    }
    return this.http.get<UserbaseListResponse>(this.baseUrl, { params });
  }

  upsert(payload: UpsertUserbaseRequest): Observable<UserbaseRecord> {
    return this.http.post<UserbaseRecord>(this.baseUrl, payload);
  }

  delete(msisdn: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${encodeURIComponent(msisdn)}`);
  }

  upload(file: File): Observable<UserbaseImportUploadResponse> {
    const formData = new FormData();
    formData.append('file', file);
    return this.http.post<UserbaseImportUploadResponse>(`${this.baseUrl}/imports`, formData);
  }

  listImports(page = 1, pageSize = 20): Observable<UserbaseImportListResponse> {
    const params = new HttpParams()
      .set('page', page.toString())
      .set('page_size', pageSize.toString());
    return this.http.get<UserbaseImportListResponse>(`${this.baseUrl}/imports`, { params });
  }

  getImport(id: string): Observable<UserbaseImportDetailResponse> {
    return this.http.get<UserbaseImportDetailResponse>(`${this.baseUrl}/imports/${encodeURIComponent(id)}`);
  }
}
