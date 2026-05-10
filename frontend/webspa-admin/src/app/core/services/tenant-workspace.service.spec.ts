import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';

import { TenantWorkspaceService } from './tenant-workspace.service';

describe('TenantWorkspaceService', () => {
  beforeEach(() => {
    sessionStorage.clear();
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
});
