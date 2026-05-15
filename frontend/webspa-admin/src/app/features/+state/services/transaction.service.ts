// transaction.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import { 
  TransactionListResponse, 
  TransactionListFilter,
  TransactionDetail,
  TransactionSummary,
  TransactionStats
} from '../models/transaction.model';

@Injectable({
  providedIn: 'root'
})
export class TransactionService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/transactions`;

  constructor(private http: HttpClient) {}

  /**
   * List transactions with optional filters
   */
  getTransactions(filter?: TransactionListFilter): Observable<TransactionListResponse> {
    let params = new HttpParams();
    
    if (filter) {
      if (filter.campaign_slug) {
        params = params.set('campaign_slug', filter.campaign_slug);
      }
      if (filter.status) {
        params = params.set('status', filter.status);
      }
      if (filter.provider) {
        params = params.set('provider', filter.provider);
      }
      if (filter.start_date) {
        params = params.set('start_date', filter.start_date);
      }
      if (filter.end_date) {
        params = params.set('end_date', filter.end_date);
      }
      if (filter.sort_by) {
        params = params.set('sort_by', filter.sort_by);
      }
      if (filter.sort_dir) {
        params = params.set('sort_dir', filter.sort_dir);
      }
      if (filter.page) {
        params = params.set('page', filter.page.toString());
      }
      if (filter.page_size) {
        params = params.set('page_size', filter.page_size.toString());
      }
    }

    return this.http.get<TransactionListResponse>(this.baseUrl, { params });
  }

  /**
   * Get a single transaction by ID
   */
  getTransactionById(id: string): Observable<TransactionDetail> {
    return this.http.get<TransactionDetail>(`${this.baseUrl}/${id}`);
  }

  /**
   * Trigger a postback for a transaction
   */
  triggerPostback(transactionId: string, event: string = 'conversion'): Observable<any> {
    const params = new HttpParams().set('event', event);
    return this.http.post(`${this.baseUrl}/${transactionId}/trigger-postback`, {}, { params });
  }

  /**
   * Get transaction stats
   */
  getTransactionStats(startDate?: string, endDate?: string): Observable<TransactionStats> {
    let params = new HttpParams();
    if (startDate) {
      params = params.set('start_date', startDate);
    }
    if (endDate) {
      params = params.set('end_date', endDate);
    }
    return this.http.get<TransactionStats>(`${this.baseUrl}/stats`, { params });
  }
}
