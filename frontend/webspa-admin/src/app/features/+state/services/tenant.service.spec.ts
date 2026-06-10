import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

import { TenantService } from './tenant.service';

describe('TenantService', () => {
  let service: TenantService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        TenantService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });

    service = TestBed.inject(TenantService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('lists tenant catalog records with paging and filters', () => {
    let total = 0;

    service.list({ page: 2, page_size: 10, q: 'nrg', status: 'ACTIVE' }).subscribe((response) => {
      total = response.total_count;
    });

    const req = httpMock.expectOne((request) =>
      request.method === 'GET' &&
      request.url.endsWith('/v1/admin/tenants') &&
      request.params.get('page') === '2' &&
      request.params.get('page_size') === '10' &&
      request.params.get('q') === 'nrg' &&
      request.params.get('status') === 'ACTIVE'
    );

    req.flush({
      tenants: [],
      total_count: 3,
      page: 2,
      page_size: 10
    });

    expect(total).toBe(3);
  });

  it('updates tenant catalog records through PATCH', () => {
    service.update('tenant-1', { name: 'NRG Prime', status: 'ACTIVE', default_country: 'GH' }).subscribe();

    const req = httpMock.expectOne((request) =>
      request.method === 'PATCH' &&
      request.url.endsWith('/v1/admin/tenants/tenant-1')
    );

    expect(req.request.body).toEqual({
      name: 'NRG Prime',
      status: 'ACTIVE',
      default_country: 'GH'
    });
    req.flush({
      id: 'tenant-1',
      tenant_key: 'nrg',
      name: 'NRG Prime',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: {},
      created_at: '2026-05-10T00:00:00Z',
      updated_at: '2026-05-10T00:00:00Z',
      audit_log_id: 'audit-1'
    });
  });

  it('creates tenant catalog records through POST', () => {
    service.create({
      tenant_key: 'newco',
      name: 'NewCo',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { owner: 'ops' }
    }).subscribe();

    const req = httpMock.expectOne((request) =>
      request.method === 'POST' &&
      request.url.endsWith('/v1/admin/tenants')
    );

    expect(req.request.body).toEqual({
      tenant_key: 'newco',
      name: 'NewCo',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { owner: 'ops' }
    });
    req.flush({
      id: 'tenant-2',
      tenant_key: 'newco',
      name: 'NewCo',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { owner: 'ops' },
      created_at: '2026-05-13T00:00:00Z',
      updated_at: '2026-05-13T00:00:00Z',
      audit_log_id: 'audit-2'
    });
  });
  it('binds channel credential via secret_ref through POST', () => {
    service.bindChannelCredential('channel-1', {
      purpose: 'provider_api',
      secret_ref: 'vault://nrg/gh/provider-api'
    }).subscribe();

    const req = httpMock.expectOne((request) =>
      request.method === 'POST' &&
      request.url.endsWith('/v1/admin/channels/channel-1/credentials')
    );

    expect(req.request.body).toEqual({
      purpose: 'provider_api',
      secret_ref: 'vault://nrg/gh/provider-api'
    });
    req.flush({
      credential_id: 'cred-1',
      tenant_id: 'tenant-1',
      channel_id: 'channel-1',
      purpose: 'provider_api',
      version: 1,
      status: 'ACTIVE',
      redacted_display: 'vault://nrg/gh/[REDACTED]',
      created_at: '2026-06-10T00:00:00Z',
      updated_at: '2026-06-10T00:00:00Z'
    });
  });

  it('serializes credential blob to JSON and POSTs as secret_value via bindChannelCredentialValue', () => {
    const blob = {
      base_url: 'https://api.timwe.example',
      api_key: 'key-abc',
      mt_api_key: 'mtkey-xyz',
      partner_service_id: 'svc-1',
      mo_pricepoint_ids: ['pp-mo-1', 'pp-mo-2']
    };

    service.bindChannelCredentialValue('channel-1', 'provider_api', blob).subscribe();

    const req = httpMock.expectOne((request) =>
      request.method === 'POST' &&
      request.url.endsWith('/v1/admin/channels/channel-1/credentials')
    );

    expect(req.request.body).toEqual({
      purpose: 'provider_api',
      secret_value: JSON.stringify(blob)
    });
    req.flush({
      credential_id: 'cred-2',
      tenant_id: 'tenant-1',
      channel_id: 'channel-1',
      purpose: 'provider_api',
      version: 2,
      status: 'ACTIVE',
      redacted_display: '[encrypted blob v2]',
      created_at: '2026-06-10T00:00:00Z',
      updated_at: '2026-06-10T00:00:00Z'
    });
  });

  it('lists channel credentials with purpose filter', () => {
    let result: any;
    service.listChannelCredentials('channel-1', 'provider_api').subscribe((r) => {
      result = r;
    });

    const req = httpMock.expectOne((request) =>
      request.method === 'GET' &&
      request.url.endsWith('/v1/admin/channels/channel-1/credentials') &&
      request.params.get('purpose') === 'provider_api'
    );

    req.flush({
      credentials: [{
        credential_id: 'cred-1',
        tenant_id: 'tenant-1',
        channel_id: 'channel-1',
        purpose: 'provider_api',
        version: 1,
        status: 'ACTIVE',
        redacted_display: 'vault://[REDACTED]',
        created_at: '2026-06-10T00:00:00Z',
        updated_at: '2026-06-10T00:00:00Z'
      }],
      total_count: 1,
      page: 1,
      page_size: 20
    });

    expect(result.total_count).toBe(1);
    expect(result.credentials[0].version).toBe(1);
  });
});
