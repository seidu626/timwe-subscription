import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { SMS_TEMPLATE_EVENT_TYPES } from '../models/sms-template.model';
import { SMSTemplateApiService } from './sms-template-api.service';

describe('SMSTemplateApiService', () => {
  let service: SMSTemplateApiService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [SMSTemplateApiService, provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(SMSTemplateApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  it('lists tenant SMS templates', () => {
    service.list().subscribe();
    const req = httpMock.expectOne((request) => request.method === 'GET' && request.url.endsWith('/api/v1/notification/sms-templates'));
    req.flush([]);
  });

  it('gets a product template with the event type query', () => {
    service.get(42, SMS_TEMPLATE_EVENT_TYPES.USER_OPTIN).subscribe();
    const req = httpMock.expectOne((request) => request.method === 'GET' && request.url.endsWith('/sms-templates/42') && request.params.get('event_type') === 'USER_OPTIN');
    req.flush({});
  });

  it('upserts a product template', () => {
    const payload = { eventType: SMS_TEMPLATE_EVENT_TYPES.USER_OPTIN, enabled: true, template: 'Welcome {{msisdn}}' };
    service.upsert(42, payload).subscribe();
    const req = httpMock.expectOne((request) => request.method === 'PUT' && request.url.endsWith('/sms-templates/42'));
    expect(req.request.body).toEqual(payload);
    req.flush({});
  });

  it('toggles a product template', () => {
    service.setEnabled(42, false).subscribe();
    const req = httpMock.expectOne((request) => request.method === 'PATCH' && request.url.endsWith('/sms-templates/42/enabled'));
    expect(req.request.body).toEqual({ enabled: false });
    req.flush({});
  });
});
