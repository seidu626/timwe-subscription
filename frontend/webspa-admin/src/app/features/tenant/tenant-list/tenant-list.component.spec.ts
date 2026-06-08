import { of } from 'rxjs';

import { TenantListComponent } from './tenant-list.component';

describe('TenantListComponent', () => {
  function createComponent() {
    const tenantService = {
      list: jasmine.createSpy().and.returnValue(of({
        tenants: [],
        total_count: 0,
        page: 1,
        page_size: 20
      })),
      update: jasmine.createSpy().and.returnValue(of({
        id: 'tenant-1',
        tenant_key: 'nrg',
        name: 'NRG Prime',
        status: 'ACTIVE',
        default_country: 'GH',
        metadata: {},
        created_at: '2026-05-10T00:00:00Z',
        updated_at: '2026-05-10T00:00:00Z',
        audit_log_id: 'audit-1'
      })),
      create: jasmine.createSpy().and.returnValue(of({
        id: 'tenant-2',
        tenant_key: 'newco',
        name: 'NewCo',
        status: 'ACTIVE',
        default_country: 'GH',
        metadata: {},
        created_at: '2026-05-13T00:00:00Z',
        updated_at: '2026-05-13T00:00:00Z',
        audit_log_id: 'audit-2'
      })),
      listMembers: jasmine.createSpy().and.returnValue(of({
        members: [],
        total_count: 0,
        page: 1,
        page_size: 100
      })),
      upsertMember: jasmine.createSpy().and.returnValue(of({
        id: 'member-1',
        tenant_id: 'tenant-1',
        auth0_subject: 'google-oauth2|123',
        email: 'admin@example.com',
        role: 'TENANT_ADMIN',
        status: 'ACTIVE',
        created_at: '2026-05-13T00:00:00Z',
        updated_at: '2026-05-13T00:00:00Z',
        audit_log_id: 'audit-3'
      })),
      deactivateMember: jasmine.createSpy().and.returnValue(of({
        audit_log_id: 'audit-4'
      })),
      listChannels: jasmine.createSpy().and.returnValue(of({
        channels: [{
          channel_id: 'channel-1',
          tenant_id: 'tenant-1',
          channel_key: 'timwe-gh-airteltigo',
          provider: 'timwe',
          country: 'GH',
          operator: 'AirtelTigo',
          capabilities: ['confirm', 'mt', 'optin'],
          status: 'ACTIVE',
          enabled: true,
          created_at: '2026-05-13T00:00:00Z',
          updated_at: '2026-05-13T00:00:00Z'
        }],
        total_count: 1,
        page: 1,
        page_size: 100
      })),
      createChannel: jasmine.createSpy().and.returnValue(of({
        channel_id: 'channel-2',
        tenant_id: 'tenant-1',
        channel_key: 'timwe-gh-mtn',
        provider: 'timwe',
        country: 'GH',
        capabilities: ['mt', 'optin'],
        status: 'INACTIVE',
        enabled: false,
        created_at: '2026-05-13T00:00:00Z',
        updated_at: '2026-05-13T00:00:00Z'
      })),
      setChannelEnabled: jasmine.createSpy().and.returnValue(of({
        channel_id: 'channel-1',
        tenant_id: 'tenant-1',
        channel_key: 'timwe-gh-airteltigo',
        provider: 'timwe',
        country: 'GH',
        capabilities: ['confirm', 'mt', 'optin'],
        status: 'INACTIVE',
        enabled: false,
        created_at: '2026-05-13T00:00:00Z',
        updated_at: '2026-05-13T00:00:00Z'
      })),
      listChannelCredentials: jasmine.createSpy().and.returnValue(of({
        credentials: [],
        total_count: 0,
        page: 1,
        page_size: 20
      })),
      bindChannelCredential: jasmine.createSpy().and.returnValue(of({
        credential_id: 'credential-1',
        tenant_id: 'tenant-1',
        channel_id: 'channel-1',
        purpose: 'provider_api',
        version: 1,
        status: 'ACTIVE',
        redacted_display: 'vault://[REDACTED]',
        created_at: '2026-05-13T00:00:00Z',
        updated_at: '2026-05-13T00:00:00Z'
      }))
    };
    const snackBar = {
      open: jasmine.createSpy()
    };

    const component = new TenantListComponent(tenantService as any, snackBar as any);
    return { component, tenantService, snackBar };
  }

  it('loads tenant catalog rows with current paging and filters', () => {
    const { component, tenantService } = createComponent();
    component.filters = { q: 'nrg', status: 'ACTIVE' };
    component.page = 2;
    component.pageSize = 10;

    component.loadTenants();

    expect(tenantService.list).toHaveBeenCalledWith({
      page: 2,
      page_size: 10,
      q: 'nrg',
      status: 'ACTIVE'
    });
  });

  it('sends normalized tenant updates with JSON metadata', () => {
    const { component, tenantService } = createComponent();
    component.editTenant({
      id: 'tenant-1',
      tenant_key: 'nrg',
      name: 'NRG',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { kind: 'canonical-default' },
      created_at: '2026-05-10T00:00:00Z',
      updated_at: '2026-05-10T00:00:00Z'
    });
    component.form.name = ' NRG Prime ';
    component.form.default_country = 'gh';
    component.metadataText = '{"tier":"gold"}';

    component.saveTenant();

    expect(tenantService.update).toHaveBeenCalledWith('tenant-1', {
      name: 'NRG Prime',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { tier: 'gold' }
    });
    expect(tenantService.create).not.toHaveBeenCalled();
  });

  it('sends normalized tenant creates with JSON metadata', () => {
    const { component, tenantService } = createComponent();
    component.form = {
      tenant_key: ' NewCo ',
      name: ' NewCo ',
      status: 'ACTIVE',
      default_country: 'gh'
    };
    component.metadataText = '{"owner":"ops"}';

    component.saveTenant();

    expect(tenantService.create).toHaveBeenCalledWith({
      tenant_key: 'newco',
      name: 'NewCo',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { owner: 'ops' }
    });
    expect(tenantService.update).not.toHaveBeenCalled();
  });

  it('does not update when metadata is not a JSON object', () => {
    const { component, tenantService, snackBar } = createComponent();
    component.editingTenantId = 'tenant-1';
    component.form = { tenant_key: 'nrg', name: 'NRG', status: 'ACTIVE', default_country: 'GH' };
    component.metadataText = '[]';

    component.saveTenant();

    expect(tenantService.update).not.toHaveBeenCalled();
    expect(tenantService.create).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Metadata must be a JSON object', 'Close', { duration: 4000 });
  });

  it('creates channels with normalized provider, country, operator, and capabilities', () => {
    const { component, tenantService } = createComponent();
    component.channelForm = {
      provider: ' TIMWE ',
      country: 'gh',
      operator: ' AirtelTigo ',
      capabilities: ['mt', 'optin'],
      enabled: false
    };

    component.saveChannel();

    expect(tenantService.createChannel).toHaveBeenCalledWith({
      provider: 'timwe',
      country: 'GH',
      operator: 'AirtelTigo',
      capabilities: ['mt', 'optin'],
      enabled: false
    });
  });

  it('binds provider credentials by secret reference only', () => {
    const { component, tenantService } = createComponent();
    component.selectedChannelId = 'channel-1';
    component.credentialForm = {
      purpose: ' provider_api ',
      secret_ref: ' vault://tenant/channel/provider-api '
    };

    component.saveCredentialReference();

    expect(tenantService.bindChannelCredential).toHaveBeenCalledWith('channel-1', {
      purpose: 'provider_api',
      secret_ref: 'vault://tenant/channel/provider-api'
    });
  });
});
