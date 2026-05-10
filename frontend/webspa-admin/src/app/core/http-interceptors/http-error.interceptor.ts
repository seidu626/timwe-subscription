import { Injectable, Injector, ErrorHandler } from '@angular/core';
import {
  HttpEvent,
  HttpInterceptor,
  HttpHandler,
  HttpRequest,
  HttpErrorResponse
} from '@angular/common/http';
import { Observable } from 'rxjs';
import { tap, retry, take } from 'rxjs/operators';
import { Router } from '@angular/router';
import { AuthService } from '@auth0/auth0-angular';
import { TenantWorkspaceService } from '../services/tenant-workspace.service';
import { environment } from '../../../environments/environment';

/**
 * HTTP Error Interceptor
 * Handles HTTP errors and integrates with Auth0 authentication.
 * - On 401 errors, redirects unauthenticated users to login
 * - Other errors are passed to the application error handler
 */
@Injectable()
export class HttpErrorInterceptor implements HttpInterceptor {
  constructor(
    private injector: Injector, 
    private router: Router,
    private auth: AuthService,
    private tenantWorkspace: TenantWorkspaceService
  ) { }

  intercept(
    request: HttpRequest<any>,
    next: HttpHandler
  ): Observable<HttpEvent<any>> {
    const request$ = this.shouldRetry(request.method)
      ? next.handle(request).pipe(retry(1))
      : next.handle(request);

    return request$.pipe(
      tap({
        error: (err: any) => {
          if (err instanceof HttpErrorResponse) {
            if (err.status === 401) {
              // Check if user is authenticated before redirecting
              this.auth.isAuthenticated$.pipe(take(1)).subscribe(isAuthenticated => {
                if (!isAuthenticated) {
                  // Store current URL for redirect after login
                  sessionStorage.setItem('auth_redirect_url', this.router.url);
                  this.router.navigate(['login']);
                }
                // If authenticated but got 401, the token might be invalid
                // Auth0 will handle token refresh automatically
              });
            } else if ((err.status === 403 || err.status === 404) && this.isWorkspaceRequest(request.url)) {
              this.tenantWorkspace.clearTenantSelection();

              if (!this.router.url.startsWith('/403')) {
                this.router.navigate(['/403'], {
                  queryParams: {
                    reason: err.status === 403 ? 'forbidden' : 'tenant-not-found'
                  }
                });
              }
            }
            const appErrorHandler = this.injector.get(ErrorHandler);
            appErrorHandler.handleError(err);
          }
        }
      })
    );
  }

  private shouldRetry(method: string): boolean {
    const normalizedMethod = method.toUpperCase();
    return normalizedMethod === 'GET' || normalizedMethod === 'HEAD' || normalizedMethod === 'OPTIONS';
  }

  private isWorkspaceRequest(url: string): boolean {
    return [
      environment.baseApiEndpoint,
      environment.subscriptionApiEndpoint,
      environment.subscriptionExternalAdminApiEndpoint,
      environment.notificationApiEndpoint,
      environment.acquisitionApiEndpoint,
      environment.cadenceEngineEndpoint
    ].some((baseUrl) => baseUrl && url.startsWith(baseUrl)) || url.includes('/api/');
  }
}
