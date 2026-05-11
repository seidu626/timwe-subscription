import { Component, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import { AdminTenant, TenantMutationPayload, TenantStatus } from '../../+state/models/tenant.model';
import { TenantService } from '../../+state/services/tenant.service';

@Component({
  selector: 'app-tenant-list',
  templateUrl: './tenant-list.component.html',
  styleUrls: ['./tenant-list.component.scss']
})
export class TenantListComponent implements OnInit {
  loading = false;
  saving = false;

  readonly statuses: Array<TenantStatus | ''> = ['', 'ACTIVE', 'INACTIVE'];
  displayedColumns: string[] = ['tenant_key', 'name', 'status', 'default_country', 'updated_at', 'actions'];
  dataSource = new MatTableDataSource<AdminTenant>([]);

  totalCount = 0;
  page = 1;
  pageSize = 20;
  pageSizes = [10, 20, 50, 100];

  filters: { q: string; status: TenantStatus | '' } = {
    q: '',
    status: ''
  };

  editingTenantId: string | null = null;
  form = this.emptyForm();
  metadataText = '{}';

  constructor(
    private tenantService: TenantService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.loadTenants();
  }

  loadTenants(): void {
    this.loading = true;
    this.tenantService.list({
      page: this.page,
      page_size: this.pageSize,
      q: this.filters.q || undefined,
      status: this.filters.status || undefined
    }).subscribe({
      next: (response) => {
        this.dataSource.data = response.tenants || [];
        this.totalCount = response.total_count || 0;
        this.loading = false;
      },
      error: (err) => {
        this.loading = false;
        this.toast(this.extractErrorMessage(err, 'Failed to load tenants'));
      }
    });
  }

  applyFilters(): void {
    this.page = 1;
    this.loadTenants();
  }

  clearFilters(): void {
    this.filters = { q: '', status: '' };
    this.page = 1;
    this.loadTenants();
  }

  onPageChange(event: PageEvent): void {
    this.page = event.pageIndex + 1;
    this.pageSize = event.pageSize;
    this.loadTenants();
  }

  editTenant(tenant: AdminTenant): void {
    this.editingTenantId = tenant.id;
    this.form = {
      name: tenant.name,
      status: tenant.status,
      default_country: tenant.default_country
    };
    this.metadataText = JSON.stringify(tenant.metadata || {}, null, 2);
  }

  resetForm(): void {
    this.editingTenantId = null;
    this.form = this.emptyForm();
    this.metadataText = '{}';
  }

  saveTenant(): void {
    if (!this.editingTenantId) {
      this.toast('Select a tenant to update');
      return;
    }
    if (!this.form.name.trim() || !this.form.default_country.trim()) {
      this.toast('Name and default country are required');
      return;
    }

    const metadata = this.parseMetadata();
    if (metadata === null) {
      return;
    }

    const payload: TenantMutationPayload = {
      name: this.form.name.trim(),
      status: this.form.status,
      default_country: this.form.default_country.trim().toUpperCase(),
      metadata
    };

    this.saving = true;
    this.tenantService.update(this.editingTenantId, payload).subscribe({
      next: () => {
        this.saving = false;
        this.toast('Tenant updated');
        this.resetForm();
        this.loadTenants();
      },
      error: (err) => {
        this.saving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to update tenant'));
      }
    });
  }

  metadataPreview(tenant: AdminTenant): string {
    const metadata = tenant.metadata || {};
    const keys = Object.keys(metadata);
    if (keys.length === 0) {
      return '{}';
    }
    return keys.slice(0, 3).map((key) => `${key}: ${String(metadata[key])}`).join(', ');
  }

  private parseMetadata(): Record<string, unknown> | null {
    try {
      const parsed = JSON.parse(this.metadataText || '{}');
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
        this.toast('Metadata must be a JSON object');
        return null;
      }
      return parsed as Record<string, unknown>;
    } catch {
      this.toast('Metadata must be valid JSON');
      return null;
    }
  }

  private emptyForm(): { name: string; status: TenantStatus; default_country: string } {
    return {
      name: '',
      status: 'ACTIVE',
      default_country: 'GH'
    };
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
