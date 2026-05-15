// postback-lookup.component.ts
import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { MatTableDataSource } from '@angular/material/table';
import { MatPaginator, PageEvent } from '@angular/material/paginator';
import { MatSort } from '@angular/material/sort';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PostbackService } from '../../+state/services/postback.service';
import {
  PostbackOutbox,
  PostbackAttempt,
  PostbackLookupResponse,
  PostbackStats,
  PostbackStatus
} from '../../+state/models/postback.model';
import { extractHttpErrorMessage } from '../../../core/utils/http-error-message';

@Component({
  selector: 'app-postback-lookup',
  templateUrl: './postback-lookup.component.html',
  styleUrls: ['./postback-lookup.component.scss']
})
export class PostbackLookupComponent implements OnInit {
  transactionId: string = '';
  loading: boolean = false;
  searched: boolean = false;
  lookupResult: PostbackLookupResponse | null = null;

  // Stats dashboard
  stats: PostbackStats | null = null;

  // Status management
  selectedStatus: PostbackStatus = 'DLQ';
  statusOptions: PostbackStatus[] = ['DLQ', 'FAILED', 'PENDING', 'PROCESSING', 'SUCCESS'];
  statusLimit: number = 100;
  statusLimitOptions: number[] = [25, 50, 100, 250];
  trackById = (_: number, row: any) => row?.id ?? _;
  trackByAttempt = (_: number, row: any) => row?.id ?? row?.attempt_number ?? _;
  statusColumns: string[] = ['event', 'provider', 'transaction_id', 'next_retry_at', 'url', 'attempt_count', 'created_at', 'actions'];
  statusDataSource = new MatTableDataSource<PostbackOutbox>([]);
  statusCount: number = 0;
  statusPageIndex: number = 0;
  loadingStatus: boolean = false;
  bulkRequeueLoading: boolean = false;
  retryingPostbackIds = new Set<string>();

  // Postback outbox table
  postbackColumns: string[] = [
    'event',
    'provider', 
    'status',
    'attempt_count',
    'next_retry_at',
    'url_template_rendered',
    'created_at',
    'updated_at',
    'actions'
  ];
  postbackDataSource = new MatTableDataSource<PostbackOutbox>([]);

  // Attempts table
  attemptColumns: string[] = [
    'attempt_number',
    'http_status',
    'duration_ms',
    'error_message',
    'response_body',
    'created_at'
  ];
  attemptDataSource = new MatTableDataSource<PostbackAttempt>([]);

  @ViewChild('postbackPaginator') postbackPaginator!: MatPaginator;
  @ViewChild('attemptPaginator') attemptPaginator!: MatPaginator;
  @ViewChild(MatSort) sort!: MatSort;

  constructor(
    private postbackService: PostbackService,
    private snackBar: MatSnackBar,
    private route: ActivatedRoute
  ) {}

  ngOnInit(): void {
    this.loadStats();
    this.loadStatusPostbacks();

    // Check for transaction_id in query params (from transaction list link)
    this.route.queryParams.subscribe(params => {
      if (params['transaction_id']) {
        this.transactionId = params['transaction_id'];
        this.searchPostbacks();
      }
    });
  }

  ngAfterViewInit(): void {
    this.postbackDataSource.paginator = this.postbackPaginator;
    this.attemptDataSource.paginator = this.attemptPaginator;
    this.postbackDataSource.sort = this.sort;
  }

  searchPostbacks(): void {
    if (!this.transactionId.trim()) {
      this.snackBar.open('Please enter a Transaction ID', 'Close', {
        duration: 3000,
        panelClass: ['error-snackbar']
      });
      return;
    }

    this.loading = true;
    this.searched = true;
    this.lookupResult = null;

    this.postbackService.getPostbacksByTransactionId(this.transactionId.trim())
      .subscribe({
        next: (response) => {
          this.lookupResult = response;
          this.postbackDataSource.data = response.postbacks || [];
          this.attemptDataSource.data = response.attempts || [];
          this.loading = false;

          if (!response.postbacks?.length) {
            this.snackBar.open('No postbacks found for this transaction', 'Close', {
              duration: 3000
            });
          }
        },
        error: (error) => {
          this.loading = false;
          const message = extractHttpErrorMessage(error, 'Failed to fetch postbacks');
          this.snackBar.open(message, 'Close', {
            duration: 5000,
            panelClass: ['error-snackbar']
          });
        }
      });
  }

  clearSearch(): void {
    this.transactionId = '';
    this.searched = false;
    this.lookupResult = null;
    this.postbackDataSource.data = [];
    this.attemptDataSource.data = [];
  }

  getStatusClass(status: string): string {
    switch (status) {
      case 'SUCCESS': return 'status-success';
      case 'PENDING': return 'status-pending';
      case 'PROCESSING': return 'status-processing';
      case 'FAILED': return 'status-failed';
      case 'DLQ': return 'status-dlq';
      default: return '';
    }
  }

  getEventClass(event: string): string {
    switch (event) {
      case 'conversion': return 'event-conversion';
      case 'subscribed': return 'event-subscribed';
      case 'failed': return 'event-failed';
      case 'cancelled': return 'event-cancelled';
      default: return '';
    }
  }

  formatUrl(url: string): string {
    // Truncate long URLs for display
    if (url && url.length > 80) {
      return url.substring(0, 80) + '...';
    }
    return url;
  }

  formatTransactionId(transactionId: string): string {
    if (!transactionId) {
      return '-';
    }

    return transactionId.length > 18
      ? `${transactionId.slice(0, 8)}...${transactionId.slice(-6)}`
      : transactionId;
  }

  copyToClipboard(text: string): void {
    if (!navigator.clipboard) {
      this.snackBar.open('Clipboard access is not available in this browser context', 'Close', {
        duration: 3000,
        panelClass: ['error-snackbar']
      });
      return;
    }

    navigator.clipboard.writeText(text).then(() => {
      this.snackBar.open('Copied to clipboard', 'Close', {
        duration: 2000
      });
    }).catch(() => {
      this.snackBar.open('Failed to copy to clipboard', 'Close', {
        duration: 3000,
        panelClass: ['error-snackbar']
      });
    });
  }

  retryPostback(id: string): void {
    if (this.retryingPostbackIds.has(id)) {
      return;
    }

    this.retryingPostbackIds.add(id);
    const request = this.postbackService.retryPostback(id).subscribe({
      next: () => {
        this.snackBar.open('Postback requeued for retry', 'Close', { duration: 3000 });
        this.loadStats();
        this.loadStatusPostbacks();
        if (this.transactionId) {
          this.searchPostbacks();
        }
      },
      error: (err: any) => {
        this.snackBar.open(
          extractHttpErrorMessage(err, 'Failed to retry postback'),
          'Close',
          { duration: 5000, panelClass: ['error-snackbar'] }
        );
      }
    });
    request.add(() => {
      this.retryingPostbackIds.delete(id);
    });
  }

  loadStats(): void {
    this.postbackService.getStats().subscribe({
      next: (stats) => {
        this.stats = stats;
      },
      error: (err) => {
        this.snackBar.open(
          extractHttpErrorMessage(err, 'Failed to load postback stats'),
          'Close',
          {
            duration: 5000,
            panelClass: ['error-snackbar']
          }
        );
      }
    });
  }

  loadStatusPostbacks(): void {
    this.loadingStatus = true;
    this.postbackService.getByStatus(this.selectedStatus, this.statusLimit, this.getStatusOffset()).subscribe({
      next: (response) => {
        const maxPageIndex = response.count ? Math.max(Math.ceil(response.count / this.statusLimit) - 1, 0) : 0;
        if (this.statusPageIndex > maxPageIndex) {
          this.statusPageIndex = maxPageIndex;
          this.loadStatusPostbacks();
          return;
        }
        this.statusDataSource.data = response.postbacks || [];
        this.statusCount = response.count ?? this.statusDataSource.data.length;
        this.loadingStatus = false;
      },
      error: (err) => {
        this.loadingStatus = false;
        this.snackBar.open(
          extractHttpErrorMessage(err, `Failed to load ${this.selectedStatus} postbacks`),
          'Close',
          {
            duration: 5000,
            panelClass: ['error-snackbar']
          }
        );
      }
    });
  }

  onStatusChange(): void {
    this.statusPageIndex = 0;
    this.loadStatusPostbacks();
  }

  canRetry(status: PostbackStatus): boolean {
    return status === 'FAILED' || status === 'DLQ';
  }

  canBulkRequeue(): boolean {
    return this.selectedStatus === 'DLQ' && this.statusDataSource.data.length > 0 && !this.bulkRequeueLoading;
  }

  getStatusSectionTitle(): string {
    return this.selectedStatus === 'DLQ' ? 'DLQ Management' : `${this.selectedStatus} Postbacks`;
  }

  getStatusSectionDescription(): string {
    return this.selectedStatus === 'DLQ'
      ? 'Review dead-letter postbacks, retry individual items, or bulk requeue them back to PENDING.'
      : `Inspect ${this.selectedStatus.toLowerCase()} postbacks and retry individual FAILED items when supported.`;
  }

  getEmptyStatusTitle(): string {
    return this.selectedStatus === 'DLQ'
      ? 'No postbacks in the Dead Letter Queue'
      : `No ${this.selectedStatus.toLowerCase()} postbacks found`;
  }

  getEmptyStatusHint(): string {
    switch (this.selectedStatus) {
      case 'DLQ':
        return 'DLQ entries appear when postbacks exhaust all retry attempts.';
      case 'FAILED':
        return 'FAILED entries are individual postbacks that can still be manually retried.';
      case 'PENDING':
        return 'PENDING entries are waiting for the dispatcher to process them.';
      case 'PROCESSING':
        return 'PROCESSING entries are currently being worked by the dispatcher.';
      case 'SUCCESS':
        return 'SUCCESS entries have already been delivered successfully.';
      default:
        return 'No postbacks match the selected status.';
    }
  }

  refreshStatusPostbacks(): void {
    this.loadStatusPostbacks();
  }

  onStatusLimitChange(): void {
    this.statusPageIndex = 0;
    this.loadStatusPostbacks();
  }

  onStatusPageChange(event: PageEvent): void {
    this.statusPageIndex = event.pageIndex;
    this.statusLimit = event.pageSize;
    this.loadStatusPostbacks();
  }

  formatRetryAt(nextRetryAt?: string): string {
    return nextRetryAt ? new Date(nextRetryAt).toLocaleString() : '-';
  }

  inspectTransactionPostbacks(transactionId: string): void {
    this.transactionId = transactionId;
    this.searchPostbacks();
  }

  isRetrying(id: string): boolean {
    return this.retryingPostbackIds.has(id);
  }

  private getStatusOffset(): number {
    return this.statusPageIndex * this.statusLimit;
  }

  bulkRequeueDlq(): void {
    if (this.selectedStatus !== 'DLQ') {
      this.snackBar.open('Bulk requeue is only available for DLQ postbacks', 'Close', { duration: 3000 });
      return;
    }

    const listedCount = this.statusDataSource.data.length;
    if (listedCount === 0) {
      return;
    }

    if (!window.confirm(`Requeue ${listedCount} listed DLQ postback(s) on this page back to PENDING?`)) {
      return;
    }

    this.bulkRequeueLoading = true;
    const request = this.postbackService.bulkRequeueDlq(this.statusLimit, this.getStatusOffset()).subscribe({
      next: (response) => {
        const requeued = response.requeued ?? 0;
        this.snackBar.open(
          requeued > 0 ? `Requeued ${requeued} DLQ postback(s)` : 'No DLQ postbacks were requeued',
          'Close',
          { duration: 3000 }
        );
        this.loadStats();
        this.loadStatusPostbacks();
        if (this.transactionId) {
          this.searchPostbacks();
        }
      },
      error: (err: any) => {
        this.snackBar.open(
          extractHttpErrorMessage(err, 'Failed to requeue DLQ postbacks'),
          'Close',
          { duration: 5000, panelClass: ['error-snackbar'] }
        );
      }
    });
    request.add(() => {
      this.bulkRequeueLoading = false;
    });
  }
}
