import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

import { PostbackService } from './postback.service';

describe('PostbackService', () => {
  let service: PostbackService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        PostbackService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });

    service = TestBed.inject(PostbackService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('maps status-list responses that return items into postbacks with paging metadata', () => {
    const received: any[] = [];

    service.getByStatus('DLQ', 50, 25).subscribe((response) => {
      received.push(response);
    });

    const req = httpMock.expectOne((request) =>
      request.method === 'GET' &&
      request.url.endsWith('/v1/admin/postbacks/status/DLQ') &&
      request.params.get('limit') === '50' &&
      request.params.get('offset') === '25'
    );

    req.flush({
      status: 'DLQ',
      count: 12,
      limit: 50,
      offset: 25,
      items: [
        {
          id: '6e4ed6fd-76ad-42cd-9487-bb2455371dcb',
          transaction_id: '54f45f42-e60d-4dca-adce-a55259737710',
          event: 'conversion',
          provider: 'mobplus',
          url_template_rendered: 'https://example.test/postback',
          http_method: 'POST',
          headers: '{}',
          attempt_count: 3,
          max_attempts: 5,
          status: 'DLQ',
          created_at: '2026-03-23T10:00:00Z',
          updated_at: '2026-03-23T10:05:00Z'
        }
      ]
    });

    expect(received[0]?.postbacks?.length).toBe(1);
    expect(received[0]?.postbacks?.[0]?.status).toBe('DLQ');
    expect(received[0]?.count).toBe(12);
    expect(received[0]?.offset).toBe(25);
  });
});
