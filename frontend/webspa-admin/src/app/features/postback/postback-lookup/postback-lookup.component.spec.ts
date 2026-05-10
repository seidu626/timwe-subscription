import { of } from 'rxjs';

import { PostbackLookupComponent } from './postback-lookup.component';

describe('PostbackLookupComponent', () => {
  function createComponent() {
    const postbackService = {
      bulkRequeueDlq: jasmine.createSpy().and.returnValue(of({ requeued: 1 })),
      retryPostback: jasmine.createSpy().and.returnValue(of({})),
      getStats: jasmine.createSpy().and.returnValue(of({})),
      getByStatus: jasmine.createSpy().and.returnValue(of({ postbacks: [] })),
      getPostbacksByTransactionId: jasmine.createSpy().and.returnValue(of({ postbacks: [], attempts: [] }))
    };

    const snackBar = {
      open: jasmine.createSpy()
    };

    const route = {
      queryParams: of({})
    };

    const component = new PostbackLookupComponent(postbackService as any, snackBar as any, route as any);

    return { component, postbackService, snackBar };
  }

  it('refreshes the active transaction search after bulk DLQ requeue', () => {
    const { component, postbackService } = createComponent();
    component.selectedStatus = 'DLQ';
    component.transactionId = 'b089f6a3-fc9d-4b0b-895a-355ea5192b66';
    component.statusDataSource.data = [{
      id: 'outbox-1',
      transaction_id: component.transactionId,
      event: 'conversion',
      provider: 'mobplus',
      url_template_rendered: 'https://example.com/postback',
      http_method: 'POST',
      headers: '{}',
      attempt_count: 2,
      max_attempts: 5,
      status: 'DLQ',
      created_at: '2026-03-23T00:00:00Z',
      updated_at: '2026-03-23T00:00:00Z'
    }];

    spyOn(window, 'confirm').and.returnValue(true);

    spyOn(component, 'loadStats');
    spyOn(component, 'loadStatusPostbacks');
    spyOn(component, 'searchPostbacks');

    component.bulkRequeueDlq();

    expect(postbackService.bulkRequeueDlq).toHaveBeenCalledWith(100, 0);
    expect(component.loadStats).toHaveBeenCalled();
    expect(component.loadStatusPostbacks).toHaveBeenCalled();
    expect(component.searchPostbacks).toHaveBeenCalled();
  });

  it('does not bulk requeue DLQ rows when the operator cancels confirmation', () => {
    const { component, postbackService } = createComponent();
    component.selectedStatus = 'DLQ';
    component.statusDataSource.data = [{
      id: 'outbox-2',
      transaction_id: '4eec612a-9839-4578-93ef-ae37508c77f5',
      event: 'conversion',
      provider: 'mobplus',
      url_template_rendered: 'https://example.com/postback',
      http_method: 'POST',
      headers: '{}',
      attempt_count: 1,
      max_attempts: 5,
      status: 'DLQ',
      created_at: '2026-03-23T00:00:00Z',
      updated_at: '2026-03-23T00:00:00Z'
    }];

    spyOn(window, 'confirm').and.returnValue(false);

    component.bulkRequeueDlq();

    expect(postbackService.bulkRequeueDlq).not.toHaveBeenCalled();
  });

  it('loads transaction-specific postbacks from a status row shortcut', () => {
    const { component } = createComponent();
    spyOn(component, 'searchPostbacks');

    component.inspectTransactionPostbacks('65a2d29b-0cfd-4e4d-a2da-bc0db697775b');

    expect(component.transactionId).toBe('65a2d29b-0cfd-4e4d-a2da-bc0db697775b');
    expect(component.searchPostbacks).toHaveBeenCalled();
  });
});
