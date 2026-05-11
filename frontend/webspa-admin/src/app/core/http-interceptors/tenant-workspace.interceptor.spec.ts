import { HttpHandler, HttpRequest } from '@angular/common/http';
import { fakeAsync, tick } from '@angular/core/testing';
import { BehaviorSubject, of } from 'rxjs';

import { TenantWorkspaceInterceptor } from './tenant-workspace.interceptor';
import { TenantWorkspaceService, TenantWorkspaceState } from '../services/tenant-workspace.service';

describe('TenantWorkspaceInterceptor', () => {
  const loadingWorkspace: TenantWorkspaceState = {
    authenticated: false,
    loading: true,
    platformScoped: false,
    currentTenant: null,
    availableTenants: [],
    canSwitchTenant: false,
    status: 'loading',
    reason: null
  };

  const readyWorkspace: TenantWorkspaceState = {
    authenticated: true,
    loading: false,
    platformScoped: true,
    currentTenant: {
      identifier: 'nrg',
      tenantId: 'nrg',
      tenantKey: 'nrg',
      label: 'NRG'
    },
    availableTenants: [
      {
        identifier: 'nrg',
        tenantId: 'nrg',
        tenantKey: 'nrg',
        label: 'NRG'
      }
    ],
    canSwitchTenant: false,
    status: 'ready',
    reason: null
  };

  const unauthenticatedWorkspace: TenantWorkspaceState = {
    authenticated: false,
    loading: false,
    platformScoped: false,
    currentTenant: null,
    availableTenants: [],
    canSwitchTenant: false,
    status: 'unauthenticated',
    reason: null
  };

  it('waits for resolved workspace state before attaching tenant headers', fakeAsync(() => {
    const workspace$ = new BehaviorSubject<TenantWorkspaceState>(loadingWorkspace);
    const tenantWorkspace = {
      workspace$: workspace$.asObservable(),
      isWorkspaceRequest: jasmine.createSpy('isWorkspaceRequest').and.returnValue(true)
    } as unknown as TenantWorkspaceService;
    const interceptor = new TenantWorkspaceInterceptor(tenantWorkspace);
    const request = new HttpRequest('GET', 'http://localhost:8084/v1/admin/reports/kpis');
    const handledRequests: HttpRequest<unknown>[] = [];
    const next: HttpHandler = {
      handle: (req) => {
        handledRequests.push(req);
        return of({} as any);
      }
    };

    interceptor.intercept(request, next).subscribe();
    tick();

    expect(handledRequests.length).toBe(0);

    workspace$.next(readyWorkspace);
    tick();

    expect(handledRequests.length).toBe(1);
    expect(handledRequests[0].headers.get('X-Tenant-Key')).toBe('nrg');
  }));

  it('waits through transient unauthenticated state before attaching tenant headers', fakeAsync(() => {
    const workspace$ = new BehaviorSubject<TenantWorkspaceState>(loadingWorkspace);
    const tenantWorkspace = {
      workspace$: workspace$.asObservable(),
      isWorkspaceRequest: jasmine.createSpy('isWorkspaceRequest').and.returnValue(true)
    } as unknown as TenantWorkspaceService;
    const interceptor = new TenantWorkspaceInterceptor(tenantWorkspace);
    const request = new HttpRequest('GET', 'http://localhost:8084/v1/admin/reports/kpis');
    const handledRequests: HttpRequest<unknown>[] = [];
    const next: HttpHandler = {
      handle: (req) => {
        handledRequests.push(req);
        return of({} as any);
      }
    };

    interceptor.intercept(request, next).subscribe();
    tick();
    workspace$.next(unauthenticatedWorkspace);
    tick();

    expect(handledRequests.length).toBe(0);

    workspace$.next(readyWorkspace);
    tick();

    expect(handledRequests.length).toBe(1);
    expect(handledRequests[0].headers.get('X-Tenant-Key')).toBe('nrg');
  }));

  it('does not delay non-workspace requests', () => {
    const tenantWorkspace = {
      workspace$: of(loadingWorkspace),
      isWorkspaceRequest: jasmine.createSpy('isWorkspaceRequest').and.returnValue(false)
    } as unknown as TenantWorkspaceService;
    const interceptor = new TenantWorkspaceInterceptor(tenantWorkspace);
    const request = new HttpRequest('GET', '/assets/config.json');
    const handledRequests: HttpRequest<unknown>[] = [];
    const next: HttpHandler = {
      handle: (req) => {
        handledRequests.push(req);
        return of({} as any);
      }
    };

    interceptor.intercept(request, next).subscribe();

    expect(handledRequests).toEqual([request]);
  });
});
