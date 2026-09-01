import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  DNDImportResponse,
  MSISDNCatalogFilters,
  MSISDNCatalogListResponse,
  MSISDNCatalogRecord,
  MSISDNCatalogStats,
  VerificationSummary
} from '../models/msisdn-catalog.model';

@Injectable({ providedIn: 'root' })
export class MSISDNCatalogService {
  private readonly baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/msisdn-catalog`;

  constructor(private readonly http: HttpClient) {}

  list(filters: MSISDNCatalogFilters): Observable<MSISDNCatalogListResponse> {
    let params = new HttpParams();
    Object.entries(filters).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        params = params.set(key, String(value));
      }
    });
    return this.http.get<MSISDNCatalogListResponse>(this.baseUrl, { params });
  }

  stats(): Observable<MSISDNCatalogStats> {
    return this.http.get<MSISDNCatalogStats>(`${this.baseUrl}/stats`);
  }

  uploadDND(file: File, region: string, telco: string): Observable<DNDImportResponse> {
    const body = new FormData();
    body.append('file', file);
    body.append('region', region);
    body.append('telco', telco);
    return this.http.post<DNDImportResponse>(`${this.baseUrl}/dnd-imports`, body);
  }

  verify(ids: number[] = [], limit = 100): Observable<VerificationSummary> {
    return this.http.post<VerificationSummary>(`${this.baseUrl}/verify`, { ids, limit });
  }

  setFlags(id: number, flags: { dnd?: boolean; invalid?: boolean; reason?: string }): Observable<MSISDNCatalogRecord> {
    return this.http.patch<MSISDNCatalogRecord>(`${this.baseUrl}/${id}`, flags);
  }
}
