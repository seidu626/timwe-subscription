'use client'

import Script from 'next/script'

interface GoogleTagProps {
  measurementId: string      // GA4: G-XXXXXXX
  adsId?: string             // Google Ads: AW-XXXXXXX
  enabled?: boolean
  autoPageView?: boolean
}

declare global {
  interface Window {
    dataLayer: unknown[]
    gtag: (...args: unknown[]) => void
  }
}

/**
 * Google Tag (gtag.js) component
 * Supports both Google Analytics 4 and Google Ads
 */
export function GoogleTag({
  measurementId,
  adsId,
  enabled = true,
  autoPageView = true
}: GoogleTagProps) {
  if (!enabled || !measurementId) return null

  const tagIds = [measurementId, adsId].filter(Boolean)
  const configStatements = tagIds
    .map(id => `gtag('config', '${id}'${autoPageView ? '' : ", { 'send_page_view': false }"});`)
    .join('\n          ')

  return (
    <>
      <Script
        src={`https://www.googletagmanager.com/gtag/js?id=${measurementId}`}
        strategy="afterInteractive"
      />
      <Script id="gtag-init" strategy="afterInteractive">
        {`
          window.dataLayer = window.dataLayer || [];
          function gtag(){dataLayer.push(arguments);}
          gtag('js', new Date());
          ${configStatements}
        `}
      </Script>
    </>
  )
}

/**
 * Send an event to Google Analytics/Ads
 * @see https://developers.google.com/analytics/devguides/collection/ga4/reference/events
 */
export function gtagEvent(
  eventName: string,
  params?: Record<string, unknown>
): void {
  if (typeof window !== 'undefined' && window.gtag) {
    window.gtag('event', eventName, params)
  }
}

/**
 * Send a conversion to Google Ads
 */
export function gtagConversion(
  conversionId: string,
  conversionLabel: string,
  params?: {
    value?: number
    currency?: string
    transaction_id?: string
  }
): void {
  if (typeof window !== 'undefined' && window.gtag) {
    window.gtag('event', 'conversion', {
      send_to: `${conversionId}/${conversionLabel}`,
      ...params,
    })
  }
}

/**
 * Set user properties
 */
export function gtagSetUserProperties(properties: Record<string, unknown>): void {
  if (typeof window !== 'undefined' && window.gtag) {
    window.gtag('set', 'user_properties', properties)
  }
}

/**
 * Standard GA4 Events
 */
export const GA_EVENTS = {
  // Engagement
  PAGE_VIEW: 'page_view',
  SCROLL: 'scroll',
  CLICK: 'click',
  VIEW_SEARCH_RESULTS: 'view_search_results',
  FILE_DOWNLOAD: 'file_download',

  // Conversions
  SIGN_UP: 'sign_up',
  LOGIN: 'login',
  GENERATE_LEAD: 'generate_lead',
  PURCHASE: 'purchase',

  // E-commerce
  VIEW_ITEM: 'view_item',
  ADD_TO_CART: 'add_to_cart',
  BEGIN_CHECKOUT: 'begin_checkout',
  ADD_PAYMENT_INFO: 'add_payment_info',
  ADD_SHIPPING_INFO: 'add_shipping_info',

  // Form Events
  FORM_START: 'form_start',
  FORM_SUBMIT: 'form_submit',
} as const

/**
 * Hook for tracking Google Analytics events
 */
export function useGoogleTag() {
  const track = (eventName: string, params?: Record<string, unknown>) => {
    gtagEvent(eventName, params)
  }

  const trackConversion = (
    conversionId: string,
    conversionLabel: string,
    params?: { value?: number; currency?: string; transaction_id?: string }
  ) => {
    gtagConversion(conversionId, conversionLabel, params)
  }

  const trackLead = (params?: {
    value?: number
    currency?: string
  }) => {
    gtagEvent(GA_EVENTS.GENERATE_LEAD, params)
  }

  const trackSignUp = (params?: {
    method?: string
    value?: number
  }) => {
    gtagEvent(GA_EVENTS.SIGN_UP, params)
  }

  const trackFormSubmit = (params?: {
    form_id?: string
    form_name?: string
    form_destination?: string
  }) => {
    gtagEvent(GA_EVENTS.FORM_SUBMIT, params)
  }

  const trackScroll = (percent_scrolled: number) => {
    gtagEvent(GA_EVENTS.SCROLL, { percent_scrolled })
  }

  const setUserProperties = (properties: Record<string, unknown>) => {
    gtagSetUserProperties(properties)
  }

  return {
    track,
    trackConversion,
    trackLead,
    trackSignUp,
    trackFormSubmit,
    trackScroll,
    setUserProperties,
  }
}

export default GoogleTag
