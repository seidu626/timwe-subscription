import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  TenantMemberListResponse,
  TenantMemberMutationResponse,
  TenantMemberPayload,
  TenantCreatePayload,
  TenantFilters,
  TenantListResponse,
  TenantMutationPayload,
  TenantMutationResponse
} from '../models/tenant.model';

@Injectable({
  providedIn: 'root'
})
export class TenantService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/tenants`;

  constructor(private http: HttpClient) {}

  list(filters?: TenantFilters): Observable<TenantListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.q) {
        params = params.set('q', filters.q);
      }
      if (filters.status) {
        params = params.set('status', filters.status);
      }
    }
    return this.http.get<TenantListResponse>(this.baseUrl, { params });
  }

  create(payload: TenantCreatePayload): Observable<TenantMutationResponse> {
    return this.http.post<TenantMutationResponse>(this.baseUrl, payload);
  }

  update(id: string, payload: TenantMutationPayload): Observable<TenantMutationResponse> {
    return this.http.patch<TenantMutationResponse>(`${this.baseUrl}/${encodeURIComponent(id)}`, payload);
  }

  listMembers(tenantId: string, filters?: TenantFilters): Observable<TenantMemberListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.q) {
        params = params.set('q', filters.q);
      }
      if (filters.status) {
        params = params.set('status', filters.status);
      }
    }
    return this.http.get<TenantMemberListResponse>(`${this.baseUrl}/${encodeURIComponent(tenantId)}/members`, { params });
  }

  upsertMember(tenantId: string, payload: TenantMemberPayload): Observable<TenantMemberMutationResponse> {
    return this.http.post<TenantMemberMutationResponse>(`${this.baseUrl}/${encodeURIComponent(tenantId)}/members`, payload);
  }

  deactivateMember(tenantId: string, auth0Subject: string): Observable<{ audit_log_id: string }> {
    return this.http.delete<{ audit_log_id: string }>(
      `${this.baseUrl}/${encodeURIComponent(tenantId)}/members/${encodeURIComponent(auth0Subject)}`
    );
  }
}
