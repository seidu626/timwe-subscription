import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  ChannelCredentialListResponse,
  ChannelCredentialPayload,
  ChannelCreatePayload,
  ChannelFilters,
  ChannelListResponse,
  ChannelUpdatePayload,
  CredentialSecretValue,
  OtpGatewaySecretValue,
  RevokeCredentialResponse,
  SmsGatewaySecretValue,
  TenantMemberListResponse,
  TenantMemberMutationResponse,
  TenantMemberPayload,
  TenantCreatePayload,
  TenantFilters,
  TenantListResponse,
  TenantMutationPayload,
  TenantMutationResponse
} from '../models/tenant.model';

export interface PresignBrandingUploadRequest {
  kind: 'logo' | 'banner';
  file_name: string;
  content_type: string;
  size_bytes: number;
}

export interface PresignBrandingUploadResponse {
  upload_url: string;
  asset_url: string;
  object_key: string;
  expires_in_seconds: number;
  max_size_bytes: number;
  allowed_content_types: string[];
}

@Injectable({
  providedIn: 'root'
})
export class TenantService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/tenants`;
  private channelsUrl = `${environment.acquisitionApiEndpoint}/v1/admin/channels`;
  private tenantAssetsUrl = `${environment.acquisitionApiEndpoint}/v1/admin/tenant-assets`;

  constructor(private http: HttpClient) {}

  list(filters?: TenantFilters): Observable<TenantListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.q) {
        params = params.set('q', filters.q);
      }
      if (filters.status) {
        params = params.set('status', filters.status);
      }
    }
    return this.http.get<TenantListResponse>(this.baseUrl, { params });
  }

  current(): Observable<TenantListResponse['tenants'][number]> {
    return this.http.get<TenantListResponse['tenants'][number]>(`${this.baseUrl}/current`);
  }

  create(payload: TenantCreatePayload): Observable<TenantMutationResponse> {
    return this.http.post<TenantMutationResponse>(this.baseUrl, payload);
  }

  update(id: string, payload: TenantMutationPayload): Observable<TenantMutationResponse> {
    return this.http.patch<TenantMutationResponse>(`${this.baseUrl}/${encodeURIComponent(id)}`, payload);
  }

  /** Workspace-scoped branding save; unlike update() it does not require platform scope. */
  updateCurrentTenantBranding(payload: { logo_url: string; banner_url: string; brand_color: string }): Observable<TenantMutationResponse> {
    return this.http.put<TenantMutationResponse>(`${this.baseUrl}/current/branding`, payload);
  }

  presignBrandingUpload(payload: PresignBrandingUploadRequest): Observable<PresignBrandingUploadResponse> {
    return this.http.post<PresignBrandingUploadResponse>(`${this.tenantAssetsUrl}/presign`, payload);
  }

  listMembers(tenantId: string, filters?: TenantFilters): Observable<TenantMemberListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.q) {
        params = params.set('q', filters.q);
      }
      if (filters.status) {
        params = params.set('status', filters.status);
      }
    }
    return this.http.get<TenantMemberListResponse>(`${this.baseUrl}/${encodeURIComponent(tenantId)}/members`, { params });
  }

  upsertMember(tenantId: string, payload: TenantMemberPayload): Observable<TenantMemberMutationResponse> {
    return this.http.post<TenantMemberMutationResponse>(`${this.baseUrl}/${encodeURIComponent(tenantId)}/members`, payload);
  }

  deactivateMember(tenantId: string, auth0Subject: string): Observable<{ audit_log_id: string }> {
    return this.http.delete<{ audit_log_id: string }>(
      `${this.baseUrl}/${encodeURIComponent(tenantId)}/members/${encodeURIComponent(auth0Subject)}`
    );
  }

  listChannels(filters?: ChannelFilters): Observable<ChannelListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.provider) {
        params = params.set('provider', filters.provider);
      }
      if (filters.country) {
        params = params.set('country', filters.country);
      }
      if (filters.enabled !== undefined) {
        params = params.set('enabled', String(filters.enabled));
      }
    }
    return this.http.get<ChannelListResponse>(this.channelsUrl, { params });
  }

  createChannel(payload: ChannelCreatePayload): Observable<ChannelListResponse['channels'][number]> {
    return this.http.post<ChannelListResponse['channels'][number]>(this.channelsUrl, payload);
  }

  updateChannel(channelId: string, payload: ChannelUpdatePayload): Observable<ChannelListResponse['channels'][number]> {
    return this.http.patch<ChannelListResponse['channels'][number]>(
      `${this.channelsUrl}/${encodeURIComponent(channelId)}`,
      payload
    );
  }

  setChannelEnabled(channelId: string, enabled: boolean): Observable<ChannelListResponse['channels'][number]> {
    return this.http.patch<ChannelListResponse['channels'][number]>(
      `${this.channelsUrl}/${encodeURIComponent(channelId)}/enabled`,
      { enabled }
    );
  }

  listChannelCredentials(channelId: string, purpose = 'provider_api'): Observable<ChannelCredentialListResponse> {
    const params = new HttpParams().set('purpose', purpose);
    return this.http.get<ChannelCredentialListResponse>(
      `${this.channelsUrl}/${encodeURIComponent(channelId)}/credentials`,
      { params }
    );
  }

  bindChannelCredential(channelId: string, payload: ChannelCredentialPayload): Observable<ChannelCredentialListResponse['credentials'][number]> {
    return this.http.post<ChannelCredentialListResponse['credentials'][number]>(
      `${this.channelsUrl}/${encodeURIComponent(channelId)}/credentials`,
      payload
    );
  }

  /** Serialize a per-field credential blob and POST it as secret_value. */
  bindChannelCredentialValue(
    channelId: string,
    purpose: string,
    value: CredentialSecretValue | SmsGatewaySecretValue | OtpGatewaySecretValue
  ): Observable<ChannelCredentialListResponse['credentials'][number]> {
    const payload: ChannelCredentialPayload = {
      purpose,
      secret_value: JSON.stringify(value)
    };
    return this.bindChannelCredential(channelId, payload);
  }

  /** Push one operator-triggered SMS through the tenant's bound sms_api gateway. */
  sendTestSMS(msisdn: string, message?: string): Observable<{ status: string; msisdn: string }> {
    const body: { msisdn: string; message?: string } = { msisdn };
    if (message?.trim()) {
      body.message = message.trim();
    }
    return this.http.post<{ status: string; msisdn: string }>(
      `${environment.acquisitionApiEndpoint}/v1/admin/sms/test`,
      body
    );
  }

  revokeChannelCredential(channelId: string, credentialId: string): Observable<RevokeCredentialResponse> {
    return this.http.delete<RevokeCredentialResponse>(
      `${this.channelsUrl}/${encodeURIComponent(channelId)}/credentials/${encodeURIComponent(credentialId)}`
    );
  }
}
