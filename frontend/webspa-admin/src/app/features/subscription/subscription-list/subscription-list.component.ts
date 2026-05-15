import { Component, OnInit, ViewChild } from '@angular/core';
import { MatPaginator } from '@angular/material/paginator';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Sort } from '@angular/material/sort';
import { MatTableDataSource } from '@angular/material/table';
import {
  AdminActionDetail,
  AdminActionSummary,
  AdminSubscriptionActionRequest,
  AdminSubscriptionOperation,
  Subscription,
} from '../../+state/models/subscription.model';
import { SubscriptionService } from '../../+state/services/subscription.service';

@Component({
  selector: 'app-subscription-list',
  templateUrl: './subscription-list.component.html',
  styleUrls: ['./subscription-list.component.scss']
})
export class SubscriptionListComponent implements OnInit {
  loading = false;

  displayedColumns: string[] = [
    'id',
    'userIdentifier',
    'productId',
    'entryChannel',
    'startDate',
    'endDate',
  ];
  filters = { startDate: '', endDate: '', shortcode: '', userIdentifier: '', entryChannel: '' };

  dataSource = new MatTableDataSource<Subscription>([]);
  totalSubscriptions = 0;
  subscriptionPageIndex = 0;
  subscriptionPageSize = 10;
  pageSizes = [5, 10, 20, 30];

  sortBy = 'startDate';
  sortDir: 'asc' | 'desc' = 'desc';

  adminOperations: AdminSubscriptionOperation[] = ['optin', 'optout', 'confirm', 'status'];
  selectedOperation: AdminSubscriptionOperation = 'optin';
  adminActionLoading = false;
  adminHeadersText = '';
  lastActionResult: AdminActionDetail | null = null;

  adminForm: {
    msisdn: string;
    productId: number | null;
    partnerRoleId: number | null;
    userIdentifierType: string;
    mcc: string;
    mnc: string;
    entryChannel: string;
    largeAccount: string;
    subKeyword: string;
    trackingId: string;
    clientIp: string;
    campaignUrl: string;
    controlKeyword: string;
    controlServiceKeyword: string;
    subId: number | null;
    cancelReason: number | null;
    cancelSource: number | null;
    transactionAuthCode: string;
    externalTxId: string;
    adminRequestId: string;
  } = {
    msisdn: '',
    productId: null,
    partnerRoleId: null,
    userIdentifierType: '',
    mcc: '',
    mnc: '',
    entryChannel: '',
    largeAccount: '',
    subKeyword: '',
    trackingId: '',
    clientIp: '',
    campaignUrl: '',
    controlKeyword: '',
    controlServiceKeyword: '',
    subId: null,
    cancelReason: null,
    cancelSource: null,
    transactionAuthCode: '',
    externalTxId: '',
    adminRequestId: '',
  };

  historyLoading = false;
  historyFilters: {
    operation: AdminSubscriptionOperation | '';
    msisdn: string;
    externalTxId: string;
    adminRequestId: string;
    productId: number | null;
    startDate: string;
    endDate: string;
    result: '' | 'ok' | 'error';
  } = {
    operation: '',
    msisdn: '',
    externalTxId: '',
    adminRequestId: '',
    productId: null,
    startDate: '',
    endDate: '',
    result: '',
  };

  historyResultOptions: Array<{ label: string; value: '' | 'ok' | 'error' }> = [
    { label: 'All', value: '' },
    { label: 'OK', value: 'ok' },
    { label: 'Error', value: 'error' },
  ];

  historyDisplayedColumns: string[] = [
    'createdAt',
    'operation',
    'msisdn',
    'productId',
    'responseStatusCode',
    'durationMs',
    'hasError',
    'actions',
  ];
  historyDataSource = new MatTableDataSource<AdminActionSummary>([]);
  historyTotalCount = 0;
  historyPageIndex = 0;
  historyPageSize = 10;
  historyPageSizes = [5, 10, 20, 50];
  historySortBy = 'createdAt';
  historySortDir: 'asc' | 'desc' = 'desc';
  selectedHistoryAction: AdminActionDetail | null = null;

  @ViewChild('subscriptionPaginator') subscriptionPaginator!: MatPaginator;
  @ViewChild('historyPaginator') historyPaginator!: MatPaginator;

  constructor(
    private subscriptionService: SubscriptionService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.loadSubscriptions(1, this.subscriptionPageSize, this.filters);
    this.loadActionHistory(1, this.historyPageSize);
  }

  trackById = (_: number, row: any) => row?.id ?? _;

  applyFilters() {
    this.subscriptionPageIndex = 0;
    this.loadSubscriptions(1, this.subscriptionPageSize, this.filters);
  }

  onPageChange(event: any) {
    this.subscriptionPageIndex = event.pageIndex;
    this.subscriptionPageSize = event.pageSize;
    this.loadSubscriptions(event.pageIndex + 1, event.pageSize, this.filters);
  }

  onSortChange(event: Sort) {
    this.sortBy = event.active || 'startDate';
    this.sortDir = (event.direction || 'desc') as 'asc' | 'desc';
    this.subscriptionPageIndex = 0;
    this.loadSubscriptions(1, this.subscriptionPageSize, this.filters);
  }

  loadSubscriptions(page: number, pageSize: number, filters: any) {
    this.loading = true;
    const formattedFilters = {
      ...filters,
      page,
      pageSize,
      sort_by: this.sortBy,
      sort_dir: this.sortDir,
      startDate: this.toDateQuery(filters.startDate),
      endDate: this.toDateQuery(filters.endDate)
    };

    this.subscriptionService.getSubscriptions(formattedFilters).subscribe({
      next: (response) => {
        this.dataSource.data = response.data;
        this.totalSubscriptions = response.totalCount;
        this.subscriptionPageIndex = (response.page || page) - 1;
        this.subscriptionPageSize = response.pageSize || pageSize;
        this.loading = false;
      },
      error: () => {
        this.loading = false;
        this.snackBar.open('Failed to load subscriptions', 'Close', {
          duration: 5000,
          panelClass: ['error-snackbar']
        });
      }
    });
  }

  submitAdminAction() {
    if (this.adminActionLoading) {
      return;
    }

    const msisdn = this.adminForm.msisdn.trim();
    if (!msisdn || !this.adminForm.productId) {
      this.snackBar.open('MSISDN and Product ID are required', 'Close', { duration: 4000 });
      return;
    }
    if (this.selectedOperation === 'confirm' && !this.adminForm.transactionAuthCode) {
      this.snackBar.open('Transaction Auth Code is required for confirm', 'Close', { duration: 4000 });
      return;
    }
    const headers = this.parseCustomHeaders();
    if (headers === null) {
      return;
    }

    const externalTxId = this.refreshExternalTxId();
    this.lastActionResult = null;
    this.selectedHistoryAction = null;

    const payload: AdminSubscriptionActionRequest = {
      msisdn,
      productId: this.adminForm.productId,
      partnerRoleId: this.adminForm.partnerRoleId || undefined,
      userIdentifierType: this.adminForm.userIdentifierType || undefined,
      mcc: this.adminForm.mcc || undefined,
      mnc: this.adminForm.mnc || undefined,
      entryChannel: this.adminForm.entryChannel || undefined,
      largeAccount: this.adminForm.largeAccount || undefined,
      subKeyword: this.adminForm.subKeyword || undefined,
      trackingId: this.adminForm.trackingId || undefined,
      clientIp: this.adminForm.clientIp || undefined,
      campaignUrl: this.adminForm.campaignUrl || undefined,
      controlKeyword: this.adminForm.controlKeyword || undefined,
      controlServiceKeyword: this.adminForm.controlServiceKeyword || undefined,
      subId: this.adminForm.subId || undefined,
      cancelReason: this.adminForm.cancelReason || undefined,
      cancelSource: this.adminForm.cancelSource || undefined,
      transactionAuthCode: this.adminForm.transactionAuthCode || undefined,
      externalTxId,
      adminRequestId: this.adminForm.adminRequestId || undefined,
      headers: headers || undefined,
    };

    this.adminActionLoading = true;
    this.subscriptionService.executeAdminAction(this.selectedOperation, payload).subscribe({
      next: (result) => {
        this.adminActionLoading = false;
        this.lastActionResult = result;
        this.selectedHistoryAction = result;
        this.historyPageIndex = 0;
        this.loadActionHistory(1, this.historyPageSize);

        const hasError = !!result.error;
        this.snackBar.open(
          hasError ? `Action completed with TIMWE error (${result.operation})` : `Action completed (${result.operation})`,
          'Close',
          { duration: hasError ? 6000 : 4000 }
        );
      },
      error: (err) => {
        this.adminActionLoading = false;
        const message = err?.error?.message || 'Failed to execute admin action';
        this.snackBar.open(`${message} (externalTxId: ${externalTxId})`, 'Close', {
          duration: 6000,
          panelClass: ['error-snackbar']
        });
      }
    });
  }

  loadActionHistory(page: number, pageSize: number) {
    this.historyLoading = true;
    this.subscriptionService.getAdminActionHistory({
      operation: this.historyFilters.operation,
      msisdn: this.historyFilters.msisdn || undefined,
      externalTxId: this.historyFilters.externalTxId || undefined,
      adminRequestId: this.historyFilters.adminRequestId || undefined,
      productId: this.historyFilters.productId || undefined,
      startDate: this.toDateQuery(this.historyFilters.startDate) || undefined,
      endDate: this.toDateQuery(this.historyFilters.endDate) || undefined,
      result: this.historyFilters.result || undefined,
      sortBy: this.historySortBy,
      sortDir: this.historySortDir,
      page,
      pageSize,
    }).subscribe({
      next: (response) => {
        this.historyLoading = false;
        this.historyDataSource.data = response.data || [];
        this.historyTotalCount = response.totalCount || 0;
        this.historyPageIndex = (response.page || page) - 1;
        this.historyPageSize = response.pageSize || pageSize;
      },
      error: () => {
        this.historyLoading = false;
        this.snackBar.open('Failed to load action history', 'Close', {
          duration: 5000,
          panelClass: ['error-snackbar']
        });
      }
    });
  }

  applyHistoryFilters() {
    this.historyPageIndex = 0;
    this.loadActionHistory(1, this.historyPageSize);
  }

  onHistoryPageChange(event: any) {
    this.historyPageIndex = event.pageIndex;
    this.historyPageSize = event.pageSize;
    this.loadActionHistory(event.pageIndex + 1, event.pageSize);
  }

  onHistorySortChange(event: Sort) {
    this.historySortBy = event.active || 'createdAt';
    this.historySortDir = (event.direction || 'desc') as 'asc' | 'desc';
    this.historyPageIndex = 0;
    this.loadActionHistory(1, this.historyPageSize);
  }

  viewActionDetails(actionId: string) {
    if (!actionId) {
      return;
    }
    this.subscriptionService.getAdminActionById(actionId).subscribe({
      next: (result) => {
        this.selectedHistoryAction = result;
      },
      error: () => {
        this.snackBar.open('Failed to load action details', 'Close', {
          duration: 5000,
          panelClass: ['error-snackbar']
        });
      }
    });
  }

  private parseCustomHeaders(): Record<string, string> | null | undefined {
    const raw = this.adminHeadersText.trim();
    if (!raw) {
      return undefined;
    }

    try {
      const parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        this.snackBar.open('Custom headers must be a JSON object', 'Close', { duration: 5000 });
        return null;
      }

      const entries = Object.entries(parsed as Record<string, unknown>)
        .filter(([key, value]) => !!key && value !== null && value !== undefined)
        .map(([key, value]) => [key, String(value)]);

      return Object.fromEntries(entries);
    } catch {
      this.snackBar.open('Custom headers JSON is invalid', 'Close', { duration: 5000 });
      return null;
    }
  }

  toPrettyJson(value: unknown): string {
    if (value === undefined || value === null) {
      return '{}';
    }
    try {
      return JSON.stringify(value, null, 2);
    } catch {
      return String(value);
    }
  }

  private toDateQuery(value: any): string {
    if (!value) return '';
    const date = value instanceof Date ? value : new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    const year = date.getFullYear();
    const month = `${date.getMonth() + 1}`.padStart(2, '0');
    const day = `${date.getDate()}`.padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  private refreshExternalTxId(): string {
    const externalTxId = this.generateExternalTxId();
    this.adminForm.externalTxId = externalTxId;
    return externalTxId;
  }

  private generateExternalTxId(): string {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    return `admin-${Date.now()}-${Math.random().toString(16).slice(2, 10)}`;
  }
}
