// campaign.model.ts

export type FlowType = 'CLICK_TO_SMS' | 'OTP' | 'REDIRECT' | 'MIXED';

export interface CampaignLPCopyLocale {
  heroTitle: string;
  heDescription: string;
  heCta: string;
  heModalTitle: string;
  heModalConfirm: string;
  msisdnDescription: string;
  msisdnPlaceholder: string;
  msisdnCta: string;
  otpDescription: string;
  otpPlaceholder: string;
  otpCta: string;
  successTitle: string;
  successBody: string;
  consentPrefix: string;
  consentTerms: string;
  termsHeading: string;
  legal: string;
  phoneRequired: string;
  phoneInvalid: string;
  otpInvalid: string;
  consentRequired: string;
}

export interface CampaignLPCopy {
  en: CampaignLPCopyLocale;
  ar?: CampaignLPCopyLocale;
}

export interface CampaignVisualConfig {
  background_image_url?: string;
  theme_color?: string;
}

export interface CampaignTrackingConfig {
  pixels?: any;
  attribution?: any;
  custom_events?: Array<{ name: string; trigger: string }>;
  visual?: CampaignVisualConfig;
}

export interface Campaign {
  id: number;
  slug: string;
  language: string;
  country: string;
  operator?: string;
  offer_product_id: number;
  pricepoint_id?: number;
  partner_role_id?: number;
  flow_type: FlowType;
  short_code?: string;
  sms_keyword?: string;
  price?: number;
  billing_cycle?: string;
  trial_flags?: any;
  terms_url?: string;
  inline_terms_text?: string;
  consent_required: boolean;
  consent_version?: string;
  attribution_mapping?: any;
  postback_rules?: any;
  throttles?: any;
  allowed_referrers?: string[];
  allowed_sources?: string[];
  landing_page_urls?: string[];
  tracking_config?: CampaignTrackingConfig;
  lp_copy?: CampaignLPCopy;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  created_by?: string;
  updated_by?: string;
}

export interface CampaignListResponse {
  campaigns: Campaign[];
}

export interface CampaignCreateRequest {
  slug: string;
  language: string;
  country: string;
  operator?: string;
  offer_product_id: number;
  pricepoint_id?: number;
  partner_role_id?: number;
  flow_type: FlowType;
  short_code?: string;
  sms_keyword?: string;
  price?: number;
  billing_cycle?: string;
  trial_flags?: any;
  terms_url?: string;
  inline_terms_text?: string;
  consent_required: boolean;
  consent_version?: string;
  attribution_mapping?: any;
  postback_rules?: any;
  throttles?: any;
  allowed_referrers?: string[];
  allowed_sources?: string[];
  landing_page_urls?: string[];
  tracking_config?: CampaignTrackingConfig;
  lp_copy?: CampaignLPCopy;
  enabled: boolean;
  created_by?: string;
  updated_by?: string;
}

export interface CampaignUpdateRequest extends Omit<CampaignCreateRequest, 'slug'> {
  // slug is immutable, taken from URL path
}

export interface SetEnabledRequest {
  enabled: boolean;
  updated_by?: string;
}
