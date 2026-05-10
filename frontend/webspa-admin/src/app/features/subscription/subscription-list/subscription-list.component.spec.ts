import { Subject, throwError, of } from 'rxjs';

import { SubscriptionListComponent } from './subscription-list.component';

describe('SubscriptionListComponent', () => {
  function createComponent() {
    const subscriptionService = {
      executeAdminAction: jasmine.createSpy(),
      getSubscriptions: jasmine.createSpy().and.returnValue(of({ data: [], totalCount: 0, page: 1, pageSize: 10, totalPages: 0 })),
      getAdminActionHistory: jasmine.createSpy().and.returnValue(of({ data: [], totalCount: 0, page: 1, pageSize: 10, totalPages: 0 })),
      getAdminActionById: jasmine.createSpy()
    };

    const snackBar = {
      open: jasmine.createSpy()
    };

    const component = new SubscriptionListComponent(subscriptionService as any, snackBar as any);

    return { component, subscriptionService, snackBar };
  }

  it('starts with blank sendable admin fields', () => {
    const { component } = createComponent();

    expect(component.adminForm.msisdn).toBe('');
    expect(component.adminForm.productId).toBeNull();
    expect(component.adminForm.userIdentifierType).toBe('');
    expect(component.adminForm.mcc).toBe('');
    expect(component.adminForm.mnc).toBe('');
    expect(component.adminForm.entryChannel).toBe('');
    expect(component.adminForm.largeAccount).toBe('');
    expect(component.adminForm.subKeyword).toBe('');
    expect(component.adminForm.trackingId).toBe('');
    expect(component.adminForm.clientIp).toBe('');
    expect(component.adminForm.campaignUrl).toBe('');
    expect(component.adminForm.externalTxId).toBe('');
  });

  it('aborts submit when custom headers JSON is malformed', () => {
    const { component, subscriptionService, snackBar } = createComponent();
    component.adminForm.msisdn = '233572503330';
    component.adminForm.productId = 8509;
    component.adminHeadersText = '{not-json';
    const generateExternalTxId = spyOn<any>(component, 'generateExternalTxId').and.returnValue('external-123');

    component.submitAdminAction();

    expect(generateExternalTxId).not.toHaveBeenCalled();
    expect(subscriptionService.executeAdminAction).not.toHaveBeenCalled();
    expect(component.adminActionLoading).toBeFalse();
    expect(snackBar.open).toHaveBeenCalledWith('Custom headers JSON is invalid', 'Close', {
      duration: 5000
    });
  });

  it('preserves the request id on failure and clears stale result state before submitting', () => {
    const { component, subscriptionService, snackBar } = createComponent();
    component.adminForm.msisdn = '233572503330';
    component.adminForm.productId = 8509;
    component.lastActionResult = { operation: 'optin' } as any;
    component.selectedHistoryAction = { id: 'history-1' } as any;
    spyOn<any>(component, 'generateExternalTxId').and.returnValue('external-fail-1');
    subscriptionService.executeAdminAction.and.returnValue(throwError(() => ({
      error: { message: 'boom' }
    })));

    component.submitAdminAction();

    expect(component.lastActionResult).toBeNull();
    expect(component.selectedHistoryAction).toBeNull();
    expect(component.adminForm.externalTxId).toBe('external-fail-1');
    expect(subscriptionService.executeAdminAction).toHaveBeenCalledTimes(1);
    expect(subscriptionService.executeAdminAction).toHaveBeenCalledWith('optin', jasmine.objectContaining({
      externalTxId: 'external-fail-1',
      msisdn: '233572503330',
      productId: 8509
    }));
    expect(snackBar.open).toHaveBeenCalledWith('boom (externalTxId: external-fail-1)', 'Close', {
      duration: 6000,
      panelClass: ['error-snackbar']
    });
  });

  it('blocks duplicate admin action submissions while the first request is in flight', () => {
    const { component, subscriptionService } = createComponent();
    const pending = new Subject<unknown>();
    component.adminForm.msisdn = '233572503330';
    component.adminForm.productId = 8509;
    spyOn<any>(component, 'generateExternalTxId').and.returnValue('external-dup-1');
    subscriptionService.executeAdminAction.and.returnValue(pending.asObservable());

    component.submitAdminAction();
    component.submitAdminAction();

    expect(component.adminActionLoading).toBeTrue();
    expect(subscriptionService.executeAdminAction).toHaveBeenCalledTimes(1);
  });
});
