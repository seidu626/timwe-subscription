import { Injectable } from '@angular/core';
import { HttpEvent, HttpHandler, HttpInterceptor, HttpRequest } from '@angular/common/http';
import { Observable } from 'rxjs';
import { filter, switchMap, take } from 'rxjs/operators';
import { TenantWorkspaceService, TenantWorkspaceState } from '../services/tenant-workspace.service';

@Injectable()
export class TenantWorkspaceInterceptor implements HttpInterceptor {
  constructor(private readonly tenantWorkspace: TenantWorkspaceService) {}

  intercept(request: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
    if (!this.tenantWorkspace.isWorkspaceRequest(request.url)) {
      return next.handle(request);
    }

    return this.tenantWorkspace.workspace$.pipe(
      filter((workspace) => !workspace.loading),
      take(1),
      switchMap((workspace) => next.handle(this.attachTenantContext(request, workspace)))
    );
  }

  private attachTenantContext(request: HttpRequest<any>, workspace: TenantWorkspaceState): HttpRequest<any> {
    const currentTenant = workspace.currentTenant;

    if (!currentTenant) {
      return request;
    }

    const headers: Record<string, string> = {
      'X-Tenant-Key': currentTenant.tenantKey
    };

    if (currentTenant.tenantId && currentTenant.tenantId !== currentTenant.tenantKey) {
      headers['X-Tenant-Id'] = currentTenant.tenantId;
    }

    return request.clone({
      setHeaders: headers
    });
  }
}
