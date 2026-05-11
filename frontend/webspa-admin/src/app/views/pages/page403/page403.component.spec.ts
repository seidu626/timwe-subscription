import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';
import { ActivatedRoute, Router } from '@angular/router';
import { of } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';
import { IconSetService } from '@coreui/icons-angular';

import { Page403Component } from './page403.component';
import { TenantWorkspaceService } from '../../../core/services/tenant-workspace.service';
import { iconSubset } from '../../../icons/icon-subset';

describe('Page403Component', () => {
  let component: Page403Component;
  let fixture: ComponentFixture<Page403Component>;
  let workspaceState: any;

  beforeEach(async () => {
    workspaceState = {
      authenticated: true,
      loading: false,
      platformScoped: true,
      currentTenant: null,
      availableTenants: [],
      canSwitchTenant: false,
      status: 'missing-tenant',
      reason: 'missing-tenant'
    };

    await TestBed.configureTestingModule({
      imports: [RouterTestingModule, Page403Component],
      providers: [
        IconSetService,
        {
          provide: AuthService,
          useValue: {
            isAuthenticated$: of(true),
            loginWithRedirect: jasmine.createSpy('loginWithRedirect'),
            logout: jasmine.createSpy('logout')
          }
        },
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              queryParams: {}
            }
          }
        },
        {
          provide: TenantWorkspaceService,
          useValue: {
            workspace$: of(workspaceState),
            selectTenant: jasmine.createSpy('selectTenant').and.returnValue(true)
          }
        }
      ]
    }).compileComponents();

    const iconSetService = TestBed.inject(IconSetService);
    iconSetService.icons = { ...iconSubset };

    fixture = TestBed.createComponent(Page403Component);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('creates the denial page', () => {
    expect(component).toBeTruthy();
  });

  it('labels workspace permission failures without implying a missing assignment', () => {
    const route = TestBed.inject(ActivatedRoute);
    route.snapshot.queryParams = { reason: 'forbidden' };

    expect(component.title).toBe('Tenant workspace denied');
    expect(component.description).toContain('additional permission');
  });

  it('redirects stale missing-tenant denials once the workspace resolves ready', () => {
    workspaceState.status = 'ready';
    workspaceState.reason = null;
    workspaceState.currentTenant = {
      identifier: 'nrg',
      tenantId: 'nrg',
      tenantKey: 'nrg',
      label: 'NRG'
    };
    const router = TestBed.inject(Router);
    spyOn(router, 'navigate').and.resolveTo(true);

    fixture = TestBed.createComponent(Page403Component);
    fixture.detectChanges();

    expect(router.navigate).toHaveBeenCalledWith(['/dashboard'], { replaceUrl: true });
  });
});
