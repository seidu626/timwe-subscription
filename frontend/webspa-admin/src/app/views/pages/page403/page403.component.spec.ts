import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { BehaviorSubject, of } from 'rxjs';
import { AuthService } from '@auth0/auth0-angular';
import { IconSetService } from '@coreui/icons-angular';

import { Page403Component } from './page403.component';
import {
  TenantWorkspaceService,
  TenantWorkspaceState
} from '../../../core/services/tenant-workspace.service';
import { iconSubset } from '../../../icons/icon-subset';

const emptyWorkspace: TenantWorkspaceState = {
  authenticated: true,
  loading: false,
  platformScoped: true,
  currentTenant: null,
  availableTenants: [],
  canSwitchTenant: false,
  status: 'missing-tenant',
  reason: 'missing-tenant'
};

describe('Page403Component', () => {
  let component: Page403Component;
  let fixture: ComponentFixture<Page403Component>;
  let queryParamMap$: BehaviorSubject<ReturnType<typeof convertToParamMap>>;
  let workspace$: BehaviorSubject<TenantWorkspaceState>;

  beforeEach(async () => {
    queryParamMap$ = new BehaviorSubject(convertToParamMap({}));
    workspace$ = new BehaviorSubject<TenantWorkspaceState>(emptyWorkspace);

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
            workspace$: workspace$.asObservable(),
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

  // Regression: the URL hint claimed missing-tenant while the workspace clearly
  // had an assigned tenant — the page used to show "no tenant assignment" and
  // the assigned tenant card at the same time.
  it('reconciles a missing-tenant hint to selection-required when multiple tenants are available', () => {
    workspace$.next({
      ...emptyWorkspace,
      platformScoped: false,
      status: 'selection-required',
      canSwitchTenant: true,
      availableTenants: [
        { identifier: 'nrg', label: 'NRG', tenantKey: 'nrg', tenantId: 'id-1' },
        { identifier: 'nrg2', label: 'NRG2', tenantKey: 'nrg2', tenantId: 'id-2' }
      ]
    });
    queryParamMap$.next(convertToParamMap({ reason: 'missing-tenant' }));

    let snapshot: { reason: string; title: string; showTenantPanel: boolean } | null = null;
    const subscription = component.view$.subscribe((view) => {
      snapshot = { reason: view.reason, title: view.title, showTenantPanel: view.showTenantPanel };
    });

    expect(snapshot!.reason).toBe('selection-required');
    expect(snapshot!.title).toBe('Select a tenant workspace');
    expect(snapshot!.showTenantPanel).toBeTrue();
    subscription.unsubscribe();
  });

  it('reconciles a missing-tenant hint to permission-error when one tenant is already assigned', () => {
    workspace$.next({
      ...emptyWorkspace,
      platformScoped: false,
      status: 'ready',
      canSwitchTenant: false,
      currentTenant: { identifier: 'nrg', label: 'NRG', tenantKey: 'nrg', tenantId: 'id-1' },
      availableTenants: [
        { identifier: 'nrg', label: 'NRG', tenantKey: 'nrg', tenantId: 'id-1' }
      ]
    });
    queryParamMap$.next(convertToParamMap({ reason: 'missing-tenant' }));

    let snapshot: { reason: string; description: string; canRetry: boolean } | null = null;
    const subscription = component.view$.subscribe((view) => {
      snapshot = { reason: view.reason, description: view.description, canRetry: view.canRetry };
    });

    expect(snapshot!.reason).toBe('permission-error');
    expect(snapshot!.description).not.toContain('does not currently have a tenant assignment');
    expect(snapshot!.canRetry).toBeTrue();
    subscription.unsubscribe();
  });
});
