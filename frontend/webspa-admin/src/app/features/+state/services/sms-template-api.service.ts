import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import { SMSTemplate, SMSTemplateEventType, SMSTemplateUpsert } from '../models/sms-template.model';

@Injectable({ providedIn: 'root' })
export class SMSTemplateApiService {
  private readonly baseUrl = `${environment.notificationApiEndpoint}/api/v1/notification/sms-templates`;

  constructor(private readonly http: HttpClient) {}

  list(): Observable<SMSTemplate[]> {
    return this.http.get<SMSTemplate[]>(this.baseUrl);
  }

  get(productId: number, eventType: SMSTemplateEventType): Observable<SMSTemplate> {
    const params = new HttpParams().set('event_type', eventType);
    return this.http.get<SMSTemplate>(`${this.baseUrl}/${productId}`, { params });
  }

  upsert(productId: number, payload: SMSTemplateUpsert): Observable<SMSTemplate> {
    return this.http.put<SMSTemplate>(`${this.baseUrl}/${productId}`, payload);
  }

  setEnabled(productId: number, enabled: boolean): Observable<SMSTemplate> {
    return this.http.patch<SMSTemplate>(`${this.baseUrl}/${productId}/enabled`, { enabled });
  }
}
