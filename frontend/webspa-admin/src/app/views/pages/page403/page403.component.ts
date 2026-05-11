import { CommonModule } from '@angular/common';
import { Component, OnInit, inject } from '@angular/core';
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
import { Observable } from 'rxjs';
import { filter, take } from 'rxjs/operators';
import { TenantWorkspaceService, TenantWorkspaceOption } from '../../../core/services/tenant-workspace.service';

type TenantDenialReason = 'selection-required' | 'invalid-selection' | 'missing-tenant' | 'forbidden' | 'tenant-not-found';

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
export class Page403Component implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly tenantWorkspace = inject(TenantWorkspaceService);

  readonly workspace$ = this.tenantWorkspace.workspace$;
  readonly isAuthenticated$: Observable<boolean> = this.auth.isAuthenticated$;

  ngOnInit(): void {
    if (this.reason === 'selection-required' || this.reason === 'invalid-selection' || this.reason === 'forbidden') {
      return;
    }

    this.workspace$.pipe(
      filter((workspace) => !workspace.loading && workspace.status === 'ready'),
      take(1)
    ).subscribe(() => {
      void this.router.navigate(['/dashboard'], { replaceUrl: true });
    });
  }

  get title(): string {
    switch (this.reason) {
      case 'selection-required':
        return 'Select a tenant workspace';
      case 'invalid-selection':
      case 'forbidden':
        return 'Tenant workspace denied';
      case 'tenant-not-found':
        return 'Tenant workspace not found';
      case 'missing-tenant':
      default:
        return 'Tenant workspace unavailable';
    }
  }

  get description(): string {
    switch (this.reason) {
      case 'selection-required':
        return 'Choose one of your permitted tenants before opening tenant-scoped admin views.';
      case 'invalid-selection':
        return 'The selected tenant is not available for this account or no longer matches the active assignment.';
      case 'forbidden':
        return 'Your tenant assignment is active, but this admin view requires additional permission for the selected workspace.';
      case 'tenant-not-found':
        return 'The selected tenant workspace could not be found. Choose another assigned tenant or return to the dashboard.';
      case 'missing-tenant':
      default:
        return 'This account does not currently have a tenant assignment, so the workspace cannot load protected data.';
    }
  }

  chooseTenant(tenant: TenantWorkspaceOption): void {
    if (!this.tenantWorkspace.selectTenant(tenant.identifier)) {
      return;
    }

    void this.router.navigate(['/dashboard'], { replaceUrl: true });
  }

  login(): void {
    this.auth.loginWithRedirect({
      appState: {
        target: this.router.url
      }
    });
  }

  logout(): void {
    this.auth.logout({
      logoutParams: {
        returnTo: window.location.origin
      }
    });
  }

  private get reason(): TenantDenialReason {
    const value = this.route.snapshot.queryParams['reason'];
    const normalized = typeof value === 'string' ? value : 'missing-tenant';

    switch (normalized) {
      case 'selection-required':
      case 'invalid-selection':
      case 'missing-tenant':
      case 'forbidden':
      case 'tenant-not-found':
        return normalized;
      default:
        return 'missing-tenant';
    }
  }
}
