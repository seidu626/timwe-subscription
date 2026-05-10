import { Component, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { ActivityLogService } from '../../+state/services/activity-log.service';
import { UserbaseService } from '../../+state/services/userbase.service';
import { AdminActivityLog } from '../../+state/models/activity-log.model';
import {
  UserbaseImportDetailResponse,
  UserbaseImportJob
} from '../../+state/models/userbase.model';

@Component({
  selector: 'app-operations-dashboard',
  templateUrl: './operations-dashboard.component.html',
  styleUrls: ['./operations-dashboard.component.scss']
})
export class OperationsDashboardComponent implements OnInit {
  logs: AdminActivityLog[] = [];
  logsLoading = false;
  logFilters = {
    entity_type: '',
    action: '',
    actor: '',
    from: '',
    to: ''
  };
  logsPage = 1;
  logsPageSize = 20;
  logsTotal = 0;

  imports: UserbaseImportJob[] = [];
  importsLoading = false;
  importsPage = 1;
  importsPageSize = 20;
  importsTotal = 0;

  selectedImport: UserbaseImportDetailResponse | null = null;
  importDetailLoading = false;

  constructor(
    private activityLogService: ActivityLogService,
    private userbaseService: UserbaseService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.loadLogs();
    this.loadImports();
  }

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
      next: (res) => {
        this.logs = res.items || [];
        this.logsTotal = res.total_count || 0;
        this.logsLoading = false;
      },
      error: () => {
        this.logsLoading = false;
        this.toast('Failed to load activity logs');
      }
    });
  }

  applyLogFilters(): void {
    this.logsPage = 1;
    this.loadLogs();
  }

  clearLogFilters(): void {
    this.logFilters = {
      entity_type: '',
      action: '',
      actor: '',
      from: '',
      to: ''
    };
    this.logsPage = 1;
    this.loadLogs();
  }

  onLogPageChange(event: PageEvent): void {
    this.logsPage = event.pageIndex + 1;
    this.logsPageSize = event.pageSize;
    this.loadLogs();
  }

  loadImports(): void {
    this.importsLoading = true;
    this.userbaseService.listImports(this.importsPage, this.importsPageSize).subscribe({
      next: (res) => {
        this.imports = res.jobs || [];
        this.importsTotal = res.total_count || 0;
        this.importsLoading = false;
      },
      error: () => {
        this.importsLoading = false;
        this.toast('Failed to load import history');
      }
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
      next: (res) => {
        this.selectedImport = res;
        this.importDetailLoading = false;
      },
      error: () => {
        this.importDetailLoading = false;
        this.toast('Failed to load import detail');
      }
    });
  }

  private toast(message: string): void {
    this.snackBar.open(message, 'Close', { duration: 4000 });
  }
}
