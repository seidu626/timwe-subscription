import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { interval, Observable, Subscription } from 'rxjs';
import { switchMap, takeWhile } from 'rxjs/operators';
import { environment } from 'src/environments/environment';

export interface BatchJob {
  id: string;
  state: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  total: number;
  processed: number;
  successful: number;
  failed: number;
  tenantKey?: string;
  channelKey?: string;
  errorDetails?: Record<string, unknown>;
  startedAt: string;
  completedAt?: string;
}

export interface BatchOptinRequest {
  telco?: string;
  count?: number;
  entry_channel?: string;
  msisdns?: string[];
  product_ids?: string[];
  tenant_key?: string;
  channel_key?: string;
}

export interface RenewalWorkerStatus {
  running: boolean;
  metrics?: Record<string, unknown>;
}

// ── Subscription health (GET /v1/admin/reports/subscription-health) ────────

export type ProviderFeedState = 'fresh' | 'stale' | 'unavailable';

export interface LocalSubscriptionCounts {
  active: number;
  inactive: number;
  other: number;
  total: number;
}

/** One provider report row: a day × source (shortcode) × product × pricepoint. */
export interface ProviderDailyBilling {
  date: string;                   // YYYY-MM-DD in the response timezone
  shortcode: string;
  product_id: number;
  product_name: string;
  pricepoint_id: number;
  success_billings: number;
  revenue: string;                // decimal string, never parsed to float
}

export interface ProviderBillingSummary {
  state: ProviderFeedState;
  last_imported_at: string | null;
  success_billings: number | null; // null when state is 'unavailable'
  revenue: string | null;          // decimal string; null when unavailable
  currency: string | null;         // null unless confirmed by source config
  daily: ProviderDailyBilling[];
}

export interface SubscriptionHealthResponse {
  as_of: string;
  timezone: string;               // IANA zone the report dates are expressed in, e.g. 'Africa/Accra'
  subscriptions: LocalSubscriptionCounts;
  provider: ProviderBillingSummary;
}

/** Must match the API: the range may contain at most this many inclusive days. */
export const SUBSCRIPTION_HEALTH_MAX_RANGE_DAYS = 31;
/** Must match the API's shortcode length limit. */
export const SUBSCRIPTION_HEALTH_MAX_SHORTCODE_LENGTH = 50;
/** Zone used before the first response reports one; matches the API default. */
export const SUBSCRIPTION_HEALTH_DEFAULT_TIMEZONE = 'Africa/Accra';

export interface SubscriptionHealthQuery {
  startDate: string;              // inclusive, YYYY-MM-DD
  endDate: string;                // inclusive, YYYY-MM-DD
  productId?: number;
  shortcode?: string;
}

@Injectable({ providedIn: 'root' })
export class SubscriptionOpsService {
  private readonly extBase = environment.subscriptionExternalAdminApiEndpoint + '/api/v1/subscription-external';
  private readonly renewalBase = environment.subscriptionExternalAdminApiEndpoint + '/api/v1/renewal';
  private readonly reportsBase = `${environment.acquisitionApiEndpoint}/v1/admin/reports`;

  constructor(private http: HttpClient) {}

  getSubscriptionHealth(query: SubscriptionHealthQuery): Observable<SubscriptionHealthResponse> {
    let params = new HttpParams()
      .set('startDate', query.startDate)
      .set('endDate', query.endDate);
    if (query.productId != null) {
      params = params.set('productId', String(query.productId));
    }
    if (query.shortcode) {
      params = params.set('shortcode', query.shortcode);
    }
    return this.http.get<SubscriptionHealthResponse>(`${this.reportsBase}/subscription-health`, { params });
  }

  triggerBatchOptin(req: BatchOptinRequest): Observable<{ jobId: string }> {
    return this.http.post<{ jobId: string }>(`${this.extBase}/batch`, req);
  }

  getBatchProgress(jobId: string): Observable<BatchJob> {
    return this.http.get<BatchJob>(`${this.extBase}/batch/progress`, {
      params: new HttpParams().set('batch_id', jobId),
    });
  }

  stopBatch(jobId: string, reason = ''): Observable<unknown> {
    return this.http.post(`${this.extBase}/batch/stop`, { batch_id: jobId, reason });
  }

  /** Poll progress every `intervalMs` ms until the job reaches a terminal state. */
  pollBatchProgress(jobId: string, intervalMs = 3000): Observable<BatchJob> {
    return interval(intervalMs).pipe(
      switchMap(() => this.getBatchProgress(jobId)),
      takeWhile(
        (job) => job.state === 'pending' || job.state === 'running',
        true, // emit the terminal state too
      ),
    );
  }

  getRenewalWorkerStatus(): Observable<RenewalWorkerStatus> {
    return this.http.get<RenewalWorkerStatus>(`${this.renewalBase}/worker/status`);
  }

  startRenewalWorker(): Observable<unknown> {
    return this.http.post(`${this.renewalBase}/worker/start`, {});
  }

  stopRenewalWorker(): Observable<unknown> {
    return this.http.post(`${this.renewalBase}/worker/stop`, {});
  }
}
