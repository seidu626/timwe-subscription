import { Component, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import {
  UpsertUserbaseRequest,
  UserbaseImportUploadResponse,
  UserbaseRecord
} from '../../+state/models/userbase.model';
import { UserbaseService } from '../../+state/services/userbase.service';

@Component({
  selector: 'app-userbase-list',
  templateUrl: './userbase-list.component.html',
  styleUrls: ['./userbase-list.component.scss']
})
export class UserbaseListComponent implements OnInit {
  loading = false;
  saving = false;

  displayedColumns: string[] = ['id', 'msisdn', 'type', 'actions'];
  dataSource = new MatTableDataSource<UserbaseRecord>([]);

  totalCount = 0;
  page = 1;
  pageSize = 20;
  pageSizes = [10, 20, 50, 100];

  filters = {
    msisdn: '',
    type: ''
  };

  form: UpsertUserbaseRequest = {
    msisdn: '',
    type: 'BLACKLISTED'
  };

  selectedFile: File | null = null;
  lastImportResult: UserbaseImportUploadResponse | null = null;

  constructor(
    private userbaseService: UserbaseService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.loadUserbase();
  }

  loadUserbase(): void {
    this.loading = true;
    this.userbaseService.list({
      page: this.page,
      page_size: this.pageSize,
      msisdn: this.filters.msisdn || undefined,
      type: this.filters.type || undefined
    }).subscribe({
      next: (response) => {
        this.dataSource.data = response.records || [];
        this.totalCount = response.total_count || 0;
        this.loading = false;
      },
      error: () => {
        this.loading = false;
        this.toast('Failed to load userbase');
      }
    });
  }

  applyFilters(): void {
    this.page = 1;
    this.loadUserbase();
  }

  clearFilters(): void {
    this.filters = { msisdn: '', type: '' };
    this.page = 1;
    this.loadUserbase();
  }

  onPageChange(event: PageEvent): void {
    this.page = event.pageIndex + 1;
    this.pageSize = event.pageSize;
    this.loadUserbase();
  }

  upsert(): void {
    if (!this.form.msisdn || !this.form.type) {
      this.toast('msisdn and type are required');
      return;
    }

    this.saving = true;
    this.userbaseService.upsert(this.form).subscribe({
      next: () => {
        this.saving = false;
        this.toast('Userbase record saved');
        this.form = { msisdn: '', type: this.form.type };
        this.loadUserbase();
      },
      error: (err) => {
        this.saving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to save userbase record'));
      }
    });
  }

  delete(rec: UserbaseRecord): void {
    const ok = confirm(`Delete ${rec.msisdn} from userbase?`);
    if (!ok) {
      return;
    }
    this.userbaseService.delete(rec.msisdn).subscribe({
      next: () => {
        this.toast('Userbase record deleted');
        this.loadUserbase();
      },
      error: (err) => this.toast(this.extractErrorMessage(err, 'Failed to delete userbase record'))
    });
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    this.selectedFile = input.files?.[0] || null;
  }

  upload(): void {
    if (!this.selectedFile) {
      this.toast('Choose a CSV/XLSX file first');
      return;
    }

    this.saving = true;
    this.userbaseService.upload(this.selectedFile).subscribe({
      next: (response) => {
        this.saving = false;
        this.lastImportResult = response;
        this.toast(`Import completed: ${response.job.success_rows} success, ${response.job.failed_rows} failed`);
        this.loadUserbase();
      },
      error: (err) => {
        this.saving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to import userbase file'));
      }
    });
  }

  private extractErrorMessage(err: any, fallback: string): string {
    if (typeof err?.error === 'string' && err.error.trim()) {
      return err.error;
    }
    if (err?.error?.error) {
      return err.error.error;
    }
    if (err?.message) {
      return err.message;
    }
    return fallback;
  }

  private toast(message: string): void {
    this.snackBar.open(message, 'Close', { duration: 4000 });
  }
}
