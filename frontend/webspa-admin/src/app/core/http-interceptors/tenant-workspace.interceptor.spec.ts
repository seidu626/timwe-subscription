import { HttpHandler, HttpRequest, HttpResponse } from '@angular/common/http';
import { Subject, firstValueFrom, of } from 'rxjs';

import { TenantWorkspaceInterceptor } from './tenant-workspace.interceptor';
import { TenantWorkspaceState } from '../services/tenant-workspace.service';

function workspaceState(partial: Partial<TenantWorkspaceState>): TenantWorkspaceState {
  return {
    authenticated: true,
    loading: false,
    platformScoped: false,
    currentTenant: null,
    availableTenants: [],
    canSwitchTenant: false,
    status: 'ready',
    reason: null,
    ...partial
  };
}

describe('TenantWorkspaceInterceptor', () => {
  it('waits for tenant workspace readiness before forwarding workspace requests', async () => {
    const workspace$ = new Subject<TenantWorkspaceState>();
    const tenantWorkspace = {
      isWorkspaceRequest: jasmine.createSpy('isWorkspaceRequest').and.returnValue(true),
      workspace$
    };
    const handler = {
      handle: jasmine.createSpy('handle').and.returnValue(of(new HttpResponse({ status: 200 })))
    } as jasmine.SpyObj<HttpHandler>;
    const interceptor = new TenantWorkspaceInterceptor(tenantWorkspace as any);
    const request = new HttpRequest('GET', 'http://localhost:8084/v1/admin/campaigns');

    const responsePromise = firstValueFrom(interceptor.intercept(request, handler));

    workspace$.next(workspaceState({ loading: true, status: 'loading' }));
    expect(handler.handle).not.toHaveBeenCalled();

    workspace$.next(workspaceState({
      currentTenant: {
        identifier: 'nrg',
        tenantId: 'tenant-nrg',
        tenantKey: 'nrg',
        label: 'NRG'
      },
      availableTenants: [
        {
          identifier: 'nrg',
          tenantId: 'tenant-nrg',
          tenantKey: 'nrg',
          label: 'NRG'
        }
      ]
    }));

    await responsePromise;

    expect(handler.handle).toHaveBeenCalledTimes(1);
    const forwarded = handler.handle.calls.mostRecent().args[0] as HttpRequest<unknown>;
    expect(forwarded.headers.get('X-Tenant-Key')).toBe('nrg');
    expect(forwarded.headers.get('X-Tenant-Id')).toBe('tenant-nrg');
  });

  it('does not wait for non-workspace requests', async () => {
    const workspace$ = new Subject<TenantWorkspaceState>();
    const tenantWorkspace = {
      isWorkspaceRequest: jasmine.createSpy('isWorkspaceRequest').and.returnValue(false),
      workspace$
    };
    const handler = {
      handle: jasmine.createSpy('handle').and.returnValue(of(new HttpResponse({ status: 200 })))
    } as jasmine.SpyObj<HttpHandler>;
    const interceptor = new TenantWorkspaceInterceptor(tenantWorkspace as any);
    const request = new HttpRequest('GET', '/assets/config.json');

    await firstValueFrom(interceptor.intercept(request, handler));

    expect(handler.handle).toHaveBeenCalledWith(request);
  });
});
