import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from '@auth0/auth0-angular';
import { map, take } from 'rxjs/operators';

/**
 * Auth0 authentication guard for protecting routes.
 * Redirects unauthenticated users to the login page.
 */
export const authGuard: CanActivateFn = (route, state) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return auth.isAuthenticated$.pipe(
    take(1),
    map(isAuthenticated => {
      if (isAuthenticated) {
        return true;
      }
      
      // Store the attempted URL for redirecting after login
      sessionStorage.setItem('auth_redirect_url', state.url);
      
      // Redirect to login page
      return router.createUrlTree(['/login']);
    })
  );
};

/**
 * Guard that redirects authenticated users away from public pages (e.g., login).
 * Use this to prevent logged-in users from seeing the login page.
 */
export const publicOnlyGuard: CanActivateFn = (route, state) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return auth.isAuthenticated$.pipe(
    take(1),
    map(isAuthenticated => {
      if (!isAuthenticated) {
        return true;
      }
      
      // Redirect authenticated users to dashboard
      return router.createUrlTree(['/dashboard']);
    })
  );
};
