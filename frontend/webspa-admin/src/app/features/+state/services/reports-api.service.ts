// reports-api.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  KPIsResponse,
  AcquisitionFunnelResponse,
  CampaignPerformanceResponse,
  TimeSeriesResponse
} from '../models/reports.model';

@Injectable({
  providedIn: 'root'
})
export class ReportsApiService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/reports`;

  constructor(private http: HttpClient) {}

  private buildParams(filters: {
    startDate?: string;
    endDate?: string;
    campaignSlug?: string;
    country?: string;
    interval?: string;
  }): HttpParams {
    let params = new HttpParams();
    if (filters.startDate) {
      params = params.set('startDate', filters.startDate);
    }
    if (filters.endDate) {
      params = params.set('endDate', filters.endDate);
    }
    if (filters.campaignSlug) {
      params = params.set('campaignSlug', filters.campaignSlug);
    }
    if (filters.country) {
      params = params.set('country', filters.country);
    }
    if (filters.interval) {
      params = params.set('interval', filters.interval);
    }
    return params;
  }

  /**
   * Get KPIs for the specified filters
   */
  getKPIs(filters: {
    startDate?: string;
    endDate?: string;
    campaignSlug?: string;
    country?: string;
  }): Observable<KPIsResponse> {
    const params = this.buildParams(filters);
    return this.http.get<KPIsResponse>(`${this.baseUrl}/kpis`, { params });
  }

  /**
   * Get acquisition funnel data
   */
  getAcquisitionFunnel(filters: {
    startDate?: string;
    endDate?: string;
    campaignSlug?: string;
    country?: string;
  }): Observable<AcquisitionFunnelResponse> {
    const params = this.buildParams(filters);
    return this.http.get<AcquisitionFunnelResponse>(`${this.baseUrl}/acquisition-funnel`, { params });
  }

  /**
   * Get campaign performance data
   */
  getCampaignPerformance(filters: {
    startDate?: string;
    endDate?: string;
    campaignSlug?: string;
    country?: string;
  }): Observable<CampaignPerformanceResponse> {
    const params = this.buildParams(filters);
    return this.http.get<CampaignPerformanceResponse>(`${this.baseUrl}/campaign-performance`, { params });
  }

  /**
   * Get time series data for charts
   */
  getTimeSeries(filters: {
    startDate?: string;
    endDate?: string;
    campaignSlug?: string;
    country?: string;
    interval?: string;
  }): Observable<TimeSeriesResponse> {
    const params = this.buildParams(filters);
    return this.http.get<TimeSeriesResponse>(`${this.baseUrl}/timeseries`, { params });
  }

  /**
   * Export campaign performance data as CSV
   * Returns a Blob that can be downloaded
   */
  exportCampaignPerformanceCSV(filters: {
    startDate?: string;
    endDate?: string;
    campaignSlug?: string;
    country?: string;
  }): Observable<Blob> {
    const params = this.buildParams(filters);
    return this.http.get(`${this.baseUrl}/campaign-performance/export`, {
      params,
      responseType: 'blob'
    });
  }
}
