import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { of } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';
import { IconSetService } from '@coreui/icons-angular';

import { Page403Component } from './page403.component';
import { TenantWorkspaceService } from '../../../core/services/tenant-workspace.service';
import { iconSubset } from '../../../icons/icon-subset';

describe('Page403Component', () => {
  let component: Page403Component;
  let fixture: ComponentFixture<Page403Component>;
  let queryParams: Record<string, string>;

  beforeEach(async () => {
    queryParams = {};

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
          provide: TenantWorkspaceService,
          useValue: {
            workspace$: of({
              authenticated: true,
              loading: false,
              platformScoped: true,
              currentTenant: null,
              availableTenants: [],
              canSwitchTenant: false,
              status: 'missing-tenant',
              reason: 'missing-tenant'
            }),
            selectTenant: jasmine.createSpy('selectTenant').and.returnValue(true)
          }
        },
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              get queryParams() {
                return queryParams;
              }
            }
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

  it('does not describe backend forbidden responses as a missing tenant assignment', () => {
    queryParams = { reason: 'forbidden' };

    expect(component.title).toBe('Tenant workspace denied');
    expect(component.description).toContain('backend rejected access');
  });

  it('does not describe backend tenant lookup failures as a missing account assignment', () => {
    queryParams = { reason: 'tenant-not-found' };

    expect(component.title).toBe('Tenant workspace not found');
    expect(component.description).toContain('backend could not resolve');
  });
});
