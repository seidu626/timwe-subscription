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

@Injectable({ providedIn: 'root' })
export class SubscriptionOpsService {
  private readonly extBase = environment.subscriptionExternalAdminApiEndpoint + '/api/v1/subscription-external';
  private readonly renewalBase = environment.subscriptionExternalAdminApiEndpoint + '/api/v1/renewal';

  constructor(private http: HttpClient) {}

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
