import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  CadenceSeries,
  CadenceScheduleRule,
  CadenceContentItem,
  CadenceCsvImportResult,
} from '../models/cadence.model';

@Injectable({
  providedIn: 'root',
})
export class CadenceApiService {
  private baseUrl = `${environment.cadenceEngineEndpoint}/v1/admin/cadence`;
  private cadenceHeaders = environment.cadenceAdminToken
    ? new HttpHeaders({ 'X-Admin-Token': environment.cadenceAdminToken })
    : undefined;

  constructor(private http: HttpClient) {}

  listSeries(filters?: {
    partnerRoleId?: number;
    productId?: number;
    active?: boolean;
    limit?: number;
  }): Observable<{ series: CadenceSeries[] }> {
    let params = new HttpParams();
    if (filters?.partnerRoleId) params = params.set('partnerRoleId', String(filters.partnerRoleId));
    if (filters?.productId) params = params.set('productId', String(filters.productId));
    if (typeof filters?.active === 'boolean') params = params.set('active', String(filters.active));
    if (filters?.limit) params = params.set('limit', String(filters.limit));
    return this.http.get<{ series: CadenceSeries[] }>(`${this.baseUrl}/series`, { params, headers: this.cadenceHeaders });
  }

  upsertSeries(payload: {
    partner_role_id: number;
    product_id: number;
    name: string;
    mode?: string;
    content_version?: number;
    is_active?: boolean;
  }): Observable<CadenceSeries> {
    return this.http.post<CadenceSeries>(`${this.baseUrl}/series`, payload, { headers: this.cadenceHeaders });
  }

  getSeries(seriesId: number): Observable<CadenceSeries> {
    return this.http.get<CadenceSeries>(`${this.baseUrl}/series/${seriesId}`, { headers: this.cadenceHeaders });
  }

  patchSeries(seriesId: number, payload: {
    is_active?: boolean;
    mode?: string;
    content_version?: number;
  }): Observable<CadenceSeries> {
    return this.http.patch<CadenceSeries>(`${this.baseUrl}/series/${seriesId}`, payload, { headers: this.cadenceHeaders });
  }

  getRule(seriesId: number): Observable<CadenceScheduleRule> {
    return this.http.get<CadenceScheduleRule>(`${this.baseUrl}/series/${seriesId}/rule`, { headers: this.cadenceHeaders });
  }

  putRule(seriesId: number, payload: {
    rule_kind: string;
    preferred_time: string;
    days_of_week?: number;
    n_days?: number;
    send_start_time: string;
    send_end_time: string;
    timezone: string;
    max_per_day: number;
    catchup_mode: string;
  }): Observable<{ status: string }> {
    return this.http.put<{ status: string }>(`${this.baseUrl}/series/${seriesId}/rule`, payload, { headers: this.cadenceHeaders });
  }

  listContent(seriesId: number, filters?: {
    contentVersion?: number;
    active?: boolean;
    limit?: number;
  }): Observable<{ items: CadenceContentItem[] }> {
    let params = new HttpParams();
    if (filters?.contentVersion) params = params.set('contentVersion', String(filters.contentVersion));
    if (typeof filters?.active === 'boolean') params = params.set('active', String(filters.active));
    if (filters?.limit) params = params.set('limit', String(filters.limit));
    return this.http.get<{ items: CadenceContentItem[] }>(`${this.baseUrl}/series/${seriesId}/content`, { params, headers: this.cadenceHeaders });
  }

  upsertContent(seriesId: number, payload: {
    content_version: number;
    seq_no: number;
    message_text: string;
    is_active?: boolean;
  }): Observable<{ status: string }> {
    return this.http.post<{ status: string }>(`${this.baseUrl}/series/${seriesId}/content`, payload, { headers: this.cadenceHeaders });
  }

  importCsv(file: File, dryRun: boolean): Observable<CadenceCsvImportResult> {
    const form = new FormData();
    form.append('file', file);
    const params = new HttpParams().set('dryRun', String(dryRun));
    return this.http.post<CadenceCsvImportResult>(`${this.baseUrl}/content/import/csv`, form, { params, headers: this.cadenceHeaders });
  }

  publishVersion(seriesId: number, contentVersion: number): Observable<{
    status: string;
    series_id: number;
    previous_version: number;
    published_version: number;
  }> {
    return this.http.post<{
      status: string;
      series_id: number;
      previous_version: number;
      published_version: number;
    }>(`${this.baseUrl}/series/${seriesId}/publish`, { content_version: contentVersion }, { headers: this.cadenceHeaders });
  }
}
