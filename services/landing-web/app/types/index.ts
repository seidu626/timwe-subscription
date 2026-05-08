// Type definitions for the landing page application

// ============================================
// UTM & Attribution Parameters
// ============================================

export interface UTMParameters {
  utm_source?: string
  utm_medium?: string
  utm_campaign?: string
  utm_content?: string
  utm_term?: string
}

export interface AdPlatformClickIds {
  fbclid?: string    // Facebook
  gclid?: string     // Google Ads
  ttclid?: string    // TikTok
  msclkid?: string   // Microsoft/LinkedIn
  li_fat_id?: string // LinkedIn
  wbraid?: string    // Google Web-to-App
  gbraid?: string    // Google App-to-Web
}

// ============================================
// Pixel & Tracking Configuration
// ============================================

export interface FacebookPixelConfig {
  pixel_id: string
  enabled: boolean
}

export interface GoogleTagConfig {
  measurement_id: string  // GA4: G-XXXXXXX
  ads_id?: string         // Google Ads: AW-XXXXXXX
  enabled: boolean
}

export interface TikTokPixelConfig {
  pixel_id: string
  enabled: boolean
}

export interface PixelConfiguration {
  facebook?: FacebookPixelConfig
  google?: GoogleTagConfig
  tiktok?: TikTokPixelConfig
}

export interface AttributionConfig {
  model: 'first_touch' | 'last_touch' | 'linear'
  window_days: number
}

export interface TrackingVisualConfig {
  background_image_url?: string
  theme_color?: string
}

export interface TrackingConfiguration {
  pixels?: PixelConfiguration
  attribution?: AttributionConfig
  visual?: TrackingVisualConfig
  redirect_url?: string
  redirect?: {
    url: string
  }
  custom_events?: Array<{
    name: string
    trigger: string
  }>
}

export interface LandingCopyLocale {
  heroTitle: string
  heDescription: string
  heCta: string
  heModalTitle: string
  heModalConfirm: string
  msisdnDescription: string
  msisdnPlaceholder: string
  msisdnCta: string
  otpDescription: string
  otpPlaceholder: string
  otpCta: string
  successTitle: string
  successBody: string
  consentPrefix: string
  consentTerms: string
  termsHeading: string
  legal: string
  phoneRequired: string
  phoneInvalid: string
  otpInvalid: string
  consentRequired: string
}

export interface LandingCopyConfig {
  en?: LandingCopyLocale
  ar?: LandingCopyLocale
}

// ============================================
// Campaign Configuration
// ============================================

export interface Campaign {
  slug: string
  language: string
  country: string
  flow_type: string
  short_code?: string
  sms_keyword?: string
  price?: number
  currency?: string
  billing_cycle?: string
  terms_url?: string
  inline_terms_text?: string
  consent_required: boolean
  og_image?: string
  tracking_config?: TrackingConfiguration
  lp_copy?: LandingCopyConfig
}

export interface TransactionResponse {
  transaction_id: string
  correlation_id?: string
  status: 'PENDING' | 'ACTION_REQUIRED' | 'CONFIRM_REQUIRED' | 'SUBSCRIBED' | 'CHARGED' | 'FAILED' | 'CANCELLED'
  next_action?: 'OPEN_SMS' | 'OTP' | 'REDIRECT' | 'SHOW_INSTRUCTIONS' | 'SUBSCRIBED'
  payload?: {
    sms_link?: string
    short_code?: string
    keyword?: string
    fallback_steps?: string[]
    transaction_id?: string
    prompt?: string
    url?: string
    redirect_url?: string
    message?: string
  }
  error?: string
}

// LP flow/UI state
export type FlowStep = 'HE_PROMPT' | 'MSISDN_ENTRY' | 'OTP_ENTRY' | 'SUCCESS'

// Backend analytics event contract for /api/analytics/landing
export type AnalyticsEventType = 'landing_view' | 'landing_click' | 'form_submit'

export interface AttributionData extends UTMParameters, AdPlatformClickIds {
  // Mobplus/Affiliate params
  click_id?: string
  campaign_id?: string
  offer_id?: string
  adv_id?: string
  aff_id?: string
  sub1?: string
  sub2?: string
  sub3?: string
  sub4?: string
  sub5?: string
  source?: string
  creative?: string
  placement?: string
  // Session tracking
  session_id?: string
  referrer?: string
  landing_url?: string
  user_agent?: string
  // Timestamps
  first_touch_at?: string
  last_touch_at?: string
  [key: string]: string | undefined
}

export interface PhoneValidationResult {
  isValid: boolean
  error?: string
}

export interface AnalyticsEvent {
  event: string
  properties: Record<string, any>
  timestamp: string
  url: string
}

export interface FormState {
  msisdn: string
  consentChecked: boolean
  loading: boolean
  error: string | null
  phoneError: string | null
  formTouched: boolean
  otpCode: string
}

export type Provider = 'mobplus' | 'generic'

export type ClickIdParam = 'click_id' | 'txid' | 'clickid' | 'cid' | 'subid'

export type AdPlatformClickIdParam = 'fbclid' | 'gclid' | 'ttclid' | 'msclkid' | 'li_fat_id' | 'wbraid' | 'gbraid'

export type UTMParam = 'utm_source' | 'utm_medium' | 'utm_campaign' | 'utm_content' | 'utm_term'

export type PassthroughParam =
  | 'campaign_id'
  | 'offer_id'
  | 'adv_id'
  | 'aff_id'
  | 'pub_id'
  | 'sub1'
  | 'sub2'
  | 'sub3'
  | 'sub4'
  | 'sub5'
  | 'source'
  | 'creative'
  | 'placement'
  | UTMParam
  | AdPlatformClickIdParam

// ============================================
// Funnel & Analytics Events
// ============================================

export type FunnelEvent =
  // Awareness
  | 'landing_view'
  // Interest
  | 'scroll_25'
  | 'scroll_50'
  | 'scroll_75'
  | 'scroll_100'
  | 'time_30s'
  | 'time_60s'
  | 'time_120s'
  // Consideration
  | 'form_focus'
  | 'phone_entered'
  | 'terms_viewed'
  | 'terms_accepted'
  // Action
  | 'form_submit'
  | 'otp_requested'
  | 'otp_entered'
  // Conversion
  | 'subscription_success'
  | 'subscription_error'

export interface ConversionEvent {
  event: FunnelEvent | string
  value?: number
  currency?: string
  transaction_id?: string
  content_name?: string
  content_category?: string
  attribution?: AttributionData
}
