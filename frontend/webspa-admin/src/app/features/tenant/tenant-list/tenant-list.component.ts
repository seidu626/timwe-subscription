import { Component, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import {
  AdminChannel,
  AdminChannelCredential,
  AdminTenant,
  AdminTenantMember,
  ChannelCredentialPayload,
  ChannelCreatePayload,
  TenantCreatePayload,
  TenantMemberPayload,
  TenantMemberRole,
  TenantMemberStatus,
  TenantMutationPayload,
  TenantStatus
} from '../../+state/models/tenant.model';
import { TenantService } from '../../+state/services/tenant.service';

@Component({
  selector: 'app-tenant-list',
  templateUrl: './tenant-list.component.html',
  styleUrls: ['./tenant-list.component.scss']
})
export class TenantListComponent implements OnInit {
  loading = false;
  saving = false;
  memberLoading = false;
  memberSaving = false;
  channelLoading = false;
  channelSaving = false;
  credentialLoading = false;
  credentialSaving = false;

  readonly statuses: Array<TenantStatus | ''> = ['', 'ACTIVE', 'INACTIVE'];
  readonly memberStatuses: TenantMemberStatus[] = ['ACTIVE', 'INACTIVE'];
  readonly memberRoles: TenantMemberRole[] = ['TENANT_ADMIN', 'TENANT_VIEWER'];
  readonly capabilityOptions = ['optin', 'confirm', 'mt', 'charge'];
  displayedColumns: string[] = ['tenant_key', 'name', 'status', 'default_country', 'updated_at', 'actions'];
  memberDisplayedColumns: string[] = ['auth0_subject', 'email', 'role', 'status', 'updated_at', 'actions'];
  channelDisplayedColumns: string[] = ['channel_key', 'provider', 'capabilities', 'status', 'updated_at', 'actions'];
  credentialDisplayedColumns: string[] = ['purpose', 'version', 'redacted_display', 'status', 'updated_at'];
  trackByTenantKey = (_: number, row: any) => row?.tenant_key ?? row?.id ?? _;
  trackByMember = (_: number, row: any) => row?.auth0_subject ?? row?.id ?? _;
  trackByChannel = (_: number, row: any) => row?.channel_id ?? row?.channel_key ?? _;
  trackByCredential = (_: number, row: any) => row?.credential_id ?? `${row?.purpose ?? 'credential'}-${row?.version ?? _}`;
  dataSource = new MatTableDataSource<AdminTenant>([]);
  memberDataSource = new MatTableDataSource<AdminTenantMember>([]);
  channelDataSource = new MatTableDataSource<AdminChannel>([]);
  credentialDataSource = new MatTableDataSource<AdminChannelCredential>([]);

  totalCount = 0;
  page = 1;
  pageSize = 20;
  pageSizes = [10, 20, 50, 100];

  get activePageCount(): number {
    return this.dataSource.data.filter((tenant) => tenant.status === 'ACTIVE').length;
  }

  get inactivePageCount(): number {
    return this.dataSource.data.filter((tenant) => tenant.status === 'INACTIVE').length;
  }

  get selectedTenantLabel(): string {
    if (!this.editingTenantId) {
      return 'None selected';
    }
    const tenant = this.dataSource.data.find((item) => item.id === this.editingTenantId);
    return tenant?.tenant_key || 'Tenant selected';
  }

  filters: { q: string; status: TenantStatus | '' } = {
    q: '',
    status: ''
  };

  editingTenantId: string | null = null;
  selectedChannelId = '';
  form = this.emptyForm();
  memberForm = this.emptyMemberForm();
  channelForm = this.emptyChannelForm();
  credentialForm = this.emptyCredentialForm();
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
      tenant_key: tenant.tenant_key,
      name: tenant.name,
      status: tenant.status,
      default_country: tenant.default_country
    };
    this.metadataText = JSON.stringify(tenant.metadata || {}, null, 2);
    this.memberForm = this.emptyMemberForm();
    this.loadTenantMembers(tenant.id);
    this.loadChannels();
  }

  resetForm(): void {
    this.editingTenantId = null;
    this.form = this.emptyForm();
    this.memberForm = this.emptyMemberForm();
    this.channelForm = this.emptyChannelForm();
    this.credentialForm = this.emptyCredentialForm();
    this.memberDataSource.data = [];
    this.channelDataSource.data = [];
    this.credentialDataSource.data = [];
    this.metadataText = '{}';
    this.selectedChannelId = '';
  }

  saveTenant(): void {
    if (!this.editingTenantId && !this.form.tenant_key.trim()) {
      this.toast('Tenant key is required');
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
    const createPayload: TenantCreatePayload = {
      ...payload,
      tenant_key: this.form.tenant_key.trim().toLowerCase(),
      name: payload.name || '',
      status: payload.status || 'ACTIVE',
      default_country: payload.default_country || 'GH'
    };
    const request$ = this.editingTenantId
      ? this.tenantService.update(this.editingTenantId, payload)
      : this.tenantService.create(createPayload);

    request$.subscribe({
      next: () => {
        this.saving = false;
        this.toast(this.editingTenantId ? 'Tenant updated' : 'Tenant created');
        this.resetForm();
        this.loadTenants();
      },
      error: (err) => {
        this.saving = false;
        this.toast(this.extractErrorMessage(err, this.editingTenantId ? 'Failed to update tenant' : 'Failed to create tenant'));
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

  saveMember(): void {
    if (!this.editingTenantId) {
      this.toast('Select a tenant first');
      return;
    }
    if (!this.memberForm.auth0_subject.trim()) {
      this.toast('Auth0 subject is required');
      return;
    }
    const payload: TenantMemberPayload = {
      auth0_subject: this.memberForm.auth0_subject.trim(),
      role: this.memberForm.role,
      status: this.memberForm.status
    };
    if (this.memberForm.email.trim()) {
      payload.email = this.memberForm.email.trim().toLowerCase();
    }

    this.memberSaving = true;
    this.tenantService.upsertMember(this.editingTenantId, payload).subscribe({
      next: () => {
        this.memberSaving = false;
        this.toast('Tenant member saved');
        this.memberForm = this.emptyMemberForm();
        this.loadTenantMembers(this.editingTenantId || '');
      },
      error: (err) => {
        this.memberSaving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to save tenant member'));
      }
    });
  }

  deactivateMember(member: AdminTenantMember): void {
    if (!this.editingTenantId) {
      return;
    }
    this.memberSaving = true;
    this.tenantService.deactivateMember(this.editingTenantId, member.auth0_subject).subscribe({
      next: () => {
        this.memberSaving = false;
        this.toast('Tenant member deactivated');
        this.loadTenantMembers(this.editingTenantId || '');
      },
      error: (err) => {
        this.memberSaving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to deactivate tenant member'));
      }
    });
  }

  saveChannel(): void {
    if (!this.channelForm.provider.trim() || !this.channelForm.country.trim()) {
      this.toast('Provider and country are required');
      return;
    }
    if (this.channelForm.capabilities.length === 0) {
      this.toast('Select at least one channel capability');
      return;
    }

    const operator = this.channelForm.operator.trim();
    const payload: ChannelCreatePayload = {
      provider: this.channelForm.provider.trim().toLowerCase(),
      country: this.channelForm.country.trim().toUpperCase(),
      capabilities: [...this.channelForm.capabilities].sort(),
      enabled: this.channelForm.enabled
    };
    if (operator) {
      payload.operator = operator;
    }

    this.channelSaving = true;
    this.tenantService.createChannel(payload).subscribe({
      next: (channel) => {
        this.channelSaving = false;
        this.channelForm = this.emptyChannelForm();
        this.selectedChannelId = channel.channel_id;
        this.toast('Channel created');
        this.loadChannels();
      },
      error: (err) => {
        this.channelSaving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to create channel'));
      }
    });
  }

  setChannelEnabled(channel: AdminChannel, enabled: boolean): void {
    this.channelSaving = true;
    this.tenantService.setChannelEnabled(channel.channel_id, enabled).subscribe({
      next: () => {
        this.channelSaving = false;
        this.toast(enabled ? 'Channel enabled' : 'Channel disabled');
        this.loadChannels();
      },
      error: (err) => {
        this.channelSaving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to update channel'));
      }
    });
  }

  selectChannel(channelId: string): void {
    this.selectedChannelId = channelId;
    this.loadChannelCredentials(channelId);
  }

  capabilitySelected(capability: string): boolean {
    return this.channelForm.capabilities.includes(capability);
  }

  toggleCapability(capability: string, checked: boolean): void {
    const next = new Set(this.channelForm.capabilities);
    if (checked) {
      next.add(capability);
    } else {
      next.delete(capability);
    }
    this.channelForm.capabilities = Array.from(next).sort();
  }

  saveCredentialReference(): void {
    if (!this.selectedChannelId) {
      this.toast('Select a channel first');
      return;
    }
    const secretRef = this.credentialForm.secret_ref.trim();
    if (!secretRef) {
      this.toast('Credential secret reference is required');
      return;
    }

    const payload: ChannelCredentialPayload = {
      purpose: this.credentialForm.purpose.trim() || 'provider_api',
      secret_ref: secretRef
    };

    this.credentialSaving = true;
    this.tenantService.bindChannelCredential(this.selectedChannelId, payload).subscribe({
      next: () => {
        this.credentialSaving = false;
        this.credentialForm = this.emptyCredentialForm();
        this.toast('Credential reference bound');
        this.loadChannelCredentials(this.selectedChannelId);
      },
      error: (err) => {
        this.credentialSaving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to bind credential reference'));
      }
    });
  }

  private loadTenantMembers(tenantId: string): void {
    if (!tenantId) {
      this.memberDataSource.data = [];
      return;
    }
    this.memberLoading = true;
    this.tenantService.listMembers(tenantId, { page: 1, page_size: 100 }).subscribe({
      next: (response) => {
        this.memberDataSource.data = response.members || [];
        this.memberLoading = false;
      },
      error: (err) => {
        this.memberLoading = false;
        this.memberDataSource.data = [];
        this.toast(this.extractErrorMessage(err, 'Failed to load tenant members'));
      }
    });
  }

  private loadChannels(): void {
    this.channelLoading = true;
    this.tenantService.listChannels({ page: 1, page_size: 100 }).subscribe({
      next: (response) => {
        const channels = response.channels || [];
        this.channelDataSource.data = channels;
        this.channelLoading = false;
        if (!channels.some((channel) => channel.channel_id === this.selectedChannelId)) {
          this.selectedChannelId = channels[0]?.channel_id || '';
        }
        if (this.selectedChannelId) {
          this.loadChannelCredentials(this.selectedChannelId);
        } else {
          this.credentialDataSource.data = [];
        }
      },
      error: (err) => {
        this.channelLoading = false;
        this.channelDataSource.data = [];
        this.credentialDataSource.data = [];
        this.toast(this.extractErrorMessage(err, 'Failed to load channels'));
      }
    });
  }

  private loadChannelCredentials(channelId: string): void {
    if (!channelId) {
      this.credentialDataSource.data = [];
      return;
    }
    this.credentialLoading = true;
    this.tenantService.listChannelCredentials(channelId, 'provider_api').subscribe({
      next: (response) => {
        this.credentialDataSource.data = response.credentials || [];
        this.credentialLoading = false;
      },
      error: (err) => {
        this.credentialDataSource.data = [];
        this.credentialLoading = false;
        this.toast(this.extractErrorMessage(err, 'Failed to load credential references'));
      }
    });
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

  private emptyForm(): { tenant_key: string; name: string; status: TenantStatus; default_country: string } {
    return {
      tenant_key: '',
      name: '',
      status: 'ACTIVE',
      default_country: 'GH'
    };
  }

  private emptyMemberForm(): { auth0_subject: string; email: string; role: TenantMemberRole; status: TenantMemberStatus } {
    return {
      auth0_subject: '',
      email: '',
      role: 'TENANT_ADMIN',
      status: 'ACTIVE'
    };
  }

  private emptyChannelForm(): { provider: string; country: string; operator: string; capabilities: string[]; enabled: boolean } {
    return {
      provider: 'timwe',
      country: 'GH',
      operator: '',
      capabilities: ['optin', 'confirm', 'mt'],
      enabled: false
    };
  }

  private emptyCredentialForm(): { purpose: string; secret_ref: string } {
    return {
      purpose: 'provider_api',
      secret_ref: ''
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
