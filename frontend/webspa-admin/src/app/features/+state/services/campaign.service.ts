// campaign.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable, map } from 'rxjs';
import { environment } from 'src/environments/environment';
import { 
  Campaign, 
  CampaignListResponse, 
  CampaignCreateRequest, 
  CampaignUpdateRequest,
  SetEnabledRequest 
} from '../models/campaign.model';

export interface PresignBackgroundUploadRequest {
  campaign_slug: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
}

export interface PresignBackgroundUploadResponse {
  upload_url: string;
  asset_url: string;
  object_key: string;
  expires_in_seconds: number;
  max_size_bytes: number;
  allowed_content_types: string[];
}

export interface CloneCampaignRequest {
  new_slug: string;
  created_by?: string;
}

@Injectable({
  providedIn: 'root'
})
export class CampaignService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/campaigns`;
  private campaignAssetBaseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/campaign-assets`;

  constructor(private http: HttpClient) {}

  /**
   * List all campaigns (admin view - includes enabled + disabled)
   */
  getCampaigns(filters?: { enabled?: boolean; country?: string }): Observable<Campaign[]> {
    let params = new HttpParams();
    if (filters?.enabled !== undefined) {
      params = params.set('enabled', filters.enabled.toString());
    }
    if (filters?.country) {
      params = params.set('country', filters.country);
    }

    return this.http.get<CampaignListResponse>(this.baseUrl, { params }).pipe(
      map(response => response.campaigns || [])
    );
  }

  /**
   * Get a campaign by slug (admin view - full details)
   */
  getCampaignBySlug(slug: string): Observable<Campaign> {
    return this.http.get<Campaign>(`${this.baseUrl}/${slug}`);
  }

  /**
   * Create a new campaign
   */
  createCampaign(campaign: CampaignCreateRequest): Observable<Campaign> {
    return this.http.post<Campaign>(this.baseUrl, campaign);
  }

  /**
   * Update an existing campaign by slug
   */
  updateCampaign(slug: string, campaign: CampaignUpdateRequest): Observable<Campaign> {
    return this.http.put<Campaign>(`${this.baseUrl}/${slug}`, campaign);
  }

  /**
   * Clone an existing campaign into a new slug.
   */
  cloneCampaign(sourceSlug: string, payload: CloneCampaignRequest): Observable<Campaign> {
    return this.http.post<Campaign>(`${this.baseUrl}/${sourceSlug}/clone`, payload);
  }

  /**
   * Enable or disable a campaign
   */
  setEnabled(slug: string, enabled: boolean, updatedBy?: string): Observable<Campaign> {
    const body: SetEnabledRequest = { enabled };
    if (updatedBy) {
      body.updated_by = updatedBy;
    }
    return this.http.patch<Campaign>(`${this.baseUrl}/${slug}/enabled`, body);
  }

  presignBackgroundUpload(payload: PresignBackgroundUploadRequest): Observable<PresignBackgroundUploadResponse> {
    return this.http.post<PresignBackgroundUploadResponse>(
      `${this.campaignAssetBaseUrl}/background/presign`,
      payload
    );
  }

  /**
   * Get the landing page preview URL for a campaign
   */
  getLandingPageUrl(slug: string): string {
    return `${environment.landingWebBaseUrl}/lp/${slug}`;
  }
}
