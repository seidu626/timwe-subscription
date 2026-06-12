import { Component, OnDestroy, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { PageEvent } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import { Subject } from 'rxjs';
import { filter, takeUntil } from 'rxjs/operators';
import {
  AdminChannel,
  AdminChannelCredential,
  AdminTenant,
  AdminTenantMember,
  ChannelCredentialPayload,
  ChannelCreatePayload,
  ChannelUpdatePayload,
  CredentialSecretValue,
  RevokeCredentialResponse,
  TenantCreatePayload,
  TenantMemberPayload,
  TenantMemberRole,
  TenantMemberStatus,
  TenantMutationPayload,
  TenantStatus
} from '../../+state/models/tenant.model';
import { TenantService } from '../../+state/services/tenant.service';
import {
  TenantWorkspaceOption,
  TenantWorkspaceService,
  TenantWorkspaceState
} from '../../../core/services/tenant-workspace.service';

@Component({
  selector: 'app-tenant-list',
  templateUrl: './tenant-list.component.html',
  styleUrls: ['./tenant-list.component.scss']
})
export class TenantListComponent implements OnInit, OnDestroy {
  loading = false;
  saving = false;
  memberLoading = false;
  memberSaving = false;
  channelLoading = false;
  channelSaving = false;
  credentialLoading = false;
  credentialSaving = false;
  credentialRevoking = false;
  platformScoped = true;
  currentWorkspaceTenant: TenantWorkspaceOption | null = null;

  readonly statuses: Array<TenantStatus | ''> = ['', 'ACTIVE', 'INACTIVE'];
  readonly memberStatuses: TenantMemberStatus[] = ['ACTIVE', 'INACTIVE'];
  readonly memberRoles: TenantMemberRole[] = ['TENANT_ADMIN', 'TENANT_VIEWER'];
  readonly capabilityOptions = ['optin', 'confirm', 'mt', 'charge'];
  displayedColumns: string[] = ['tenant_key', 'name', 'status', 'default_country', 'updated_at', 'actions'];
  memberDisplayedColumns: string[] = ['auth0_subject', 'email', 'role', 'status', 'updated_at', 'actions'];
  channelDisplayedColumns: string[] = ['channel_key', 'provider', 'capabilities', 'status', 'updated_at', 'actions'];
  credentialDisplayedColumns: string[] = ['purpose', 'version', 'redacted_display', 'status', 'updated_at', 'created_at', 'actions'];
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

  filters: { q: string; status: TenantStatus | '' } = {
    q: '',
    status: ''
  };

  /** 'ref' = secret_ref mode; 'value' = per-field direct-value mode */
  credentialMode: 'ref' | 'value' = 'ref';
  showApiKey = false;
  showMtApiKey = false;
  showPsk = false;

  editingTenantId: string | null = null;
  editingChannelId: string | null = null;
  selectedChannelId = '';
  form = this.emptyForm();
  memberForm = this.emptyMemberForm();
  channelForm = this.emptyChannelForm();
  channelEditForm = this.emptyChannelEditForm();
  credentialForm = this.emptyCredentialForm();
  credentialValueForm = this.emptyCredentialValueForm();
  metadataText = '{}';
  private readonly destroy$ = new Subject<void>();
  private workspaceKey = '';

  constructor(
    private tenantService: TenantService,
    private snackBar: MatSnackBar,
    private tenantWorkspace: TenantWorkspaceService
  ) {}

  ngOnInit(): void {
    this.tenantWorkspace.workspace$.pipe(
      filter((workspace) => !workspace.loading),
      takeUntil(this.destroy$)
    ).subscribe((workspace) => this.applyWorkspace(workspace));
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadTenants(): void {
    if (!this.platformScoped) {
      this.loadCurrentTenant();
      return;
    }
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
    this.selectTenantForManagement(tenant);
  }

  private selectTenantForManagement(tenant: AdminTenant): void {
    this.editingTenantId = tenant.id;
    this.form = {
      tenant_key: tenant.tenant_key,
      name: tenant.name,
      status: tenant.status,
      default_country: tenant.default_country
    };
    this.metadataText = JSON.stringify(tenant.metadata || {}, null, 2);
    this.memberForm = this.emptyMemberForm();
    if (this.platformScoped) {
      this.loadTenantMembers(tenant.id);
    } else {
      this.memberDataSource.data = [];
    }
    this.loadChannels();
  }

  resetForm(): void {
    if (!this.platformScoped && this.dataSource.data.length === 1) {
      this.selectTenantForManagement(this.dataSource.data[0]);
      return;
    }
    this.editingTenantId = null;
    this.editingChannelId = null;
    this.form = this.emptyForm();
    this.memberForm = this.emptyMemberForm();
    this.channelForm = this.emptyChannelForm();
    this.channelEditForm = this.emptyChannelEditForm();
    this.credentialForm = this.emptyCredentialForm();
    this.credentialValueForm = this.emptyCredentialValueForm();
    this.credentialMode = 'ref';
    this.showApiKey = false;
    this.showMtApiKey = false;
    this.showPsk = false;
    this.memberDataSource.data = [];
    this.channelDataSource.data = [];
    this.credentialDataSource.data = [];
    this.metadataText = '{}';
    this.selectedChannelId = '';
  }

  saveTenant(): void {
    if (!this.platformScoped) {
      this.toast('Platform scope is required to update tenant catalog records');
      return;
    }
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
    if (!this.platformScoped) {
      this.toast('Platform scope is required to manage tenant members');
      return;
    }
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
    if (!this.platformScoped) {
      this.toast('Platform scope is required to manage tenant members');
      return;
    }
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

  editChannel(channel: AdminChannel): void {
    this.editingChannelId = channel.channel_id;
    this.channelEditForm = {
      provider: channel.provider,
      country: channel.country,
      operator: channel.operator || '',
      capabilities: [...channel.capabilities]
    };
  }

  cancelChannelEdit(): void {
    this.editingChannelId = null;
    this.channelEditForm = this.emptyChannelEditForm();
  }

  saveChannelUpdate(): void {
    if (!this.editingChannelId) {
      return;
    }
    const payload: ChannelUpdatePayload = {};
    const p = this.channelEditForm.provider.trim().toLowerCase();
    if (p) {
      payload.provider = p;
    }
    const c = this.channelEditForm.country.trim().toUpperCase();
    if (c) {
      payload.country = c;
    }
    payload.operator = this.channelEditForm.operator.trim();
    if (this.channelEditForm.capabilities.length > 0) {
      payload.capabilities = [...this.channelEditForm.capabilities].sort();
    }

    if (!payload.provider && !payload.country && payload.operator === undefined && !payload.capabilities) {
      this.toast('No fields to update');
      return;
    }

    this.channelSaving = true;
    this.tenantService.updateChannel(this.editingChannelId, payload).subscribe({
      next: () => {
        this.channelSaving = false;
        this.editingChannelId = null;
        this.channelEditForm = this.emptyChannelEditForm();
        this.toast('Channel updated');
        this.loadChannels();
      },
      error: (err) => {
        this.channelSaving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to update channel'));
      }
    });
  }

  editCapabilitySelected(capability: string): boolean {
    return this.channelEditForm.capabilities.includes(capability);
  }

  toggleEditCapability(capability: string, checked: boolean): void {
    const next = new Set(this.channelEditForm.capabilities);
    if (checked) {
      next.add(capability);
    } else {
      next.delete(capability);
    }
    this.channelEditForm.capabilities = Array.from(next).sort();
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

  saveCredentialValue(): void {
    if (!this.selectedChannelId) {
      this.toast('Select a channel first');
      return;
    }
    const f = this.credentialValueForm;
    if (!f.base_url.trim() && !f.api_key.trim()) {
      this.toast('At least base_url or api_key is required');
      return;
    }

    const value: CredentialSecretValue = {};
    if (f.base_url.trim()) { value.base_url = f.base_url.trim(); }
    if (f.api_key.trim()) { value.api_key = f.api_key.trim(); }
    if (f.mt_api_key.trim()) { value.mt_api_key = f.mt_api_key.trim(); }
    if (f.psk.trim()) { value.psk = f.psk.trim(); }
    if (f.partner_service_id.trim()) { value.partner_service_id = f.partner_service_id.trim(); }
    if (f.partner_role_id.trim()) { value.partner_role_id = f.partner_role_id.trim(); }
    if (f.realm.trim()) { value.realm = f.realm.trim(); }
    if (f.mcc.trim()) { value.mcc = f.mcc.trim(); }
    if (f.mnc.trim()) { value.mnc = f.mnc.trim(); }
    if (f.large_account.trim()) { value.large_account = f.large_account.trim(); }
    if (f.service_name.trim()) { value.service_name = f.service_name.trim(); }
    if (f.free_mt_pricepoint_id.trim()) { value.free_mt_pricepoint_id = f.free_mt_pricepoint_id.trim(); }
    if (f.he_iv_param_spec_key.trim()) { value.he_iv_param_spec_key = f.he_iv_param_spec_key.trim(); }

    const moPpIds = this.parseIdList(f.mo_pricepoint_ids_text);
    if (moPpIds === null) {
      this.toast('mo_pricepoint_ids: each entry must be non-empty');
      return;
    }
    if (moPpIds.length > 0) { value.mo_pricepoint_ids = moPpIds; }

    const billingPpIds = this.parseIdList(f.billing_pricepoint_ids_text);
    if (billingPpIds === null) {
      this.toast('billing_pricepoint_ids: each entry must be non-empty');
      return;
    }
    if (billingPpIds.length > 0) { value.billing_pricepoint_ids = billingPpIds; }

    const purpose = f.purpose.trim() || 'provider_api';
    this.credentialSaving = true;
    this.tenantService.bindChannelCredentialValue(this.selectedChannelId, purpose, value).subscribe({
      next: () => {
        this.credentialSaving = false;
        this.credentialValueForm = this.emptyCredentialValueForm();
        this.showApiKey = false;
        this.showMtApiKey = false;
        this.showPsk = false;
        this.toast('Credential value bound');
        this.loadChannelCredentials(this.selectedChannelId);
      },
      error: (err) => {
        this.credentialSaving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to bind credential value'));
      }
    });
  }

  /** Parse a newline/comma-separated list of IDs; returns null on invalid entries. */
  parseIdList(raw: string): string[] | null {
    if (!raw.trim()) { return []; }
    const items = raw.split(/[\n,]+/).map((s) => s.trim()).filter((s) => s.length > 0);
    if (items.some((s) => s === '')) { return null; }
    return items;
  }

  revokeCredential(credential: AdminChannelCredential): void {
    if (!this.selectedChannelId || !credential.credential_id) {
      return;
    }
    if (credential.status === 'REVOKED') {
      this.toast('Credential is already revoked');
      return;
    }
    const confirmed = window.confirm(
      `Revoke credential "${credential.redacted_display}" (purpose: ${credential.purpose}, v${credential.version})?\n\nThis will permanently delete the stored ciphertext and cannot be undone.`
    );
    if (!confirmed) {
      return;
    }
    this.credentialRevoking = true;
    this.tenantService.revokeChannelCredential(this.selectedChannelId, credential.credential_id).subscribe({
      next: (result: RevokeCredentialResponse) => {
        this.credentialRevoking = false;
        const msg = result.was_only_active
          ? 'Credential revoked. Channel now has no active credential for this purpose.'
          : 'Credential revoked';
        this.toast(msg);
        this.loadChannelCredentials(this.selectedChannelId);
      },
      error: (err: any) => {
        this.credentialRevoking = false;
        if (err?.status === 409) {
          this.toast('Credential was already revoked');
          this.loadChannelCredentials(this.selectedChannelId);
        } else {
          this.toast(this.extractErrorMessage(err, 'Failed to revoke credential'));
        }
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

  private loadCurrentTenant(): void {
    this.loading = true;
    this.tenantService.current().subscribe({
      next: (tenant) => {
        this.dataSource.data = [tenant];
        this.totalCount = 1;
        this.loading = false;
        this.selectTenantForManagement(tenant);
      },
      error: (err) => {
        this.loading = false;
        this.dataSource.data = [];
        this.totalCount = 0;
        this.toast(this.extractErrorMessage(err, 'Failed to load current tenant'));
      }
    });
  }

  private applyWorkspace(workspace: TenantWorkspaceState): void {
    const currentTenant = workspace.currentTenant;
    const nextKey = [
      workspace.platformScoped ? 'platform' : 'tenant',
      workspace.status,
      currentTenant?.tenantId ?? '',
      currentTenant?.tenantKey ?? ''
    ].join(':');

    if (nextKey === this.workspaceKey) {
      return;
    }

    this.workspaceKey = nextKey;
    this.platformScoped = workspace.platformScoped;
    this.currentWorkspaceTenant = currentTenant;

    if (workspace.status !== 'ready') {
      return;
    }

    this.page = 1;
    this.loadTenants();
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

  private emptyChannelEditForm(): { provider: string; country: string; operator: string; capabilities: string[] } {
    return {
      provider: '',
      country: '',
      operator: '',
      capabilities: []
    };
  }

  private emptyCredentialForm(): { purpose: string; secret_ref: string } {
    return {
      purpose: 'provider_api',
      secret_ref: ''
    };
  }

  private emptyCredentialValueForm(): {
    purpose: string;
    base_url: string;
    api_key: string;
    mt_api_key: string;
    psk: string;
    partner_service_id: string;
    partner_role_id: string;
    realm: string;
    mcc: string;
    mnc: string;
    large_account: string;
    service_name: string;
    free_mt_pricepoint_id: string;
    mo_pricepoint_ids_text: string;
    billing_pricepoint_ids_text: string;
    he_iv_param_spec_key: string;
  } {
    return {
      purpose: 'provider_api',
      base_url: '',
      api_key: '',
      mt_api_key: '',
      psk: '',
      partner_service_id: '',
      partner_role_id: '',
      realm: '',
      mcc: '',
      mnc: '',
      large_account: '',
      service_name: '',
      free_mt_pricepoint_id: '',
      mo_pricepoint_ids_text: '',
      billing_pricepoint_ids_text: '',
      he_iv_param_spec_key: ''
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
