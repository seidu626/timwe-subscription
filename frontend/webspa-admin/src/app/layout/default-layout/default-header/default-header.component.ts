import { Component, computed, DestroyRef, inject, Input } from '@angular/core';
import {
  AvatarComponent,
  BadgeComponent,
  BreadcrumbRouterComponent,
  ColorModeService,
  ContainerComponent,
  DropdownComponent,
  DropdownDividerDirective,
  DropdownHeaderDirective,
  DropdownItemDirective,
  DropdownMenuDirective,
  DropdownToggleDirective,
  HeaderComponent,
  HeaderNavComponent,
  HeaderTogglerDirective,
  NavItemComponent,
  NavLinkDirective,
  ProgressBarDirective,
  ProgressComponent,
  SidebarToggleDirective,
  TextColorDirective,
  ThemeDirective
} from '@coreui/angular';
import { CommonModule, NgStyle, NgTemplateOutlet } from '@angular/common';
import { ActivatedRoute, Router, RouterLink, RouterLinkActive } from '@angular/router';
import { IconDirective } from '@coreui/icons-angular';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { delay, filter, map, tap } from 'rxjs/operators';
import { Observable } from 'rxjs';
import { AuthService, User } from '@auth0/auth0-angular';
import { TenantWorkspaceOption, TenantWorkspaceService } from '../../../core/services/tenant-workspace.service';


@Component({
  selector: 'app-default-header',
  templateUrl: './default-header.component.html',
  standalone: true,
  imports: [ContainerComponent, CommonModule,
    HeaderTogglerDirective, SidebarToggleDirective, IconDirective, HeaderNavComponent, NavItemComponent, NavLinkDirective, RouterLink, RouterLinkActive, NgTemplateOutlet, BreadcrumbRouterComponent, ThemeDirective, DropdownComponent, DropdownToggleDirective, TextColorDirective, AvatarComponent, DropdownMenuDirective, DropdownHeaderDirective, DropdownItemDirective, BadgeComponent, DropdownDividerDirective, ProgressBarDirective, ProgressComponent, NgStyle]
})
export class DefaultHeaderComponent extends HeaderComponent {

  readonly #activatedRoute: ActivatedRoute = inject(ActivatedRoute);
  readonly #colorModeService = inject(ColorModeService);
  readonly #router = inject(Router);
  readonly #authService = inject(AuthService);
  readonly #tenantWorkspace = inject(TenantWorkspaceService);
  readonly colorMode = this.#colorModeService.colorMode;
  readonly #destroyRef: DestroyRef = inject(DestroyRef);

  // Auth0 observables
  readonly isAuthenticated$: Observable<boolean> = this.#authService.isAuthenticated$;
  readonly user$: Observable<User | null | undefined> = this.#authService.user$;
  readonly workspace$ = this.#tenantWorkspace.workspace$;

  readonly colorModes = [
    { name: 'light', text: 'Light', icon: 'cilSun' },
    { name: 'dark', text: 'Dark', icon: 'cilMoon' },
    { name: 'auto', text: 'Auto', icon: 'cilContrast' }
  ];

  readonly icons = computed(() => {
    const currentMode = this.colorMode();
    return this.colorModes.find(mode=> mode.name === currentMode)?.icon ?? 'cilSun';
  });

  constructor() {
    super();
    this.#colorModeService.localStorageItemName.set('webspa-admin-theme-default');
    this.#colorModeService.eventName.set('ColorSchemeChange');

    this.#activatedRoute.queryParams
      .pipe(
        delay(1),
        map(params => <string>params['theme']?.match(/^[A-Za-z0-9\s]+/)?.[0]),
        filter(theme => ['dark', 'light', 'auto'].includes(theme)),
        tap(theme => {
          this.colorMode.set(theme);
        }),
        takeUntilDestroyed(this.#destroyRef)
      )
      .subscribe();
  }

  onLoginClick(): void {
    this.#authService.loginWithRedirect();
  }

  onLogoutClick(): void {
    this.#authService.logout({ 
      logoutParams: { 
        returnTo: window.location.origin 
      } 
    });
  }

  selectTenant(tenant: TenantWorkspaceOption): void {
    if (!this.#tenantWorkspace.selectTenant(tenant.identifier)) {
      return;
    }

    void this.#router.navigate(['/dashboard']);
  }

  tenantDisplayName(tenant: TenantWorkspaceOption): string {
    return tenant.label?.trim() || tenant.tenantKey || tenant.identifier;
  }

  tenantSecondaryText(tenant: TenantWorkspaceOption): string | null {
    const tenantKey = tenant.tenantKey?.trim();
    const displayName = this.tenantDisplayName(tenant).trim();

    if (!tenantKey || tenantKey.toLowerCase() === displayName.toLowerCase()) {
      return null;
    }

    return `Key: ${tenantKey}`;
  }

  @Input() sidebarId: string = 'sidebar1';

}
