import { Component, OnDestroy, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { of, Subject, Subscription, timer } from 'rxjs';
import { catchError, filter, map, switchMap, takeUntil } from 'rxjs/operators';
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
  ProviderDailyBilling,
  ProviderFeedState,
  RenewalWorkerStatus,
  SUBSCRIPTION_HEALTH_DEFAULT_TIMEZONE,
  SUBSCRIPTION_HEALTH_MAX_RANGE_DAYS,
  SUBSCRIPTION_HEALTH_MAX_SHORTCODE_LENGTH,
  SubscriptionHealthQuery,
  SubscriptionHealthResponse,
  SubscriptionOpsService
} from '../../+state/services/subscription-ops.service';

type HealthLoadResult =
  | { ok: true; data: SubscriptionHealthResponse; query: SubscriptionHealthQuery }
  | { ok: false; message: string };

/** Dashboard re-query cadence. The provider report itself is only polled server-side (~15 min). */
const HEALTH_REFRESH_MS = 60_000;
const DAY_MS = 86_400_000;
const ISO_DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

const zoneFormatters = new Map<string, Intl.DateTimeFormat>();

/**
 * Calendar date of `instant` in an IANA zone as YYYY-MM-DD. Built from formatToParts so the
 * result never depends on the browser locale or its separator conventions.
 */
function isoDateInZone(instant: Date, timeZone: string): string {
  let fmt = zoneFormatters.get(timeZone);
  if (!fmt) {
    try {
      fmt = new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' });
    } catch {
      // Unknown zone from the API: fall back to the documented default rather than the browser zone.
      return isoDateInZone(instant, SUBSCRIPTION_HEALTH_DEFAULT_TIMEZONE);
    }
    zoneFormatters.set(timeZone, fmt);
  }
  const parts = fmt.formatToParts(instant);
  const part = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
  return `${part('year')}-${part('month')}-${part('day')}`;
}

/** Whole days since the epoch for a YYYY-MM-DD string, or null if it is not a real date. */
function isoDateToDayNumber(iso: string): number | null {
  if (!ISO_DATE_RE.test(iso)) { return null; }
  const [y, m, d] = iso.split('-').map(Number);
  const ms = Date.UTC(y, m - 1, d);
  const back = new Date(ms);
  if (back.getUTCFullYear() !== y || back.getUTCMonth() !== m - 1 || back.getUTCDate() !== d) { return null; }
  return ms / DAY_MS;
}

/** Pure calendar arithmetic on YYYY-MM-DD, independent of any time zone. */
function addIsoDays(iso: string, days: number): string {
  const n = isoDateToDayNumber(iso);
  return n === null ? iso : new Date((n + days) * DAY_MS).toISOString().slice(0, 10);
}

@Component({
  selector: 'app-operations-dashboard',
  templateUrl: './operations-dashboard.component.html',
  styleUrls: ['./operations-dashboard.component.scss']
})
export class OperationsDashboardComponent implements OnInit, OnDestroy {
  trackById = (_: number, row: any) => row?.id ?? _;
  trackByRowNumber = (_: number, row: any) => row?.row_number ?? _;
  // A daily row is unique per date × source × product × pricepoint, never per date alone.
  trackByDailyRow = (_: number, row: ProviderDailyBilling) =>
    `${row.date}|${row.shortcode}|${row.product_id}|${row.pricepoint_id}`;

  // ── Subscription health ──────────────────────────────────────────────────
  healthFilters: { startDate: string; endDate: string; shortcode: string; productId: number | null } = {
    startDate: '', endDate: '', shortcode: '', productId: null
  };
  health: SubscriptionHealthResponse | null = null;
  healthQuery: SubscriptionHealthQuery | null = null;   // scope of the figures on screen
  healthLoadedAt: Date | null = null;
  healthLoading = false;       // user-initiated load: shows spinner and dims figures
  healthRefreshing = false;    // background auto-refresh: figures stay fully visible
  healthError: string | null = null;
  healthValidationError: string | null = null;
  healthDimensions = { shortcodes: 0, products: 0, pricepoints: 0 };
  readonly healthRefreshSeconds = HEALTH_REFRESH_MS / 1000;
  readonly healthMaxRangeDays = SUBSCRIPTION_HEALTH_MAX_RANGE_DAYS;
  readonly healthDailyColumns = [
    'date', 'shortcode', 'product', 'pricepoint', 'success_billings', 'revenue', 'status'
  ];
  private readonly healthLoad$ = new Subject<SubscriptionHealthQuery>();
  private healthInFlight = false;
  // Last valid query the user asked for; auto-refresh repeats it (not unsubmitted form edits).
  private healthRequestedQuery: SubscriptionHealthQuery | null = null;
  private readonly destroy$ = new Subject<void>();

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
    this.initHealth();
    this.loadLogs();
    this.loadImports();
    this.loadRenewalStatus();
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
    this.batchPollSub?.unsubscribe();
    this.renewalPollSub?.unsubscribe();
  }

  // ── Subscription health methods ──────────────────────────────────────────
  private initHealth(): void {
    this.resetHealthDates();
    // switchMap cancels an in-flight request when a newer one is issued (manual load or
    // refresh), so at most one request is outstanding and a slow earlier response can never
    // overwrite a newer scope.
    this.healthLoad$.pipe(
      switchMap((query) =>this.opsService.getSubscriptionHealth(query).pipe(
        map((data): HealthLoadResult => ({ ok: true, data, query })),
        catchError((err) => of<HealthLoadResult>({
          ok: false,
          message: err?.error?.error ?? err?.error?.message ?? err?.message ?? 'Request failed'
        }))
      )),
      takeUntil(this.destroy$)
    ).subscribe((result) => {
      this.healthInFlight = false;
      this.healthLoading = false;
      this.healthRefreshing = false;
      if (result.ok) {
        this.health = result.data;
        this.healthQuery = result.query;
        this.healthLoadedAt = new Date();
        this.healthError = null;
        this.healthDimensions = this.countDimensions(result.data);
      } else {
        // Keep previous figures visible but flagged; never blank them silently.
        this.healthError = result.message;
      }
    });

    // Auto-refresh while mounted. Ticks are skipped while any request is still outstanding
    // (so requests never overlap) and while the tab is hidden; destroy$ tears the timer down.
    timer(HEALTH_REFRESH_MS, HEALTH_REFRESH_MS).pipe(
      filter(() => !this.healthInFlight && !!this.healthRequestedQuery && !this.documentHidden()),
      takeUntil(this.destroy$)
    ).subscribe(() => this.requestHealth(this.healthRequestedQuery!, true));

    this.loadHealth();
  }

  private requestHealth(query: SubscriptionHealthQuery, background: boolean): void {
    this.healthInFlight = true;
    if (background) {
      this.healthRefreshing = true;
    } else {
      this.healthLoading = true;
      this.healthRefreshing = false;
    }
    this.healthLoad$.next(query);
  }

  private documentHidden(): boolean {
    return typeof document !== 'undefined' && document.visibilityState === 'hidden';
  }

  private countDimensions(data: SubscriptionHealthResponse) {
    const rows = data.provider?.daily ?? [];
    return {
      shortcodes: new Set(rows.map((r) => r.shortcode)).size,
      products: new Set(rows.map((r) => r.product_id)).size,
      pricepoints: new Set(rows.map((r) => `${r.product_id}|${r.pricepoint_id}`)).size
    };
  }

  private resetHealthDates(): void {
    const end = this.healthToday;
    this.healthFilters.startDate = addIsoDays(end, -6);
    this.healthFilters.endDate = end;
  }

  loadHealth(): void {
    const f = this.healthFilters;
    const shortcode = (f.shortcode ?? '').trim();
    const productId = f.productId === null || (f.productId as unknown) === '' ? null : Number(f.productId);

    if (!f.startDate || !f.endDate) {
      this.healthValidationError = 'Choose both a start and an end date.';
      return;
    }
    const startDay = isoDateToDayNumber(f.startDate);
    const endDay = isoDateToDayNumber(f.endDate);
    if (startDay === null || endDay === null) {
      this.healthValidationError = 'Dates must be valid calendar dates (YYYY-MM-DD).';
      return;
    }
    if (startDay > endDay) {
      this.healthValidationError = 'Start date must be on or before the end date.';
      return;
    }
    if (endDay - startDay + 1 > SUBSCRIPTION_HEALTH_MAX_RANGE_DAYS) {
      this.healthValidationError =
        `Date range must contain at most ${SUBSCRIPTION_HEALTH_MAX_RANGE_DAYS} days, including both ends `
        + `(selected ${endDay - startDay + 1}).`;
      return;
    }
    if (shortcode.length > SUBSCRIPTION_HEALTH_MAX_SHORTCODE_LENGTH) {
      this.healthValidationError = `Shortcode must be at most ${SUBSCRIPTION_HEALTH_MAX_SHORTCODE_LENGTH} characters.`;
      return;
    }
    if (productId !== null && (!Number.isInteger(productId) || productId <= 0)) {
      this.healthValidationError = 'Product ID must be a positive whole number.';
      return;
    }
    this.healthValidationError = null;
    this.healthRequestedQuery = {
      startDate: f.startDate,
      endDate: f.endDate,
      shortcode: shortcode || undefined,
      productId: productId ?? undefined
    };
    this.requestHealth(this.healthRequestedQuery, false);
  }

  clearHealthFilters(): void {
    this.healthFilters = { startDate: '', endDate: '', shortcode: '', productId: null };
    this.resetHealthDates();
    this.loadHealth();
  }

  /** IANA zone the report's dates are expressed in (from the API, else its documented default). */
  get healthTimezone(): string {
    return this.health?.timezone || SUBSCRIPTION_HEALTH_DEFAULT_TIMEZONE;
  }

  /** "Today" in the report timezone, not the browser's zone or locale. */
  get healthToday(): string {
    return isoDateInZone(new Date(), this.healthTimezone);
  }

  get providerState(): ProviderFeedState | null {
    return this.health?.provider?.state ?? null;
  }

  /** Current-day (or later) provider rows are partial and may be revised by the next import. */
  isProvisional(row: ProviderDailyBilling): boolean {
    return row.date >= this.healthToday;
  }

  get healthRangeIncludesToday(): boolean {
    return !!this.healthQuery && this.healthQuery.endDate >= this.healthToday
      && this.healthQuery.startDate <= this.healthToday;
  }

  get healthScopeLabel(): string {
    const q = this.healthQuery;
    if (!q) { return ''; }
    const parts = [q.shortcode ? `shortcode ${q.shortcode}` : 'all shortcodes',
      q.productId != null ? `product ${q.productId}` : 'all products',
      `dates in ${this.healthTimezone}`];
    return parts.join(' · ');
  }

  providerStateLabel(state: ProviderFeedState | null): string {
    switch (state) {
      case 'fresh': return 'Recent import';
      case 'stale': return 'Stale';
      case 'unavailable': return 'Unavailable';
      default: return 'Unknown';
    }
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
