import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';
import { HttpClientTestingModule, HttpTestingController } from '@angular/common/http/testing';

import { TenantWorkspaceService } from './tenant-workspace.service';
import { environment } from '../../../environments/environment';

describe('TenantWorkspaceService', () => {
  beforeEach(() => {
    sessionStorage.clear();
    delete (window as unknown as Record<string, unknown>)['__ADMIN_TENANT_BOOTSTRAP__'];
  });

  it('resolves a single assigned tenant for a tenant admin', () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              tenant_id: 'tenant-a',
              tenant_key: 'tenant-a',
              name: 'Tenant Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('ready');
    expect(workspace.currentTenant?.tenantKey).toBe('tenant-a');
    expect(workspace.canSwitchTenant).toBeFalse();
  });

  it('requires an explicit tenant selection for platform scoped users with multiple tenants', () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              permissions: ['platform:all_tenants'],
              tenants: [
                { tenant_key: 'tenant-a', tenant_id: 'tenant-a', name: 'Tenant A' },
                { tenant_key: 'tenant-b', tenant_id: 'tenant-b', name: 'Tenant B' }
              ]
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const initialWorkspace = service.getCurrentWorkspace();

    expect(initialWorkspace.status).toBe('selection-required');
    expect(initialWorkspace.currentTenant).toBeNull();
    expect(initialWorkspace.canSwitchTenant).toBeTrue();

    expect(service.selectTenant('tenant-b')).toBeTrue();

    const selectedWorkspace = service.getCurrentWorkspace();
    expect(selectedWorkspace.status).toBe('ready');
    expect(selectedWorkspace.currentTenant?.tenantKey).toBe('tenant-b');
  });

  it('maps configured bootstrap admin emails to platform tenant workspaces', () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'almauricin@gmail.com',
              email_verified: true,
              name: 'Bootstrap Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('ready');
    expect(workspace.platformScoped).toBeTrue();
    expect(workspace.currentTenant?.tenantKey).toBe('nrg');
    expect(workspace.availableTenants.map((tenant) => tenant.tenantKey)).toContain('nrg');
  });

  it('maps configured bootstrap admin emails when email_verified is unavailable', () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'almauricin@gmail.com',
              name: 'Bootstrap Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('ready');
    expect(workspace.platformScoped).toBeTrue();
    expect(workspace.currentTenant?.tenantKey).toBe('nrg');
  });

  it('maps configured runtime bootstrap workspaces when email_verified is unavailable', () => {
    (window as unknown as Record<string, unknown>)['__ADMIN_TENANT_BOOTSTRAP__'] = {
      platformAdminEmails: ['bootstrap@example.com'],
      tenantWorkspaces: [
        { tenant_key: 'tenant-runtime', tenant_id: 'tenant-runtime-id', name: 'Runtime Tenant' }
      ]
    };
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'bootstrap@example.com',
              name: 'Bootstrap Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('ready');
    expect(workspace.platformScoped).toBeTrue();
    expect(workspace.currentTenant?.tenantKey).toBe('tenant-runtime');
    expect(workspace.currentTenant?.tenantId).toBe('tenant-runtime-id');
  });

  it('maps configured runtime bootstrap subjects when the access token has no email claim', () => {
    (window as unknown as Record<string, unknown>)['__ADMIN_TENANT_BOOTSTRAP__'] = {
      platformAdminSubjects: ['google-oauth2|platform-admin'],
      tenantWorkspaces: [
        { tenant_key: 'tenant-runtime', tenant_id: 'tenant-runtime-id', name: 'Runtime Tenant' }
      ]
    };
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              sub: 'google-oauth2|platform-admin',
              name: 'Bootstrap Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('ready');
    expect(workspace.platformScoped).toBeTrue();
    expect(workspace.currentTenant?.tenantKey).toBe('tenant-runtime');
    expect(workspace.currentTenant?.tenantId).toBe('tenant-runtime-id');
  });

  it('maps bootstrap admin emails from user metadata case-insensitively', () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              app_metadata: {
                email: 'SEIDU.ABDULAI@HOTMAIL.COM',
                email_verified: true
              },
              name: 'Bootstrap Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('ready');
    expect(workspace.platformScoped).toBeTrue();
    expect(workspace.currentTenant?.tenantKey).toBe('nrg');
  });

  it('requires selection when a bootstrap admin has multiple runtime tenant workspaces', () => {
    (window as unknown as Record<string, unknown>)['__ADMIN_TENANT_BOOTSTRAP__'] = {
      platformAdminEmails: ['seidu.abdulai@hotmail.com'],
      tenantWorkspaces: [
        { tenant_key: 'tenant-a', tenant_id: 'tenant-a', name: 'Tenant A' },
        { tenant_key: 'tenant-b', tenant_id: 'tenant-b', name: 'Tenant B' }
      ]
    };
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'seidu.abdulai@hotmail.com',
              email_verified: true,
              name: 'Bootstrap Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('selection-required');
    expect(workspace.platformScoped).toBeTrue();
    expect(workspace.canSwitchTenant).toBeTrue();
    expect(workspace.currentTenant).toBeNull();
    expect(service.selectTenant('tenant-b')).toBeTrue();
    expect(service.getCurrentWorkspace().status).toBe('ready');
    expect(service.getCurrentWorkspace().currentTenant?.tenantKey).toBe('tenant-b');
  });

  it('does not bootstrap an unverified listed email', () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'almauricin@gmail.com',
              email_verified: false,
              name: 'Bootstrap Admin'
            })
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const workspace = service.getCurrentWorkspace();

    expect(workspace.status).toBe('missing-tenant');
    expect(workspace.platformScoped).toBeFalse();
  });

  it('loads backend tenant workspaces with an Auth0 bearer token outside interceptors', () => {
    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule],
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'tenant.admin@example.com',
              email_verified: true,
              name: 'Tenant Admin'
            }),
            getAccessTokenSilently: jasmine.createSpy('getAccessTokenSilently').and.returnValue(of('workspace-token'))
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const http = TestBed.inject(HttpTestingController);
    const req = http.expectOne(`${environment.acquisitionApiEndpoint}/v1/admin/tenants/workspaces`);

    expect(service.getCurrentWorkspace().loading).toBeTrue();

    expect(req.request.headers.get('Authorization')).toBe('Bearer workspace-token');
    req.flush({
      platform_scoped: false,
      tenants: [
        {
          id: 'tenant-a',
          tenant_key: 'tenant-a',
          name: 'Tenant A'
        }
      ]
    });
    http.verify();

    const workspace = service.getCurrentWorkspace();
    expect(workspace.status).toBe('ready');
    expect(workspace.currentTenant?.tenantKey).toBe('tenant-a');
  });

  it('waits for the backend workspace response before marking users without claim tenants as missing tenant', () => {
    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule],
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'runtime.tenant@example.com',
              email_verified: true,
              name: 'Runtime Tenant Admin'
            }),
            getAccessTokenSilently: jasmine.createSpy('getAccessTokenSilently').and.returnValue(of('workspace-token'))
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const http = TestBed.inject(HttpTestingController);
    const req = http.expectOne(`${environment.acquisitionApiEndpoint}/v1/admin/tenants/workspaces`);

    expect(service.getCurrentWorkspace().status).toBe('loading');

    req.flush({
      platform_scoped: false,
      tenants: [
        {
          id: '66d39a9a-f1ef-4721-a31c-5bb966d25c3d',
          tenant_key: 'nrg',
          name: 'NRG'
        }
      ]
    });
    http.verify();

    const workspace = service.getCurrentWorkspace();
    expect(workspace.status).toBe('ready');
    expect(workspace.currentTenant?.tenantKey).toBe('nrg');
  });

  it('requires selection for non-platform users assigned to multiple backend tenant workspaces', () => {
    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule],
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'multi.tenant@example.com',
              email_verified: true,
              name: 'Multi Tenant Admin'
            }),
            getAccessTokenSilently: jasmine.createSpy('getAccessTokenSilently').and.returnValue(of('workspace-token'))
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const http = TestBed.inject(HttpTestingController);
    const req = http.expectOne(`${environment.acquisitionApiEndpoint}/v1/admin/tenants/workspaces`);

    req.flush({
      platform_scoped: false,
      tenants: [
        {
          id: '66d39a9a-f1ef-4721-a31c-5bb966d25c3d',
          tenant_key: 'nrg',
          name: 'NRG'
        },
        {
          id: 'dded2b2a-0a76-43c3-8a2b-5106bc07911f',
          tenant_key: 'careerify',
          name: 'Careerify'
        }
      ]
    });
    http.verify();

    const workspace = service.getCurrentWorkspace();
    expect(workspace.status).toBe('selection-required');
    expect(workspace.platformScoped).toBeFalse();
    expect(workspace.canSwitchTenant).toBeTrue();
    expect(workspace.currentTenant).toBeNull();

    expect(service.selectTenant('careerify')).toBeTrue();
    const selectedWorkspace = service.getCurrentWorkspace();
    expect(selectedWorkspace.status).toBe('ready');
    expect(selectedWorkspace.currentTenant?.tenantKey).toBe('careerify');
  });

  // Regression: the backend workspace fetch only ran on auth transitions, so a
  // single failed call pinned the workspace on missing-tenant until the user
  // signed out and back in.
  it('recovers from a failed backend workspace fetch via refreshWorkspace', () => {
    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule],
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'tenant.admin@example.com',
              email_verified: true,
              name: 'Tenant Admin'
            }),
            getAccessTokenSilently: jasmine.createSpy('getAccessTokenSilently').and.returnValue(of('workspace-token'))
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const http = TestBed.inject(HttpTestingController);

    http.expectOne(`${environment.acquisitionApiEndpoint}/v1/admin/tenants/workspaces`)
      .flush('unavailable', { status: 503, statusText: 'Service Unavailable' });
    expect(service.getCurrentWorkspace().status).toBe('missing-tenant');

    service.refreshWorkspace();
    expect(service.getCurrentWorkspace().status).toBe('loading');

    http.expectOne(`${environment.acquisitionApiEndpoint}/v1/admin/tenants/workspaces`).flush({
      platform_scoped: false,
      tenants: [
        {
          id: '66d39a9a-f1ef-4721-a31c-5bb966d25c3d',
          tenant_key: 'nrg',
          name: 'NRG'
        }
      ]
    });
    http.verify();

    const workspace = service.getCurrentWorkspace();
    expect(workspace.status).toBe('ready');
    expect(workspace.currentTenant?.tenantKey).toBe('nrg');
  });

  it('starts an interactive login when the session can no longer mint tokens silently', () => {
    const loginWithRedirect = jasmine.createSpy('loginWithRedirect');

    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule],
      providers: [
        {
          provide: AuthService,
          useValue: {
            isLoading$: of(false),
            isAuthenticated$: of(true),
            user$: of({
              email: 'tenant.admin@example.com',
              email_verified: true,
              name: 'Tenant Admin'
            }),
            getAccessTokenSilently: jasmine.createSpy('getAccessTokenSilently')
              .and.returnValue(throwError(() => ({ error: 'login_required' }))),
            loginWithRedirect
          }
        }
      ]
    });

    const service = TestBed.inject(TenantWorkspaceService);
    const http = TestBed.inject(HttpTestingController);
    http.verify();

    expect(loginWithRedirect).toHaveBeenCalled();
    expect(service.getCurrentWorkspace().status).toBe('missing-tenant');
  });
});
