import { Component, OnDestroy, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { Subscription } from 'rxjs';
import { ActivityLogService } from '../../+state/services/activity-log.service';
import { UserbaseService } from '../../+state/services/userbase.service';
import { AdminActivityLog } from '../../+state/models/activity-log.model';
import {
  UserbaseImportDetailResponse,
  UserbaseImportJob
} from '../../+state/models/userbase.model';
import {
  BatchJob,
  BatchOptinRequest,
  RenewalWorkerStatus,
  SubscriptionOpsService
} from '../../+state/services/subscription-ops.service';

@Component({
  selector: 'app-operations-dashboard',
  templateUrl: './operations-dashboard.component.html',
  styleUrls: ['./operations-dashboard.component.scss']
})
export class OperationsDashboardComponent implements OnInit, OnDestroy {
  trackById = (_: number, row: any) => row?.id ?? _;
  trackByRowNumber = (_: number, row: any) => row?.row_number ?? _;

  // ── Activity logs ────────────────────────────────────────────────────────
  logs: AdminActivityLog[] = [];
  logsLoading = false;
  logFilters = { entity_type: '', action: '', actor: '', from: '', to: '' };
  logsPage = 1;
  logsPageSize = 20;
  logsTotal = 0;

  // ── Import history ───────────────────────────────────────────────────────
  imports: UserbaseImportJob[] = [];
  importsLoading = false;
  importsPage = 1;
  importsPageSize = 20;
  importsTotal = 0;
  selectedImport: UserbaseImportDetailResponse | null = null;
  importDetailLoading = false;

  // ── Batch optin ──────────────────────────────────────────────────────────
  batchForm: BatchOptinRequest = {
    telco: '',
    count: 100,
    entry_channel: 'API',
    product_ids: [],
    msisdns: []
  };
  batchProductIdsRaw = '';   // comma-separated input
  batchMsisdnsRaw = '';      // newline-separated input
  batchJobId: string | null = null;
  batchJob: BatchJob | null = null;
  batchLoading = false;
  private batchPollSub: Subscription | null = null;

  // ── Renewal worker ───────────────────────────────────────────────────────
  renewalStatus: RenewalWorkerStatus | null = null;
  renewalLoading = false;
  private renewalPollSub: Subscription | null = null;

  constructor(
    private activityLogService: ActivityLogService,
    private userbaseService: UserbaseService,
    private opsService: SubscriptionOpsService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.loadLogs();
    this.loadImports();
    this.loadRenewalStatus();
  }

  ngOnDestroy(): void {
    this.batchPollSub?.unsubscribe();
    this.renewalPollSub?.unsubscribe();
  }

  // ── Activity log methods ─────────────────────────────────────────────────
  loadLogs(): void {
    this.logsLoading = true;
    this.activityLogService.list({
      page: this.logsPage,
      page_size: this.logsPageSize,
      entity_type: this.logFilters.entity_type || undefined,
      action: this.logFilters.action || undefined,
      actor: this.logFilters.actor || undefined,
      from: this.logFilters.from || undefined,
      to: this.logFilters.to || undefined
    }).subscribe({
      next: (res) => { this.logs = res.items || []; this.logsTotal = res.total_count || 0; this.logsLoading = false; },
      error: () => { this.logsLoading = false; this.toast('Failed to load activity logs'); }
    });
  }

  applyLogFilters(): void { this.logsPage = 1; this.loadLogs(); }

  clearLogFilters(): void {
    this.logFilters = { entity_type: '', action: '', actor: '', from: '', to: '' };
    this.logsPage = 1;
    this.loadLogs();
  }

  onLogPageChange(event: PageEvent): void {
    this.logsPage = event.pageIndex + 1;
    this.logsPageSize = event.pageSize;
    this.loadLogs();
  }

  // ── Import history methods ───────────────────────────────────────────────
  loadImports(): void {
    this.importsLoading = true;
    this.userbaseService.listImports(this.importsPage, this.importsPageSize).subscribe({
      next: (res) => { this.imports = res.jobs || []; this.importsTotal = res.total_count || 0; this.importsLoading = false; },
      error: () => { this.importsLoading = false; this.toast('Failed to load import history'); }
    });
  }

  onImportPageChange(event: PageEvent): void {
    this.importsPage = event.pageIndex + 1;
    this.importsPageSize = event.pageSize;
    this.loadImports();
  }

  openImport(job: UserbaseImportJob): void {
    this.importDetailLoading = true;
    this.userbaseService.getImport(job.id).subscribe({
      next: (res) => { this.selectedImport = res; this.importDetailLoading = false; },
      error: () => { this.importDetailLoading = false; this.toast('Failed to load import detail'); }
    });
  }

  // ── Batch optin methods ──────────────────────────────────────────────────
  get batchRunning(): boolean {
    return this.batchJob?.state === 'pending' || this.batchJob?.state === 'running';
  }

  get batchTerminal(): boolean {
    const s = this.batchJob?.state;
    return s === 'completed' || s === 'failed' || s === 'cancelled';
  }

  triggerBatch(): void {
    this.batchPollSub?.unsubscribe();
    this.batchJobId = null;
    this.batchJob = null;
    this.batchLoading = true;

    const req: BatchOptinRequest = {
      ...this.batchForm,
      product_ids: this.batchProductIdsRaw
        .split(',')
        .map(s => s.trim())
        .filter(Boolean),
      msisdns: this.batchMsisdnsRaw
        .split('\n')
        .map(s => s.trim())
        .filter(Boolean)
    };
    // tenant_key/channel_key are injected by X-Tenant-Key header via interceptor;
    // we still pass them in the body for server-side validation fallback.

    this.opsService.triggerBatchOptin(req).subscribe({
      next: (res) => {
        this.batchJobId = res.jobId;
        this.batchLoading = false;
        this.toast(`Batch job started: ${res.jobId}`);
        this.startBatchPoll(res.jobId);
      },
      error: (err) => {
        this.batchLoading = false;
        this.toast('Failed to start batch job: ' + (err?.error?.error ?? err?.message ?? 'unknown'));
      }
    });
  }

  private startBatchPoll(jobId: string): void {
    this.batchPollSub = this.opsService.pollBatchProgress(jobId).subscribe({
      next: (job) => { this.batchJob = job; },
      error: () => { this.toast('Lost contact with batch job; check manually.'); }
    });
  }

  stopBatch(): void {
    if (!this.batchJobId) { return; }
    this.opsService.stopBatch(this.batchJobId, 'operator stop').subscribe({
      next: () => { this.toast('Stop signal sent.'); this.batchPollSub?.unsubscribe(); if (this.batchJob) { this.batchJob.state = 'cancelled'; } },
      error: () => { this.toast('Failed to stop batch job.'); }
    });
  }

  // ── Renewal worker methods ───────────────────────────────────────────────
  loadRenewalStatus(): void {
    this.renewalLoading = true;
    this.opsService.getRenewalWorkerStatus().subscribe({
      next: (s) => { this.renewalStatus = s; this.renewalLoading = false; },
      error: () => { this.renewalLoading = false; }
    });
  }

  startRenewal(): void {
    this.renewalLoading = true;
    this.opsService.startRenewalWorker().subscribe({
      next: () => { this.toast('Renewal worker started.'); this.loadRenewalStatus(); },
      error: (err) => { this.renewalLoading = false; this.toast('Failed to start: ' + (err?.error ?? err?.message ?? 'unknown')); }
    });
  }

  stopRenewal(): void {
    this.renewalLoading = true;
    this.opsService.stopRenewalWorker().subscribe({
      next: () => { this.toast('Renewal worker stopped.'); this.loadRenewalStatus(); },
      error: (err) => { this.renewalLoading = false; this.toast('Failed to stop: ' + (err?.error ?? err?.message ?? 'unknown')); }
    });
  }

  private toast(message: string): void {
    this.snackBar.open(message, 'Close', { duration: 4000 });
  }
}
