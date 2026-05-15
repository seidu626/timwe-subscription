import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { BehaviorSubject, of } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';
import { IconSetService } from '@coreui/icons-angular';

import { Page403Component } from './page403.component';
import { TenantWorkspaceService } from '../../../core/services/tenant-workspace.service';
import { iconSubset } from '../../../icons/icon-subset';

describe('Page403Component', () => {
  let component: Page403Component;
  let fixture: ComponentFixture<Page403Component>;
  let queryParamMap$: BehaviorSubject<ReturnType<typeof convertToParamMap>>;

  beforeEach(async () => {
    queryParamMap$ = new BehaviorSubject(convertToParamMap({}));

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
            queryParamMap: queryParamMap$.asObservable()
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
    let title = '';
    let description = '';
    const subscription = component.view$.subscribe((view) => {
      title = view.title;
      description = view.description;
    });

    queryParamMap$.next(convertToParamMap({ reason: 'forbidden' }));

    expect(title).toBe('Tenant workspace permission denied');
    expect(description).toContain('backend rejected access');
    subscription.unsubscribe();
  });

  it('does not describe backend tenant lookup failures as a missing account assignment', () => {
    let title = '';
    let description = '';
    const subscription = component.view$.subscribe((view) => {
      title = view.title;
      description = view.description;
    });

    queryParamMap$.next(convertToParamMap({ reason: 'tenant-not-found' }));

    expect(title).toBe('Tenant workspace not found');
    expect(description).toContain('backend could not resolve');
    subscription.unsubscribe();
  });
});
