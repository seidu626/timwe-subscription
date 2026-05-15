import { inject } from '@angular/core';
import { ActivatedRouteSnapshot, CanActivateChildFn, Router, RouterStateSnapshot, UrlTree } from '@angular/router';
import { AuthService } from '@auth0/auth0-angular';
import { combineLatest } from 'rxjs';
import { filter, map, take } from 'rxjs/operators';
import { TenantWorkspaceService } from '../services/tenant-workspace.service';

function tenantIdentifierFromRoute(route: ActivatedRouteSnapshot): string | null {
  return route.queryParamMap.get('tenant')
    ?? route.queryParamMap.get('tenantKey')
    ?? route.queryParamMap.get('tenant_key')
    ?? route.queryParamMap.get('tenantId');
}

function routeRequiresPlatformScope(route: ActivatedRouteSnapshot): boolean {
  let current: ActivatedRouteSnapshot | null = route;

  while (current) {
    if (current.data?.['platformRequired'] === true) {
      return true;
    }
    current = current.parent;
  }

  return false;
}

function denialTree(router: Router, reason: string): UrlTree {
  return router.createUrlTree(['/403'], {
    queryParams: { reason }
  });
}

export const tenantWorkspaceGuard: CanActivateChildFn = (route: ActivatedRouteSnapshot, state: RouterStateSnapshot) => {
  const auth = inject(AuthService);
  const router = inject(Router);
  const tenantWorkspace = inject(TenantWorkspaceService);
  const requestedTenant = tenantIdentifierFromRoute(route);
  const platformRequired = routeRequiresPlatformScope(route);

  return combineLatest([
    auth.isLoading$,
    auth.isAuthenticated$,
    tenantWorkspace.workspace$
  ]).pipe(
    filter(([loading, , workspace]) => !loading && !workspace.loading),
    take(1),
    map(([, authenticated, workspace]) => {
      if (!authenticated) {
        sessionStorage.setItem('auth_redirect_url', state.url);
        return router.createUrlTree(['/login']);
      }

      if (requestedTenant) {
        if (!tenantWorkspace.selectTenant(requestedTenant)) {
          tenantWorkspace.clearTenantSelection();
          return denialTree(router, 'invalid-selection');
        }

        return true;
      }

      if (workspace.status === 'ready') {
        if (platformRequired && !workspace.platformScoped) {
          return denialTree(router, 'platform-required');
        }
        return true;
      }

      if (workspace.status === 'selection-required') {
        return denialTree(router, 'selection-required');
      }

      if (workspace.status === 'invalid-selection') {
        tenantWorkspace.clearTenantSelection();
        return denialTree(router, 'invalid-selection');
      }

      return denialTree(router, workspace.reason ?? 'missing-tenant');
    })
  );
};
