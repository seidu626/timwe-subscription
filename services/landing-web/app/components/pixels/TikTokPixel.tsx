'use client'

import Script from 'next/script'

interface TikTokPixelProps {
  pixelId: string
  enabled?: boolean
  autoPageView?: boolean
}

declare global {
  interface Window {
    ttq: {
      load: (pixelId: string) => void
      page: () => void
      track: (eventName: string, params?: Record<string, unknown>) => void
      identify: (params: Record<string, unknown>) => void
      instances: unknown[]
      _i: Record<string, unknown>
      _t: Record<string, number>
      _o: Record<string, unknown>
    }
    TiktokAnalyticsObject: string
  }
}

/**
 * TikTok Pixel component
 * @see https://ads.tiktok.com/marketing_api/docs?id=1739585700402178
 */
export function TikTokPixel({
  pixelId,
  enabled = true,
  autoPageView = true
}: TikTokPixelProps) {
  if (!enabled || !pixelId) return null

  return (
    <Script id="tiktok-pixel-init" strategy="afterInteractive">
      {`
        !function (w, d, t) {
          w.TiktokAnalyticsObject=t;var ttq=w[t]=w[t]||[];
          ttq.methods=["page","track","identify","instances","debug","on","off","once","ready","alias","group","enableCookie","disableCookie"];
          ttq.setAndDefer=function(t,e){t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}};
          for(var i=0;i<ttq.methods.length;i++)ttq.setAndDefer(ttq,ttq.methods[i]);
          ttq.instance=function(t){for(var e=ttq._i[t]||[],n=0;n<ttq.methods.length;n++)ttq.setAndDefer(e,ttq.methods[n]);return e};
          ttq.load=function(e,n){var i="https://analytics.tiktok.com/i18n/pixel/events.js";
          ttq._i=ttq._i||{};ttq._i[e]=[];ttq._i[e]._u=i;ttq._t=ttq._t||{};ttq._t[e]=+new Date;ttq._o=ttq._o||{};ttq._o[e]=n||{};
          var o=document.createElement("script");o.type="text/javascript";o.async=!0;o.src=i+"?sdkid="+e+"&lib="+t;
          var a=document.getElementsByTagName("script")[0];a.parentNode.insertBefore(o,a)};
          ttq.load('${pixelId}');
          ${autoPageView ? 'ttq.page();' : ''}
        }(window, document, 'ttq');
      `}
    </Script>
  )
}

/**
 * Track a TikTok event
 * @see https://ads.tiktok.com/marketing_api/docs?id=1739585696931842
 */
export function ttqTrackEvent(
  eventName: string,
  params?: Record<string, unknown>
): void {
  if (typeof window !== 'undefined' && window.ttq) {
    window.ttq.track(eventName, params)
  }
}

/**
 * Identify a user for TikTok Pixel
 */
export function ttqIdentify(params: {
  email?: string
  phone_number?: string
  external_id?: string
}): void {
  if (typeof window !== 'undefined' && window.ttq) {
    window.ttq.identify(params)
  }
}

/**
 * Standard TikTok Pixel Events
 */
export const TT_EVENTS = {
  // Page Events
  VIEW_CONTENT: 'ViewContent',
  CLICK_BUTTON: 'ClickButton',
  SEARCH: 'Search',

  // Form Events
  SUBMIT_FORM: 'SubmitForm',
  CONTACT: 'Contact',
  SUBSCRIBE: 'Subscribe',

  // Conversion Events
  COMPLETE_REGISTRATION: 'CompleteRegistration',
  COMPLETE_PAYMENT: 'CompletePayment',
  PLACE_AN_ORDER: 'PlaceAnOrder',

  // E-commerce
  ADD_TO_CART: 'AddToCart',
  ADD_TO_WISHLIST: 'AddToWishlist',
  INITIATE_CHECKOUT: 'InitiateCheckout',
  ADD_PAYMENT_INFO: 'AddPaymentInfo',

  // Download
  DOWNLOAD: 'Download',
} as const

/**
 * Hook for tracking TikTok events
 */
export function useTikTokPixel() {
  const track = (eventName: string, params?: Record<string, unknown>) => {
    ttqTrackEvent(eventName, params)
  }

  const identify = (params: {
    email?: string
    phone_number?: string
    external_id?: string
  }) => {
    ttqIdentify(params)
  }

  const trackSubmitForm = (params?: {
    content_type?: string
    content_id?: string
  }) => {
    ttqTrackEvent(TT_EVENTS.SUBMIT_FORM, params)
  }

  const trackSubscribe = (params?: {
    value?: number
    currency?: string
  }) => {
    ttqTrackEvent(TT_EVENTS.SUBSCRIBE, params)
  }

  const trackCompleteRegistration = (params?: {
    content_type?: string
    content_id?: string
  }) => {
    ttqTrackEvent(TT_EVENTS.COMPLETE_REGISTRATION, params)
  }

  const trackContact = (params?: Record<string, unknown>) => {
    ttqTrackEvent(TT_EVENTS.CONTACT, params)
  }

  return {
    track,
    identify,
    trackSubmitForm,
    trackSubscribe,
    trackCompleteRegistration,
    trackContact,
  }
}

export default TikTokPixel
