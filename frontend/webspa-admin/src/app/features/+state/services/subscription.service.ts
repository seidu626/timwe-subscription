import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { catchError, map, Observable, throwError } from 'rxjs';
import { environment } from 'src/environments/environment';
import { DataService } from '../../../core/services';
import {
  AdminActionDetail,
  AdminActionHistoryResponse,
  AdminSubscriptionActionRequest,
  AdminSubscriptionOperation,
  Subscription,
  SubscriptionPagedResponse,
} from '../models/subscription.model';

@Injectable({
  providedIn: 'root'
})
export class SubscriptionService {
  private readonly baseUrl = environment.subscriptionApiEndpoint + '/api/v1/subscription';
  private readonly adminBaseUrl = environment.subscriptionExternalAdminApiEndpoint + '/api/v1/subscription-external/admin';

  constructor(
    private dataService: DataService,
    private http: HttpClient,
  ) {}

  getSubscriptions(filters: any): Observable<SubscriptionPagedResponse> {
    let params = new HttpParams();
    Object.keys(filters).forEach(key => {
      if (filters[key] !== undefined && filters[key] !== null && filters[key] !== '') {
        params = params.set(key, filters[key]);
      }
    });

    return this.dataService.get(`${this.baseUrl}/list`, params, true).pipe(
      map((response: any) => {
        if (!response || !response.headers) {
          throw new Error('Missing headers in the response');
        }

        const result: SubscriptionPagedResponse = {
          pageSize: +response.body.pageSize || 10,
          page: +response.body.page || 1,
          totalCount: +response.body.totalCount || 0,
          data: response.body.data || [],
          totalPages: response.body.totalPages || 1,
        };
        return result;
      }),
      catchError(error => {
        console.error('Error fetching subscriptions:', error);
        return throwError(() => error);
      })
    );
  }

  getSubscriptionById(id: number): Observable<Subscription> {
    return this.dataService.get(`${this.baseUrl}/${id}`);
  }

  executeAdminAction(operation: AdminSubscriptionOperation, payload: AdminSubscriptionActionRequest): Observable<AdminActionDetail> {
    return this.http.post<AdminActionDetail>(`${this.adminBaseUrl}/${operation}`, payload);
  }

  getAdminActionHistory(filters: {
    operation?: AdminSubscriptionOperation | '';
    msisdn?: string;
    externalTxId?: string;
    adminRequestId?: string;
    page?: number;
    pageSize?: number;
  }): Observable<AdminActionHistoryResponse> {
    let params = new HttpParams();

    if (filters.operation) {
      params = params.set('operation', filters.operation);
    }
    if (filters.msisdn) {
      params = params.set('msisdn', filters.msisdn);
    }
    if (filters.externalTxId) {
      params = params.set('externalTxId', filters.externalTxId);
    }
    if (filters.adminRequestId) {
      params = params.set('adminRequestId', filters.adminRequestId);
    }
    if (filters.page) {
      params = params.set('page', filters.page.toString());
    }
    if (filters.pageSize) {
      params = params.set('pageSize', filters.pageSize.toString());
    }

    return this.http.get<AdminActionHistoryResponse>(`${this.adminBaseUrl}/actions`, { params });
  }

  getAdminActionById(actionId: string): Observable<AdminActionDetail> {
    return this.http.get<AdminActionDetail>(`${this.adminBaseUrl}/actions/${encodeURIComponent(actionId)}`);
  }
}
