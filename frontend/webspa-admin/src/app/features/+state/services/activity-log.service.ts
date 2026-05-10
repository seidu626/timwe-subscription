import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import { ActivityLogFilters, ActivityLogListResponse } from '../models/activity-log.model';

@Injectable({
  providedIn: 'root'
})
export class ActivityLogService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/activity-logs`;

  constructor(private http: HttpClient) {}

  list(filters?: ActivityLogFilters): Observable<ActivityLogListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.entity_type) {
        params = params.set('entity_type', filters.entity_type);
      }
      if (filters.action) {
        params = params.set('action', filters.action);
      }
      if (filters.actor) {
        params = params.set('actor', filters.actor);
      }
      if (filters.from) {
        params = params.set('from', filters.from);
      }
      if (filters.to) {
        params = params.set('to', filters.to);
      }
    }
    return this.http.get<ActivityLogListResponse>(this.baseUrl, { params });
  }
}
