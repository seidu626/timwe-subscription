import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { MatSnackBar } from '@angular/material/snack-bar';
import { BehaviorSubject, of } from 'rxjs';

import { TenantListComponent } from './tenant-list.component';
import { TenantService } from '../../+state/services/tenant.service';
import { TenantWorkspaceService, TenantWorkspaceState } from '../../../core/services/tenant-workspace.service';
import { MaterialModule } from '../../../shared/material.module';
import { SharedModule } from '../../../shared/shared.module';

/**
 * Renders the credential form to confirm the purpose picker actually swaps the
 * fields an operator sees. The logic specs cover what is sent; this covers
 * whether the operator can reach it at all.
 */
describe('TenantListComponent credential form rendering', () => {
  let fixture: ComponentFixture<TenantListComponent>;
  let component: TenantListComponent;

  const workspace: TenantWorkspaceState = {
    authenticated: true,
    loading: false,
    platformScoped: true,
    currentTenant: { identifier: 'nrg', tenantId: 'tenant-1', tenantKey: 'nrg', label: 'NRG' },
    availableTenants: [{ identifier: 'nrg', tenantId: 'tenant-1', tenantKey: 'nrg', label: 'NRG' }],
    canSwitchTenant: false,
    status: 'ready',
    reason: null
  };

  beforeEach(async () => {
    const tenantService = {
      list: () => of({ tenants: [], total_count: 0, page: 1, page_size: 20 }),
      listChannels: () => of({ channels: [], total_count: 0, page: 1, page_size: 20 }),
      listTenantMembers: () => of({ members: [], total_count: 0, page: 1, page_size: 20 }),
      listChannelCredentials: () => of({ credentials: [], total_count: 0, page: 1, page_size: 20 })
    };

    await TestBed.configureTestingModule({
      declarations: [TenantListComponent],
      imports: [CommonModule, FormsModule, NoopAnimationsModule, MaterialModule, SharedModule],
      providers: [
        { provide: TenantService, useValue: tenantService },
        { provide: MatSnackBar, useValue: { open: () => undefined } },
        {
          provide: TenantWorkspaceService,
          useValue: { workspace$: new BehaviorSubject<TenantWorkspaceState>(workspace).asObservable() }
        }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(TenantListComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
    component.editingTenantId = 'tenant-1';
    component.credentialMode = 'value';
  });

  function renderedText(): string {
    fixture.detectChanges();
    return (fixture.nativeElement as HTMLElement).textContent ?? '';
  }

  it('shows the carrier fields for a provider API credential', () => {
    component.credentialValueForm.purpose = 'provider_api';
    const text = renderedText();
    expect(text).toContain('Partner Service ID');
    expect(text).not.toContain('Generate URL');
  });

  it('shows the gateway fields for an SMS credential', () => {
    component.credentialValueForm.purpose = 'sms_api';
    const text = renderedText();
    expect(text).toContain('Gateway URL');
    expect(text).toContain('Success Field');
    expect(text).not.toContain('Partner Service ID');
  });

  it('shows the provider fields and the mode-switch warning for a delegated OTP credential', () => {
    component.credentialValueForm.purpose = 'otp_api';
    const text = renderedText();
    expect(text).toContain('Generate URL');
    expect(text).toContain('Verify URL');
    expect(text).toContain('Delegated OTP replaces built-in code delivery');
    expect(text).not.toContain('Partner Service ID');
  });

  it('masks every secret field it renders', () => {
    for (const purpose of ['provider_api', 'sms_api', 'otp_api']) {
      component.credentialValueForm.purpose = purpose;
      fixture.detectChanges();
      const keyInputs = (fixture.nativeElement as HTMLElement).querySelectorAll('input[autocomplete="new-password"]');
      expect(keyInputs.length)
        .withContext(`${purpose} should render at least one secret field`)
        .toBeGreaterThan(0);
      keyInputs.forEach((input) => {
        expect((input as HTMLInputElement).type)
          .withContext(`${purpose} secret field must start masked`)
          .toBe('password');
      });
    }
  });
});
