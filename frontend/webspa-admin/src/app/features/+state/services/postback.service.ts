// postback.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { map, Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  BulkRequeueResponse,
  PostbackLookupResponse,
  PostbackStats,
  PostbackStatusApiResponse,
  PostbackStatusResponse
} from '../models/postback.model';

@Injectable({
  providedIn: 'root'
})
export class PostbackService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/postbacks`;

  constructor(private http: HttpClient) {}

  /**
   * Look up postbacks for a transaction by transaction_id
   */
  getPostbacksByTransactionId(transactionId: string): Observable<PostbackLookupResponse> {
    const params = new HttpParams().set('transaction_id', transactionId);
    return this.http.get<PostbackLookupResponse>(this.baseUrl, { params });
  }

  /**
   * Get aggregate postback stats (pending, processing, success, failed, dlq, total)
   */
  getStats(): Observable<PostbackStats> {
    return this.http.get<PostbackStats>(`${this.baseUrl}/stats`);
  }

  /**
   * Get postbacks filtered by status
   */
  getByStatus(status: string, limit: number = 50, offset: number = 0): Observable<PostbackStatusResponse> {
    const params = new HttpParams()
      .set('limit', limit.toString())
      .set('offset', offset.toString());
    return this.http.get<PostbackStatusApiResponse>(`${this.baseUrl}/status/${status}`, { params }).pipe(
      map((response) => ({
        status: response.status,
        count: response.count,
        limit: response.limit,
        offset: response.offset,
        postbacks: response.postbacks ?? response.items ?? []
      }))
    );
  }

  /**
   * Retry a failed/DLQ postback by resetting it to PENDING
   */
  retryPostback(id: string): Observable<any> {
    return this.http.post(`${this.baseUrl}/${id}/retry`, {});
  }

  /**
   * Bulk requeue all DLQ postbacks
   */
  bulkRequeueDlq(limit: number = 100, offset: number = 0): Observable<BulkRequeueResponse> {
    const params = new HttpParams()
      .set('limit', limit.toString())
      .set('offset', offset.toString());
    return this.http.post<BulkRequeueResponse>(`${this.baseUrl}/requeue-dlq`, {}, { params });
  }
}
