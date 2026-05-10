import { ErrorHandler, Injector } from '@angular/core';
import { HttpErrorResponse, HttpHandler, HttpRequest } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { of, throwError } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';
import { TenantWorkspaceService } from '../services/tenant-workspace.service';

import { HttpErrorInterceptor } from './http-error.interceptor';

describe('HttpErrorInterceptor', () => {
  let interceptor: HttpErrorInterceptor;
  let errorHandler: jasmine.SpyObj<ErrorHandler>;

  beforeEach(() => {
    errorHandler = jasmine.createSpyObj<ErrorHandler>('ErrorHandler', ['handleError']);

    TestBed.configureTestingModule({
      providers: [
        { provide: ErrorHandler, useValue: errorHandler },
        {
          provide: Router,
          useValue: jasmine.createSpyObj<Router>('Router', ['navigate'], { url: '/transactions' })
        },
        {
          provide: AuthService,
          useValue: {
            isAuthenticated$: of(true)
          }
        },
        {
          provide: TenantWorkspaceService,
          useValue: {
            workspace$: of({
              authenticated: true,
              loading: false,
              platformScoped: false,
              currentTenant: {
                identifier: 'tenant-a',
                tenantId: 'tenant-a',
                tenantKey: 'tenant-a',
                label: 'Tenant A'
              },
              availableTenants: [],
              canSwitchTenant: false,
              status: 'ready',
              reason: null
            }),
            clearTenantSelection: jasmine.createSpy('clearTenantSelection'),
            isWorkspaceRequest: jasmine.createSpy('isWorkspaceRequest').and.returnValue(true)
          }
        }
      ]
    });

    interceptor = new HttpErrorInterceptor(
      TestBed.inject(Injector),
      TestBed.inject(Router),
      TestBed.inject(AuthService),
      TestBed.inject(TenantWorkspaceService)
    );
  });

  it('does not retry non-idempotent POST requests', () => {
    let subscriptionCount = 0;
    const request = new HttpRequest('POST', '/v1/admin/transactions/123/trigger-postback', null);
    const next: HttpHandler = {
      handle: () => throwError(() => {
        subscriptionCount += 1;
        return new HttpErrorResponse({ status: 500, statusText: 'Server Error' });
      })
    };

    interceptor.intercept(request, next).subscribe({
      error: () => undefined
    });

    expect(subscriptionCount).toBe(1);
  });

  it('retries GET requests once', () => {
    let subscriptionCount = 0;
    const request = new HttpRequest('GET', '/v1/admin/postbacks/stats');
    const next: HttpHandler = {
      handle: () => throwError(() => {
        subscriptionCount += 1;
        return new HttpErrorResponse({ status: 500, statusText: 'Server Error' });
      })
    };

    interceptor.intercept(request, next).subscribe({
      error: () => undefined
    });

    expect(subscriptionCount).toBe(2);
  });

  it('redirects workspace 403 responses to the denial page', () => {
    const router = TestBed.inject(Router) as jasmine.SpyObj<Router>;
    const request = new HttpRequest('GET', '/api/v1/admin/tenants/current');
    const next: HttpHandler = {
      handle: () => throwError(() => new HttpErrorResponse({ status: 403, statusText: 'Forbidden', url: request.url }))
    };

    interceptor.intercept(request, next).subscribe({
      error: () => undefined
    });

    expect(router.navigate).toHaveBeenCalledWith(['/403'], {
      queryParams: { reason: 'forbidden' }
    });
  });
});
