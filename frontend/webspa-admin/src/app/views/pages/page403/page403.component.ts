import { CommonModule } from '@angular/common';
import { Component, inject } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { AuthService } from '@auth0/auth0-angular';
import { IconDirective } from '@coreui/icons-angular';
import {
  ButtonDirective,
  CardBodyComponent,
  CardComponent,
  CardGroupComponent,
  ColComponent,
  ContainerComponent,
  RowComponent
} from '@coreui/angular';
import { combineLatest, Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import {
  TenantWorkspaceOption,
  TenantWorkspaceService,
  TenantWorkspaceState
} from '../../../core/services/tenant-workspace.service';

type DenialReason =
  | 'missing-tenant'
  | 'selection-required'
  | 'invalid-selection'
  | 'forbidden'
  | 'tenant-not-found'
  | 'platform-required'
  | 'permission-error';

export interface Page403View {
  reason: DenialReason;
  title: string;
  description: string;
  hint: string | null;
  showTenantPanel: boolean;
  canRetry: boolean;
  workspace: TenantWorkspaceState;
}

@Component({
  selector: 'app-page403',
  templateUrl: './page403.component.html',
  styleUrls: ['./page403.component.scss'],
  standalone: true,
  imports: [
    CommonModule,
    ContainerComponent,
    RowComponent,
    ColComponent,
    CardGroupComponent,
    CardComponent,
    CardBodyComponent,
    ButtonDirective,
    IconDirective,
    RouterLink
  ]
})
export class Page403Component {
  private readonly route = inject(ActivatedRoute);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly tenantWorkspace = inject(TenantWorkspaceService);

  readonly workspace$: Observable<TenantWorkspaceState> = this.tenantWorkspace.workspace$;
  readonly isAuthenticated$: Observable<boolean> = this.auth.isAuthenticated$;

  readonly view$: Observable<Page403View> = combineLatest([
    this.route.queryParamMap,
    this.workspace$
  ]).pipe(
    map(([params, workspace]) => this.deriveView(params.get('reason'), workspace))
  );

  monogramFor(tenant: TenantWorkspaceOption): string {
    const seed = (tenant.label || tenant.tenantKey || tenant.identifier || '?').trim();
    const parts = seed.split(/[\s\-_]+/).filter(Boolean);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase();
    }
    return seed.slice(0, 2).toUpperCase();
  }

  chooseTenant(tenant: TenantWorkspaceOption): void {
    if (!this.tenantWorkspace.selectTenant(tenant.identifier)) {
      return;
    }
    void this.router.navigate(['/dashboard'], { replaceUrl: true });
  }

  retry(): void {
    void this.router.navigate(['/dashboard'], { replaceUrl: true });
  }

  login(): void {
    this.auth.loginWithRedirect({
      appState: { target: this.router.url }
    });
  }

  logout(): void {
    this.auth.logout({
      logoutParams: { returnTo: window.location.origin }
    });
  }

  // Reconcile the URL ?reason hint with the actual workspace state so the
  // page can't say "no tenant assignment" while a tenant is clearly assigned.
  // The URL hint is preferred when explicit; otherwise reason is inferred from
  // the live workspace state.
  private deriveView(rawReason: string | null, workspace: TenantWorkspaceState): Page403View {
    const reason = this.reconcileReason(rawReason, workspace);
    const hasTenants = workspace.availableTenants.length > 0;

    switch (reason) {
      case 'selection-required':
        return {
          reason,
          title: 'Select a tenant workspace',
          description: 'Choose one of your permitted tenants before opening tenant-scoped admin views.',
          hint: 'Your selection is remembered for this browser session.',
          showTenantPanel: hasTenants,
          canRetry: false,
          workspace
        };
      case 'invalid-selection':
        return {
          reason,
          title: 'Tenant workspace denied',
          description: 'The selected tenant is not available for this account or no longer matches the active assignment.',
          hint: 'Pick a tenant below to continue.',
          showTenantPanel: hasTenants,
          canRetry: false,
          workspace
        };
      case 'forbidden':
        return {
          reason,
          title: 'Tenant workspace permission denied',
          description: hasTenants
            ? 'Your account is assigned to this workspace, but the backend rejected access to this view. The view may require additional permissions.'
            : 'The active tenant is assigned, but the backend rejected access to this workspace view.',
          hint: 'If this is unexpected, contact your workspace administrator.',
          showTenantPanel: hasTenants,
          canRetry: hasTenants,
          workspace
        };
      case 'tenant-not-found':
        return {
          reason,
          title: 'Tenant workspace not found',
          description: 'The active tenant is assigned, but the backend could not resolve the tenant for this workspace view.',
          hint: 'Try again in a moment, or pick another tenant if you have access.',
          showTenantPanel: hasTenants,
          canRetry: hasTenants,
          workspace
        };
      case 'permission-error':
        return {
          reason,
          title: 'Workspace authorization error',
          description: 'Your account has tenant access, but this view could not load. The authorization layer may be temporarily unavailable.',
          hint: 'Retry in a moment, or sign out and back in to refresh your session.',
          showTenantPanel: hasTenants,
          canRetry: true,
          workspace
        };
      case 'platform-required':
        return {
          reason,
          title: 'Platform admin access required',
          description: 'This page manages the platform tenant catalog and is only available to platform-scoped administrators.',
          hint: 'Use the tenant workspace pages for tenant-scoped subscriptions, products, notifications, campaigns, and reports.',
          showTenantPanel: hasTenants,
          canRetry: false,
          workspace
        };
      case 'missing-tenant':
      default:
        return {
          reason: 'missing-tenant',
          title: 'Tenant workspace unavailable',
          description: 'This account does not currently have a tenant assignment, so the workspace cannot load protected data.',
          hint: 'Ask a workspace administrator to invite you, or sign in with a different account.',
          showTenantPanel: false,
          canRetry: false,
          workspace
        };
    }
  }

  private reconcileReason(rawReason: string | null, workspace: TenantWorkspaceState): DenialReason {
    const explicit = this.normalizeReason(rawReason);
    const hasTenants = workspace.availableTenants.length > 0;

    if (explicit) {
      // Explicit hint stands unless it directly contradicts the workspace —
      // claiming "missing-tenant" while the user clearly has memberships
      // is the contradiction that prompted this fix.
      if (explicit === 'missing-tenant' && hasTenants) {
        return workspace.canSwitchTenant ? 'selection-required' : 'permission-error';
      }
      return explicit;
    }

    // No explicit hint — infer from workspace state.
    if (workspace.status === 'selection-required') {
      return 'selection-required';
    }
    if (workspace.status === 'invalid-selection') {
      return 'invalid-selection';
    }
    if (hasTenants) {
      return 'permission-error';
    }
    return 'missing-tenant';
  }

  private normalizeReason(value: string | null): DenialReason | null {
    switch (value) {
      case 'missing-tenant':
      case 'selection-required':
      case 'invalid-selection':
      case 'forbidden':
      case 'tenant-not-found':
      case 'platform-required':
      case 'permission-error':
        return value;
      default:
        return null;
    }
  }
}
