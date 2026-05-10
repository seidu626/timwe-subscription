import { TestBed } from '@angular/core/testing';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot, UrlTree, convertToParamMap } from '@angular/router';
import { AuthService } from '@auth0/auth0-angular';
import { firstValueFrom, of } from 'rxjs';

import { tenantWorkspaceGuard } from './tenant-workspace.guard';
import { TenantWorkspaceService } from '../services/tenant-workspace.service';

describe('tenantWorkspaceGuard', () => {
  beforeEach(() => {
    sessionStorage.clear();
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
        },
        {
          provide: Router,
          useValue: jasmine.createSpyObj<Router>('Router', ['navigate', 'createUrlTree'], { url: '/dashboard' })
        }
      ]
    });
  });

  it('routes tampered tenant URLs to the denial page', async () => {
    const router = TestBed.inject(Router) as jasmine.SpyObj<Router>;
    router.createUrlTree.and.returnValue({} as UrlTree);

    const route = {
      queryParamMap: convertToParamMap({ tenant: 'tenant-b' })
    } as ActivatedRouteSnapshot;
    const state = {
      url: '/dashboard?tenant=tenant-b'
    } as RouterStateSnapshot;

    const result$ = TestBed.runInInjectionContext(() => tenantWorkspaceGuard(route, state));
    await firstValueFrom(result$ as any);

    expect(router.createUrlTree).toHaveBeenCalledWith(['/403'], {
      queryParams: { reason: 'invalid-selection' }
    });
  });

  it('allows the assigned tenant without a query parameter', async () => {
    const route = {
      queryParamMap: convertToParamMap({})
    } as ActivatedRouteSnapshot;
    const state = {
      url: '/dashboard'
    } as RouterStateSnapshot;

    const result$ = TestBed.runInInjectionContext(() => tenantWorkspaceGuard(route, state));
    const result = await firstValueFrom(result$ as any);

    expect(result).toBeTrue();
    expect(TestBed.inject(TenantWorkspaceService).getCurrentWorkspace().status).toBe('ready');
  });
});
