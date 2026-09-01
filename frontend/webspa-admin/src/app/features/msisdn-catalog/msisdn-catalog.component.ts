import { Component, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import { finalize, forkJoin } from 'rxjs';
import {
  DNDImportResponse,
  MSISDNCatalogRecord,
  MSISDNCatalogStats
} from '../+state/models/msisdn-catalog.model';
import { MSISDNCatalogService } from '../+state/services/msisdn-catalog.service';

@Component({
  selector: 'app-msisdn-catalog',
  templateUrl: './msisdn-catalog.component.html',
  styleUrls: ['./msisdn-catalog.component.scss']
})
export class MSISDNCatalogComponent implements OnInit {
  readonly columns = ['select', 'msisdn', 'market', 'verification', 'guardrails', 'source', 'checked', 'actions'];
  readonly pageSizes = [25, 50, 100, 200];
  readonly emptyStats: MSISDNCatalogStats = { total: 0, ready: 0, verified: 0, pending: 0, dnd: 0, invalid: 0, errors: 0 };

  dataSource = new MatTableDataSource<MSISDNCatalogRecord>([]);
  stats = { ...this.emptyStats };
  selected = new Set<number>();
  totalCount = 0;
  page = 1;
  pageSize = 25;
  loading = false;
  actionPending = false;
  uploadExpanded = false;

  filters = { q: '', region: '', telco: '', status: '', pool: '' };
  dndFile: File | null = null;
  dndRegion = 'GH';
  dndTelco = 'MTN';
  dndResult: DNDImportResponse | null = null;

  constructor(private readonly catalog: MSISDNCatalogService, private readonly snackBar: MatSnackBar) {}

  ngOnInit(): void { this.refresh(); }

  refresh(): void {
    this.loading = true;
    const poolFlags = this.poolFilters();
    forkJoin({
      list: this.catalog.list({
        page: this.page, page_size: this.pageSize, q: this.filters.q,
        region: this.filters.region, telco: this.filters.telco,
        status: this.filters.pool === 'READY' ? 'VERIFIED' : this.filters.status,
        ...poolFlags
      }),
      stats: this.catalog.stats()
    }).pipe(finalize(() => this.loading = false)).subscribe({
      next: ({ list, stats }) => {
        this.dataSource.data = list.records || [];
        this.totalCount = list.total_count || 0;
        this.stats = stats || { ...this.emptyStats };
        this.selected.clear();
      },
      error: (error) => this.toast(this.errorMessage(error, 'Could not load the catalog'))
    });
  }

  applyFilters(): void { this.page = 1; this.refresh(); }
  clearFilters(): void { this.filters = { q: '', region: '', telco: '', status: '', pool: '' }; this.page = 1; this.refresh(); }
  onPage(event: PageEvent): void { this.page = event.pageIndex + 1; this.pageSize = event.pageSize; this.refresh(); }

  toggleSelection(id: number, checked: boolean): void { checked ? this.selected.add(id) : this.selected.delete(id); }
  togglePage(checked: boolean): void {
    this.selected.clear();
    if (checked) { this.dataSource.data.forEach(row => this.selected.add(row.id)); }
  }
  pageSelected(): boolean { return this.dataSource.data.length > 0 && this.dataSource.data.every(row => this.selected.has(row.id)); }
  pagePartiallySelected(): boolean { return this.selected.size > 0 && !this.pageSelected(); }

  verify(): void {
    const ids = [...this.selected];
    this.actionPending = true;
    this.catalog.verify(ids, ids.length ? ids.length : 100).pipe(finalize(() => this.actionPending = false)).subscribe({
      next: result => {
        this.toast(`MADAPI complete: ${result.verified} verified, ${result.invalid} invalid, ${result.errors} errors`);
        this.refresh();
      },
      error: error => this.toast(this.errorMessage(error, 'MADAPI verification failed'))
    });
  }

  setDND(record: MSISDNCatalogRecord, dnd: boolean): void {
    this.actionPending = true;
    this.catalog.setFlags(record.id, { dnd }).pipe(finalize(() => this.actionPending = false)).subscribe({
      next: () => { this.toast(dnd ? 'Moved to DND pool' : 'Removed from DND pool'); this.refresh(); },
      error: error => this.toast(this.errorMessage(error, 'Could not update DND status'))
    });
  }

  setInvalid(record: MSISDNCatalogRecord, invalid: boolean): void {
    this.actionPending = true;
    this.catalog.setFlags(record.id, { invalid, reason: invalid ? 'Marked invalid by operator' : '' })
      .pipe(finalize(() => this.actionPending = false)).subscribe({
        next: () => { this.toast(invalid ? 'Moved to invalid pool' : 'Returned to pending verification'); this.refresh(); },
        error: error => this.toast(this.errorMessage(error, 'Could not update invalid status'))
      });
  }

  onDNDFile(event: Event): void {
    const input = event.target as HTMLInputElement;
    this.dndFile = input.files?.[0] || null;
    this.dndResult = null;
  }

  uploadDND(): void {
    if (!this.dndFile) { this.toast('Choose a CSV file first'); return; }
    this.actionPending = true;
    this.catalog.uploadDND(this.dndFile, this.dndRegion, this.dndTelco)
      .pipe(finalize(() => this.actionPending = false)).subscribe({
        next: result => { this.dndResult = result; this.toast(`${result.imported} DND numbers saved`); this.refresh(); },
        error: error => this.toast(this.errorMessage(error, 'DND upload failed'))
      });
  }

  statusLabel(status: string): string { return status === 'ERROR' ? 'Needs retry' : status.toLowerCase(); }
  sourceLabel(source: string): string { return source.replaceAll('_', ' ').toLowerCase(); }
  trackById = (_: number, row: MSISDNCatalogRecord): number => row.id;

  private poolFilters(): { dnd?: boolean; invalid?: boolean } {
    if (this.filters.pool === 'DND') { return { dnd: true }; }
    if (this.filters.pool === 'INVALID') { return { invalid: true }; }
    if (this.filters.pool === 'READY') { return { dnd: false, invalid: false }; }
    return {};
  }

  private errorMessage(error: any, fallback: string): string {
    if (typeof error?.error === 'string' && error.error.trim()) { return error.error; }
    return error?.error?.error || error?.message || fallback;
  }

  private toast(message: string): void { this.snackBar.open(message, 'Close', { duration: 4500 }); }
}
