// transaction-list.component.ts
import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { MatTableDataSource } from '@angular/material/table';
import { PageEvent } from '@angular/material/paginator';
import { Sort } from '@angular/material/sort';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatDialog } from '@angular/material/dialog';
import { TransactionService } from '../../+state/services/transaction.service';
import { CampaignService } from '../../+state/services/campaign.service';
import {
  TransactionDetail,
  TransactionSummary,
  TransactionListFilter,
  TransactionStats
} from '../../+state/models/transaction.model';
import { Campaign } from '../../+state/models/campaign.model';
import { extractHttpErrorMessage } from '../../../core/utils/http-error-message';
import { ErrorDialogComponent } from '../../../shared/error-dialog/error-dialog.component';

@Component({
  selector: 'app-transaction-list',
  templateUrl: './transaction-list.component.html',
  styleUrls: ['./transaction-list.component.scss']
})
export class TransactionListComponent implements OnInit {
  loading: boolean = false;
  statsLoading: boolean = false;

  // Filter state
  filters: TransactionListFilter = this.createDefaultFilters();
  campaigns: Campaign[] = [];
  statuses: string[] = [
    'PENDING', 'ACTION_REQUIRED', 'CONFIRM_REQUIRED', 
    'SUBSCRIBED', 'CHARGED', 'FAILED', 'CANCELLED'
  ];
  providers: string[] = ['mobplus', 'generic'];

  // Stats
  stats: TransactionStats | null = null;
  selectedTransactionId: string | null = null;
  selectedTransaction: TransactionDetail | null = null;
  transactionDetailLoading: boolean = false;
  transactionDetailError: string | null = null;
  private transactionDetailRequestToken: number = 0;
  triggeringPostbackIds = new Set<string>();
  manuallyTriggeredTransactionIds = new Set<string>();

  // Table
  displayedColumns: string[] = [
    'created_at',
    'campaign_slug',
    'msisdn',
    'status',
    'ad_provider',
    'click_id',
    'conversion_postback_sent',
    'actions'
  ];
  dataSource = new MatTableDataSource<TransactionSummary>([]);
  totalCount: number = 0;
  pageSizes: number[] = [10, 20, 50, 100];

  trackById = (_: number, row: TransactionSummary) => row?.id ?? _;

  constructor(
    private transactionService: TransactionService,
    private campaignService: CampaignService,
    private router: Router,
    private snackBar: MatSnackBar,
    private dialog: MatDialog
  ) {}

  ngOnInit(): void {
    this.loadCampaigns();
    this.loadStats();
    this.loadTransactions();
  }

  loadCampaigns(): void {
    this.campaignService.getCampaigns().subscribe({
      next: (campaigns) => {
        this.campaigns = campaigns;
      },
      error: (err) => {
        this.snackBar.open(
          extractHttpErrorMessage(err, 'Failed to load campaign filters'),
          'Close',
          {
            duration: 5000,
            panelClass: ['error-snackbar']
          }
        );
      }
    });
  }

  loadStats(): void {
    this.statsLoading = true;
    const startDate = this.filters.start_date || this.getDefaultStartDate();
    const endDate = this.filters.end_date || this.getDefaultEndDate();
    
    this.transactionService.getTransactionStats(startDate, endDate).subscribe({
      next: (stats) => {
        this.stats = stats;
        this.statsLoading = false;
      },
      error: (err) => {
        this.stats = null;
        this.statsLoading = false;
        this.snackBar.open(
          extractHttpErrorMessage(err, 'Failed to load transaction stats'),
          'Close',
          {
            duration: 5000,
            panelClass: ['error-snackbar']
          }
        );
      }
    });
  }

  loadTransactions(): void {
    this.loading = true;
    this.transactionService.getTransactions(this.filters).subscribe({
      next: (response) => {
        this.dataSource.data = response.transactions;
        this.totalCount = response.total_count;
        const visibleIds = new Set(response.transactions.map((tx) => tx.id));
        for (const id of Array.from(this.manuallyTriggeredTransactionIds)) {
          if (!visibleIds.has(id)) {
            this.manuallyTriggeredTransactionIds.delete(id);
          }
        }
        if (this.selectedTransactionId && !response.transactions.some((tx) => tx.id === this.selectedTransactionId)) {
          this.selectedTransactionId = null;
          this.selectedTransaction = null;
          this.transactionDetailError = null;
        }
        this.loading = false;
      },
      error: (err) => {
        this.dataSource.data = [];
        this.totalCount = 0;
        this.selectedTransactionId = null;
        this.selectedTransaction = null;
        this.transactionDetailError = null;
        this.loading = false;
        this.snackBar.open(
          extractHttpErrorMessage(err, 'Failed to load transactions'),
          'Close',
          {
            duration: 5000,
            panelClass: ['error-snackbar']
          }
        );
      }
    });
  }

  applyFilters(): void {
    this.filters.page = 1;
    this.loadTransactions();
    this.loadStats();
  }

  clearFilters(): void {
    this.filters = this.createDefaultFilters(this.filters.page_size);
    this.loadTransactions();
    this.loadStats();
  }

  onPageChange(event: PageEvent): void {
    this.filters.page = event.pageIndex + 1;
    this.filters.page_size = event.pageSize;
    this.loadTransactions();
  }

  onSortChange(event: Sort): void {
    this.filters.sort_by = event.active || 'created_at';
    this.filters.sort_dir = (event.direction || 'desc') as 'asc' | 'desc';
    this.filters.page = 1;
    this.loadTransactions();
  }

  toggleTransactionDetails(tx: TransactionSummary): void {
    if (this.selectedTransactionId === tx.id && !this.transactionDetailLoading) {
      this.selectedTransactionId = null;
      this.selectedTransaction = null;
      this.transactionDetailError = null;
      return;
    }

    this.selectedTransactionId = tx.id;
    this.selectedTransaction = null;
    this.transactionDetailError = null;
    this.transactionDetailLoading = true;
    const requestToken = ++this.transactionDetailRequestToken;

    this.transactionService.getTransactionById(tx.id).subscribe({
      next: (detail) => {
        if (requestToken !== this.transactionDetailRequestToken || this.selectedTransactionId !== tx.id) {
          return;
        }
        this.selectedTransaction = detail;
        this.transactionDetailLoading = false;
      },
      error: (err) => {
        if (requestToken !== this.transactionDetailRequestToken || this.selectedTransactionId !== tx.id) {
          return;
        }
        this.transactionDetailLoading = false;
        this.transactionDetailError = extractHttpErrorMessage(err, 'Failed to load transaction details');
        this.snackBar.open(this.transactionDetailError, 'Close', {
          duration: 5000,
          panelClass: ['error-snackbar']
        });
      }
    });
  }

  viewPostbacks(transactionId: string): void {
    this.router.navigate(['/postback'], { 
      queryParams: { transaction_id: transactionId } 
    });
  }

  getStatusClass(status: string): string {
    switch (status) {
      case 'CHARGED': return 'status-charged';
      case 'SUBSCRIBED': return 'status-subscribed';
      case 'PENDING': 
      case 'ACTION_REQUIRED':
      case 'CONFIRM_REQUIRED':
        return 'status-pending';
      case 'FAILED': return 'status-failed';
      case 'CANCELLED': return 'status-cancelled';
      default: return '';
    }
  }

  getStatusCount(status: string): number {
    return this.stats?.status_counts?.[status] || 0;
  }

  private getDefaultStartDate(): string {
    const date = new Date();
    date.setDate(date.getDate() - 7);
    return date.toISOString().split('T')[0];
  }

  private getDefaultEndDate(): string {
    return new Date().toISOString().split('T')[0];
  }

  maskMsisdn(msisdn: string): string {
    if (!msisdn || msisdn.length < 6) return msisdn;
    return msisdn.substring(0, 6) + '****' + msisdn.substring(msisdn.length - 2);
  }

  formatClickId(clickId?: string): string {
    if (!clickId) {
      return '-';
    }

    return clickId.length > 12 ? `${clickId.slice(0, 12)}...` : clickId;
  }

  isTriggerDisabled(tx: TransactionSummary): boolean {
    if (this.triggeringPostbackIds.has(tx.id)) return true;
    if (this.manuallyTriggeredTransactionIds.has(tx.id)) return true;
    // Disable if a postback already exists and is not in a terminal failed state
    if (tx.postback_status && tx.postback_status !== 'FAILED' && tx.postback_status !== 'DLQ') return true;
    return false;
  }

  getTriggerLabel(tx: TransactionSummary): string {
    if (this.triggeringPostbackIds.has(tx.id)) {
      return 'Sending';
    }
    if (this.manuallyTriggeredTransactionIds.has(tx.id)) {
      return 'Queued';
    }
    if (tx.postback_status === 'PENDING' || tx.postback_status === 'PROCESSING') {
      return 'Queued';
    }
    if (tx.postback_status === 'SUCCESS') {
      return 'Sent';
    }
    if (tx.postback_status === 'FAILED' || tx.postback_status === 'DLQ') {
      return 'Retry';
    }
    return 'Trigger';
  }

  triggerPostback(tx: TransactionSummary): void {
    if (this.isTriggerDisabled(tx)) {
      return;
    }

    if (!confirm(`Trigger conversion postback for transaction ${tx.id}?`)) return;
    this.triggeringPostbackIds.add(tx.id);
    const request = this.transactionService.triggerPostback(tx.id).subscribe({
      next: (response: any) => {
        this.manuallyTriggeredTransactionIds.add(tx.id);
        const enqueued = response?.enqueued || [];
        const succeeded = enqueued.filter((r: any) => r.status === 'enqueued');
        const failed = enqueued.filter((r: any) => r.status === 'failed');

        if (failed.length > 0 && succeeded.length > 0) {
          this.snackBar.open(
            `${succeeded.length} postback(s) enqueued, ${failed.length} failed`,
            'Close',
            { duration: 5000 }
          );
        } else {
          this.snackBar.open('Postback triggered successfully', 'Close', { duration: 3000 });
        }
        this.loadTransactions();
      },
      error: (err) => {
        const body = err?.error || {};
        const results = body?.results || [];
        const failedDetails = results
          .filter((r: any) => r.status === 'failed')
          .map((r: any) => `${r.provider}: ${r.error}`)
          .join('\n');

        const message = failedDetails
          || extractHttpErrorMessage(err, 'Failed to trigger postback');

        const hint = results.length === 0
          ? 'This campaign may not have postback_rules configured. Add postback rules in the campaign settings.'
          : '';

        this.dialog.open(ErrorDialogComponent, {
          width: '500px',
          data: {
            title: 'Postback Trigger Failed',
            message: hint || 'The postback could not be enqueued for this transaction.',
            details: message,
          }
        });
      }
    });
    request.add(() => {
      this.triggeringPostbackIds.delete(tx.id);
    });
  }

  formatJson(value: Record<string, unknown> | null | undefined): string {
    if (!value || Object.keys(value).length === 0) {
      return '';
    }

    return JSON.stringify(value, null, 2);
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

  private createDefaultFilters(pageSize: number = 20): TransactionListFilter {
    return {
      start_date: this.getDefaultStartDate(),
      end_date: this.getDefaultEndDate(),
      sort_by: 'created_at',
      sort_dir: 'desc',
      page: 1,
      page_size: pageSize
    };
  }
}
