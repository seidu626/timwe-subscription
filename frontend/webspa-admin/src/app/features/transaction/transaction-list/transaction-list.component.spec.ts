import { fakeAsync, flushMicrotasks } from '@angular/core/testing';
import { Subject } from 'rxjs';

import { TransactionListComponent } from './transaction-list.component';

describe('TransactionListComponent', () => {
  function createComponent() {
    const transactionService = {
      getTransactions: jasmine.createSpy(),
      getTransactionStats: jasmine.createSpy(),
      triggerPostback: jasmine.createSpy(),
      getTransactionById: jasmine.createSpy()
    };

    const campaignService = {
      getCampaigns: jasmine.createSpy()
    };

    const router = {
      navigate: jasmine.createSpy()
    };

    const snackBar = {
      open: jasmine.createSpy()
    };

    const dialog = {
      open: jasmine.createSpy()
    };

    const component = new TransactionListComponent(
      transactionService as any,
      campaignService as any,
      router as any,
      snackBar as any,
      dialog as any
    );

    return { component, transactionService, snackBar };
  }

  it('initializes the list filters with the same default date window used by stats', () => {
    const { component } = createComponent();

    expect(component.filters.start_date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(component.filters.end_date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(component.filters.page).toBe(1);
    expect(component.filters.page_size).toBe(20);
  });

  it('shows an error snackbar when clipboard copy fails', fakeAsync(() => {
    const { component, snackBar } = createComponent();
    const writeText = jasmine.createSpy().and.returnValue(Promise.reject(new Error('denied')));

    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText }
    });

    component.copyToClipboard('233241234567');
    flushMicrotasks();

    expect(writeText).toHaveBeenCalledWith('233241234567');
    expect(snackBar.open).toHaveBeenCalledWith('Failed to copy to clipboard', 'Close', {
      duration: 3000,
      panelClass: ['error-snackbar']
    });
  }));

  it('ignores stale transaction detail responses when operators switch rows quickly', () => {
    const { component, transactionService } = createComponent();
    const firstResponse = new Subject<any>();
    const secondResponse = new Subject<any>();

    transactionService.getTransactionById.and.returnValues(firstResponse, secondResponse);

    component.toggleTransactionDetails({ id: 'tx-a' } as any);
    component.toggleTransactionDetails({ id: 'tx-b' } as any);

    secondResponse.next({ id: 'tx-b', correlation_id: 'corr-b' });
    secondResponse.complete();
    firstResponse.next({ id: 'tx-a', correlation_id: 'corr-a' });
    firstResponse.complete();

    expect(component.selectedTransactionId).toBe('tx-b');
    expect(component.selectedTransaction?.id).toBe('tx-b');
    expect(component.selectedTransaction?.correlation_id).toBe('corr-b');
  });
});
