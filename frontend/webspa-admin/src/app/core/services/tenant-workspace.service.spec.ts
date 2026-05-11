import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';

import { TenantWorkspaceService } from './tenant-workspace.service';

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
});
