import { BehaviorSubject, of } from 'rxjs';

import { TenantListComponent } from './tenant-list.component';
import { TenantWorkspaceState } from '../../../core/services/tenant-workspace.service';

describe('TenantListComponent', () => {
  const platformWorkspace: TenantWorkspaceState = {
    authenticated: true,
    loading: false,
    platformScoped: true,
    currentTenant: {
      identifier: 'nrg',
      tenantId: 'tenant-1',
      tenantKey: 'nrg',
      label: 'NRG'
    },
    availableTenants: [
      {
        identifier: 'nrg',
        tenantId: 'tenant-1',
        tenantKey: 'nrg',
        label: 'NRG'
      }
    ],
    canSwitchTenant: false,
    status: 'ready',
    reason: null
  };

  function createComponent(workspace: TenantWorkspaceState = platformWorkspace) {
    const currentTenant = {
      id: 'tenant-1',
      tenant_key: 'nrg',
      name: 'NRG',
      status: 'ACTIVE' as const,
      default_country: 'GH',
      metadata: {},
      created_at: '2026-05-10T00:00:00Z',
      updated_at: '2026-05-10T00:00:00Z'
    };
    const tenantService = {
      list: jasmine.createSpy().and.returnValue(of({
        tenants: [],
        total_count: 0,
        page: 1,
        page_size: 20
      })),
      current: jasmine.createSpy().and.returnValue(of(currentTenant)),
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
    // bindChannelCredentialValue delegates to bindChannelCredential after JSON serialization
    (tenantService as any).bindChannelCredentialValue = (channelId: string, purpose: string, value: object) => {
      return tenantService.bindChannelCredential(channelId, {
        purpose,
        secret_value: JSON.stringify(value)
      });
    };
    const snackBar = {
      open: jasmine.createSpy()
    };
    const workspace$ = new BehaviorSubject<TenantWorkspaceState>(workspace);
    const tenantWorkspace = {
      workspace$: workspace$.asObservable()
    };

    const component = new TenantListComponent(tenantService as any, snackBar as any, tenantWorkspace as any);
    return { component, tenantService, snackBar, workspace$ };
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

  it('loads the current tenant and provider setup in tenant workspace mode', () => {
    const tenantWorkspace: TenantWorkspaceState = {
      ...platformWorkspace,
      platformScoped: false,
      currentTenant: {
        identifier: 'nrg',
        tenantId: 'tenant-1',
        tenantKey: 'nrg',
        label: 'NRG'
      },
      status: 'ready'
    };
    const { component, tenantService } = createComponent(tenantWorkspace);

    component.ngOnInit();

    expect(tenantService.current).toHaveBeenCalled();
    expect(tenantService.list).not.toHaveBeenCalled();
    expect(component.editingTenantId).toBe('tenant-1');
    expect(component.form.tenant_key).toBe('nrg');
    expect(tenantService.listMembers).not.toHaveBeenCalled();
    expect(tenantService.listChannels).toHaveBeenCalledWith({ page: 1, page_size: 100 });
  });

  it('does not send tenant catalog updates from tenant workspace mode', () => {
    const tenantWorkspace: TenantWorkspaceState = {
      ...platformWorkspace,
      platformScoped: false,
      status: 'ready'
    };
    const { component, tenantService, snackBar } = createComponent(tenantWorkspace);
    component.platformScoped = false;
    component.editingTenantId = 'tenant-1';
    component.form = { tenant_key: 'nrg', name: 'NRG', status: 'ACTIVE', default_country: 'GH' };

    component.saveTenant();

    expect(tenantService.update).not.toHaveBeenCalled();
    expect(tenantService.create).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Platform scope is required to update tenant catalog records', 'Close', { duration: 4000 });
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

  it('rejects saveCredentialReference when no channel is selected', () => {
    const { component, tenantService, snackBar } = createComponent();
    component.selectedChannelId = '';
    component.credentialForm = { purpose: 'provider_api', secret_ref: 'vault://x' };

    component.saveCredentialReference();

    expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Select a channel first', 'Close', { duration: 4000 });
  });

  it('rejects saveCredentialReference when secret_ref is empty', () => {
    const { component, tenantService, snackBar } = createComponent();
    component.selectedChannelId = 'channel-1';
    component.credentialForm = { purpose: 'provider_api', secret_ref: '   ' };

    component.saveCredentialReference();

    expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Credential secret reference is required', 'Close', { duration: 4000 });
  });

  it('binds direct credential value as serialized JSON via bindChannelCredentialValue', () => {
    const { component, tenantService } = createComponent();
    component.selectedChannelId = 'channel-1';
    component.credentialValueForm = {
      purpose: 'provider_api',
      base_url: ' https://api.example.com ',
      api_key: 'key-abc',
      mt_api_key: 'mtkey-xyz',
      psk: 'psk-secret',
      partner_service_id: 'svc-1',
      partner_role_id: 'role-1',
      realm: 'gh',
      mcc: '620',
      mnc: '01',
      large_account: 'la-001',
      service_name: 'newsub',
      free_mt_pricepoint_id: 'pp-free',
      mo_pricepoint_ids_text: 'pp-mo-1\npp-mo-2',
      billing_pricepoint_ids_text: 'pp-bill-1',
      he_iv_param_spec_key: 'iv-key'
    };

    component.saveCredentialValue();

    // Key order matches saveCredentialValue(): scalar fields first, list fields last
    const expectedBlob = {
      base_url: 'https://api.example.com',
      api_key: 'key-abc',
      mt_api_key: 'mtkey-xyz',
      psk: 'psk-secret',
      partner_service_id: 'svc-1',
      partner_role_id: 'role-1',
      realm: 'gh',
      mcc: '620',
      mnc: '01',
      large_account: 'la-001',
      service_name: 'newsub',
      free_mt_pricepoint_id: 'pp-free',
      he_iv_param_spec_key: 'iv-key',
      mo_pricepoint_ids: ['pp-mo-1', 'pp-mo-2'],
      billing_pricepoint_ids: ['pp-bill-1']
    };
    expect(tenantService.bindChannelCredential).toHaveBeenCalledWith('channel-1', {
      purpose: 'provider_api',
      secret_value: JSON.stringify(expectedBlob)
    });
  });

  it('omits empty optional fields from the credential value blob', () => {
    const { component, tenantService } = createComponent();
    component.selectedChannelId = 'channel-1';
    component.credentialValueForm = {
      purpose: 'provider_api',
      base_url: 'https://api.example.com',
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

    component.saveCredentialValue();

    const call = (tenantService.bindChannelCredential as jasmine.Spy).calls.mostRecent();
    const body = call.args[1] as { purpose: string; secret_value: string };
    const parsed = JSON.parse(body.secret_value);
    expect(Object.keys(parsed)).toEqual(['base_url']);
    expect(parsed.api_key).toBeUndefined();
    expect(parsed.mo_pricepoint_ids).toBeUndefined();
  });

  it('rejects saveCredentialValue when no channel is selected', () => {
    const { component, tenantService, snackBar } = createComponent();
    component.selectedChannelId = '';

    component.saveCredentialValue();

    expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Select a channel first', 'Close', { duration: 4000 });
  });

  it('rejects saveCredentialValue when both base_url and api_key are empty', () => {
    const { component, tenantService, snackBar } = createComponent();
    component.selectedChannelId = 'channel-1';
    component.credentialValueForm = {
      purpose: 'provider_api',
      base_url: '  ',
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

    component.saveCredentialValue();

    expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('At least base_url or api_key is required', 'Close', { duration: 4000 });
  });

  describe('parseIdList', () => {
    it('parses newline-separated IDs', () => {
      const { component } = createComponent();
      expect(component.parseIdList('a\nb\nc')).toEqual(['a', 'b', 'c']);
    });

    it('parses comma-separated IDs', () => {
      const { component } = createComponent();
      expect(component.parseIdList('a,b,c')).toEqual(['a', 'b', 'c']);
    });

    it('trims surrounding whitespace from each entry', () => {
      const { component } = createComponent();
      expect(component.parseIdList('  pp-1  \n  pp-2  ')).toEqual(['pp-1', 'pp-2']);
    });

    it('returns empty array for blank input', () => {
      const { component } = createComponent();
      expect(component.parseIdList('   ')).toEqual([]);
    });

    it('returns empty array for empty string', () => {
      const { component } = createComponent();
      expect(component.parseIdList('')).toEqual([]);
    });

    it('filters out blank lines between entries', () => {
      const { component } = createComponent();
      expect(component.parseIdList('a\n\nb')).toEqual(['a', 'b']);
    });
  });

  describe('SMS gateway credential', () => {
    it('binds the gateway blob with the api key carried in headers', () => {
      const { component, tenantService } = createComponent();
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'sms_api';
      component.smsGatewayForm = {
        url: ' https://sms.arkesel.com/api/v2/sms/send ',
        method: 'post',
        api_key: ' gateway-key ',
        api_key_header: 'api-key',
        body_template: '{"sender":"{{sender}}","message":"{{text}}","recipients":["{{msisdn}}"]}',
        sender_id: 'Dayline',
        message_template: 'Your code is {{code}}.',
        success_field: 'status',
        success_value: 'success'
      };

      component.saveCredentialValue();

      const call = (tenantService.bindChannelCredential as jasmine.Spy).calls.mostRecent();
      expect(call.args[1].purpose).toBe('sms_api');
      expect(JSON.parse(call.args[1].secret_value)).toEqual({
        url: 'https://sms.arkesel.com/api/v2/sms/send',
        method: 'POST',
        headers: { 'api-key': 'gateway-key' },
        body_template: '{"sender":"{{sender}}","message":"{{text}}","recipients":["{{msisdn}}"]}',
        sender_id: 'Dayline',
        message_template: 'Your code is {{code}}.',
        success_field: 'status',
        success_value: 'success'
      });
    });

    it('rejects a message that would send without the code in it', () => {
      const { component, tenantService } = createComponent();
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'sms_api';
      component.smsGatewayForm.api_key = 'gateway-key';
      component.smsGatewayForm.message_template = 'Welcome to Dayline';

      component.saveCredentialValue();

      expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    });

    it('rejects a gateway that carries the message in neither the body nor the URL', () => {
      const { component, tenantService } = createComponent();
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'sms_api';
      component.smsGatewayForm.body_template = '';
      component.smsGatewayForm.url = 'https://sms.example.com/send';

      component.saveCredentialValue();

      expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    });

    it('rejects half a success marker', () => {
      const { component, tenantService } = createComponent();
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'sms_api';
      component.smsGatewayForm.success_value = '';

      component.saveCredentialValue();

      expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    });
  });

  describe('delegated OTP credential', () => {
    it('binds the provider blob after the operator confirms the mode switch', () => {
      const { component, tenantService } = createComponent();
      spyOn(window, 'confirm').and.returnValue(true);
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'otp_api';
      component.otpGatewayForm = {
        generate_url: 'https://sms.arkesel.com/api/otp/generate',
        verify_url: 'https://sms.arkesel.com/api/otp/verify',
        api_key: ' main-key ',
        api_key_header: 'api-key',
        sender_id: 'Dayline',
        message_template: 'Your code is %otp_code%.',
        length: 6,
        expiry_minutes: 5,
        medium: 'sms',
        type: 'numeric'
      };

      component.saveCredentialValue();

      const call = (tenantService.bindChannelCredential as jasmine.Spy).calls.mostRecent();
      expect(call.args[1].purpose).toBe('otp_api');
      expect(JSON.parse(call.args[1].secret_value)).toEqual({
        generate_url: 'https://sms.arkesel.com/api/otp/generate',
        verify_url: 'https://sms.arkesel.com/api/otp/verify',
        headers: { 'api-key': 'main-key' },
        sender_id: 'Dayline',
        message_template: 'Your code is %otp_code%.',
        length: 6,
        expiry_minutes: 5,
        medium: 'sms',
        type: 'numeric'
      });
    });

    // Binding switches where login codes come from for every user of the
    // tenant, so declining the prompt must leave the tenant untouched.
    it('binds nothing when the operator declines the mode switch', () => {
      const { component, tenantService } = createComponent();
      spyOn(window, 'confirm').and.returnValue(false);
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'otp_api';
      component.otpGatewayForm.api_key = 'main-key';

      component.saveCredentialValue();

      expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    });

    it('rejects a blob the provider would reject, before any confirmation', () => {
      const { component, tenantService } = createComponent();
      const confirmSpy = spyOn(window, 'confirm').and.returnValue(true);
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'otp_api';
      component.otpGatewayForm.api_key = 'main-key';
      component.otpGatewayForm.sender_id = 'TwelveCharsX';

      component.saveCredentialValue();

      expect(confirmSpy).not.toHaveBeenCalled();
      expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    });

    it('rejects an expiry the provider does not support', () => {
      const { component, tenantService } = createComponent();
      spyOn(window, 'confirm').and.returnValue(true);
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'otp_api';
      component.otpGatewayForm.api_key = 'main-key';
      component.otpGatewayForm.expiry_minutes = 30;

      component.saveCredentialValue();

      expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    });

    it('requires an api key, which the provider needs on every call', () => {
      const { component, tenantService } = createComponent();
      component.selectedChannelId = 'channel-1';
      component.credentialValueForm.purpose = 'otp_api';
      component.otpGatewayForm.api_key = '';

      component.saveCredentialValue();

      expect(tenantService.bindChannelCredential).not.toHaveBeenCalled();
    });
  });
});
