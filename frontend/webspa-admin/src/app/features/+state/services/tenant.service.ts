import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
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

  update(id: string, payload: TenantMutationPayload): Observable<TenantMutationResponse> {
    return this.http.patch<TenantMutationResponse>(`${this.baseUrl}/${encodeURIComponent(id)}`, payload);
  }
}
