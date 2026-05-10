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
import { Observable } from 'rxjs';
import { TenantWorkspaceService, TenantWorkspaceOption } from '../../../core/services/tenant-workspace.service';

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

  readonly workspace$ = this.tenantWorkspace.workspace$;
  readonly isAuthenticated$: Observable<boolean> = this.auth.isAuthenticated$;

  get title(): string {
    const reason = this.route.snapshot.queryParams['reason'];

    switch (reason) {
      case 'selection-required':
        return 'Select a tenant workspace';
      case 'invalid-selection':
        return 'Tenant workspace denied';
      case 'missing-tenant':
      default:
        return 'Tenant workspace unavailable';
    }
  }

  get description(): string {
    const reason = this.route.snapshot.queryParams['reason'];

    switch (reason) {
      case 'selection-required':
        return 'Choose one of your permitted tenants before opening tenant-scoped admin views.';
      case 'invalid-selection':
        return 'The selected tenant is not available for this account or no longer matches the active assignment.';
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
}
