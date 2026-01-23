'use client'

import { createContext, useContext, useCallback, useMemo } from 'react'
import { FacebookPixel, fbTrackEvent, FB_EVENTS } from './FacebookPixel'
import { GoogleTag, gtagEvent, gtagConversion, GA_EVENTS } from './GoogleTag'
import { TikTokPixel, ttqTrackEvent, TT_EVENTS } from './TikTokPixel'
import type { PixelConfiguration, ConversionEvent, AttributionData } from '@/app/types'

// ============================================
// Types
// ============================================

interface PixelManagerProps {
  config?: PixelConfiguration
  children: React.ReactNode
}

interface PixelContextValue {
  trackConversion: (event: ConversionEvent) => void
  trackPageView: (params?: Record<string, unknown>) => void
  trackEvent: (eventName: string, params?: Record<string, unknown>) => void
  setAttribution: (attribution: AttributionData) => void
  config: PixelConfiguration | undefined
}

// ============================================
// Event Mapping
// ============================================

/**
 * Maps internal events to platform-specific event names
 */
const EVENT_MAP: Record<string, { fb: string; ga: string; tt: string }> = {
  // Conversion Events
  subscription_success: {
    fb: FB_EVENTS.SUBSCRIBE,
    ga: GA_EVENTS.SIGN_UP,
    tt: TT_EVENTS.COMPLETE_REGISTRATION,
  },
  form_submit: {
    fb: FB_EVENTS.LEAD,
    ga: GA_EVENTS.GENERATE_LEAD,
    tt: TT_EVENTS.SUBMIT_FORM,
  },
  otp_requested: {
    fb: FB_EVENTS.COMPLETE_REGISTRATION,
    ga: GA_EVENTS.FORM_SUBMIT,
    tt: TT_EVENTS.SUBMIT_FORM,
  },
  otp_confirmed: {
    fb: FB_EVENTS.SUBSCRIBE,
    ga: GA_EVENTS.SIGN_UP,
    tt: TT_EVENTS.COMPLETE_REGISTRATION,
  },

  // Engagement Events
  landing_view: {
    fb: 'PageView',
    ga: GA_EVENTS.PAGE_VIEW,
    tt: 'Pageview',
  },
  scroll_25: {
    fb: 'ViewContent',
    ga: GA_EVENTS.SCROLL,
    tt: TT_EVENTS.VIEW_CONTENT,
  },
  scroll_50: {
    fb: 'ViewContent',
    ga: GA_EVENTS.SCROLL,
    tt: TT_EVENTS.VIEW_CONTENT,
  },
  scroll_75: {
    fb: 'ViewContent',
    ga: GA_EVENTS.SCROLL,
    tt: TT_EVENTS.VIEW_CONTENT,
  },
  scroll_100: {
    fb: 'ViewContent',
    ga: GA_EVENTS.SCROLL,
    tt: TT_EVENTS.VIEW_CONTENT,
  },

  // Form Events
  form_focus: {
    fb: 'Contact',
    ga: GA_EVENTS.FORM_START,
    tt: TT_EVENTS.CLICK_BUTTON,
  },
  phone_entered: {
    fb: 'Lead',
    ga: 'phone_entered',
    tt: TT_EVENTS.CONTACT,
  },
  terms_viewed: {
    fb: 'ViewContent',
    ga: 'terms_viewed',
    tt: TT_EVENTS.VIEW_CONTENT,
  },
  terms_accepted: {
    fb: 'Lead',
    ga: 'terms_accepted',
    tt: TT_EVENTS.CONTACT,
  },
}

// ============================================
// Context
// ============================================

const PixelContext = createContext<PixelContextValue | null>(null)

/**
 * Hook to access pixel tracking functions
 */
export function usePixels() {
  const context = useContext(PixelContext)
  if (!context) {
    // Return no-op functions if outside provider
    return {
      trackConversion: () => {},
      trackPageView: () => {},
      trackEvent: () => {},
      setAttribution: () => {},
      config: undefined,
    }
  }
  return context
}

// ============================================
// Attribution Storage
// ============================================

const ATTRIBUTION_KEY = 'lp_attribution'
const ATTRIBUTION_EXPIRY_DAYS = 7

/**
 * Store attribution data in cookie and sessionStorage
 */
function storeAttribution(attribution: AttributionData): void {
  if (typeof window === 'undefined') return

  // Store in sessionStorage for same-session access
  try {
    sessionStorage.setItem(ATTRIBUTION_KEY, JSON.stringify(attribution))
  } catch {}

  // Store in cookie for cross-session (7 days)
  try {
    const expires = new Date()
    expires.setDate(expires.getDate() + ATTRIBUTION_EXPIRY_DAYS)
    document.cookie = `${ATTRIBUTION_KEY}=${encodeURIComponent(
      JSON.stringify(attribution)
    )}; expires=${expires.toUTCString()}; path=/; SameSite=Lax`
  } catch {}
}

/**
 * Retrieve stored attribution data
 */
export function getStoredAttribution(): AttributionData | null {
  if (typeof window === 'undefined') return null

  // Try sessionStorage first
  try {
    const session = sessionStorage.getItem(ATTRIBUTION_KEY)
    if (session) return JSON.parse(session)
  } catch {}

  // Fallback to cookie
  try {
    const cookies = document.cookie.split(';')
    for (const cookie of cookies) {
      const [name, value] = cookie.trim().split('=')
      if (name === ATTRIBUTION_KEY && value) {
        return JSON.parse(decodeURIComponent(value))
      }
    }
  } catch {}

  return null
}

// ============================================
// PixelManager Component
// ============================================

/**
 * Unified Pixel Manager
 * Renders all configured pixels and provides unified tracking context
 */
export function PixelManager({ config, children }: PixelManagerProps) {
  /**
   * Track an event across all platforms
   */
  const trackEvent = useCallback(
    (eventName: string, params?: Record<string, unknown>) => {
      const mapped = EVENT_MAP[eventName] || {
        fb: eventName,
        ga: eventName,
        tt: eventName,
      }

      // Include stored attribution in params
      const attribution = getStoredAttribution()
      const enrichedParams = {
        ...params,
        ...(attribution && {
          click_id: attribution.click_id,
          utm_source: attribution.utm_source,
          utm_campaign: attribution.utm_campaign,
        }),
      }

      // Facebook
      if (config?.facebook?.enabled) {
        fbTrackEvent(mapped.fb, enrichedParams)
      }

      // Google
      if (config?.google?.enabled) {
        gtagEvent(mapped.ga, enrichedParams)
      }

      // TikTok
      if (config?.tiktok?.enabled) {
        ttqTrackEvent(mapped.tt, enrichedParams)
      }
    },
    [config]
  )

  /**
   * Track a conversion event with value
   */
  const trackConversion = useCallback(
    (event: ConversionEvent) => {
      const { event: eventName, value, currency, transaction_id, ...rest } = event
      const params: Record<string, unknown> = {
        ...rest,
        value,
        currency: currency || 'USD',
        transaction_id,
      }

      trackEvent(eventName, params)

      // Also send Google Ads conversion if configured
      if (config?.google?.enabled && config?.google?.ads_id && transaction_id) {
        gtagConversion(
          config.google.ads_id,
          'conversion', // You'd typically have a specific label
          { value, currency: currency || 'USD', transaction_id }
        )
      }
    },
    [config, trackEvent]
  )

  /**
   * Track page view across all platforms
   */
  const trackPageView = useCallback(
    (params?: Record<string, unknown>) => {
      trackEvent('landing_view', params)
    },
    [trackEvent]
  )

  /**
   * Set attribution data
   */
  const setAttribution = useCallback((attribution: AttributionData) => {
    storeAttribution(attribution)
  }, [])

  const contextValue = useMemo<PixelContextValue>(
    () => ({
      trackConversion,
      trackPageView,
      trackEvent,
      setAttribution,
      config,
    }),
    [trackConversion, trackPageView, trackEvent, setAttribution, config]
  )

  return (
    <PixelContext.Provider value={contextValue}>
      {/* Render pixel scripts */}
      {config?.facebook?.enabled && config.facebook.pixel_id && (
        <FacebookPixel
          pixelId={config.facebook.pixel_id}
          enabled={config.facebook.enabled}
          autoPageView={false} // We'll handle page views ourselves
        />
      )}

      {config?.google?.enabled && config.google.measurement_id && (
        <GoogleTag
          measurementId={config.google.measurement_id}
          adsId={config.google.ads_id}
          enabled={config.google.enabled}
          autoPageView={false}
        />
      )}

      {config?.tiktok?.enabled && config.tiktok.pixel_id && (
        <TikTokPixel
          pixelId={config.tiktok.pixel_id}
          enabled={config.tiktok.enabled}
          autoPageView={false}
        />
      )}

      {children}
    </PixelContext.Provider>
  )
}

// ============================================
// Exports
// ============================================

export { FacebookPixel, GoogleTag, TikTokPixel }
export { fbTrackEvent, gtagEvent, ttqTrackEvent }
export { FB_EVENTS, GA_EVENTS, TT_EVENTS }

export default PixelManager
