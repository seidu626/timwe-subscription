'use client'

import Script from 'next/script'
import { useEffect } from 'react'

interface FacebookPixelProps {
  pixelId: string
  enabled?: boolean
  autoPageView?: boolean
}

declare global {
  interface Window {
    fbq: (...args: unknown[]) => void
    _fbq: unknown
  }
}

/**
 * Facebook/Meta Pixel component
 * Initializes the FB pixel and tracks PageView on mount
 */
export function FacebookPixel({
  pixelId,
  enabled = true,
  autoPageView = true
}: FacebookPixelProps) {
  if (!enabled || !pixelId) return null

  return (
    <>
      <Script id="fb-pixel-init" strategy="afterInteractive">
        {`
          !function(f,b,e,v,n,t,s)
          {if(f.fbq)return;n=f.fbq=function(){n.callMethod?
          n.callMethod.apply(n,arguments):n.queue.push(arguments)};
          if(!f._fbq)f._fbq=n;n.push=n;n.loaded=!0;n.version='2.0';
          n.queue=[];t=b.createElement(e);t.async=!0;
          t.src=v;s=b.getElementsByTagName(e)[0];
          s.parentNode.insertBefore(t,s)}(window, document,'script',
          'https://connect.facebook.net/en_US/fbevents.js');
          fbq('init', '${pixelId}');
          ${autoPageView ? "fbq('track', 'PageView');" : ''}
        `}
      </Script>
      <noscript>
        <img
          height="1"
          width="1"
          style={{ display: 'none' }}
          src={`https://www.facebook.com/tr?id=${pixelId}&ev=PageView&noscript=1`}
          alt=""
        />
      </noscript>
    </>
  )
}

/**
 * Track a standard Facebook event
 * @see https://developers.facebook.com/docs/meta-pixel/reference
 */
export function fbTrackEvent(
  eventName: string,
  params?: Record<string, unknown>
): void {
  if (typeof window !== 'undefined' && window.fbq) {
    window.fbq('track', eventName, params)
  }
}

/**
 * Track a custom Facebook event
 */
export function fbTrackCustomEvent(
  eventName: string,
  params?: Record<string, unknown>
): void {
  if (typeof window !== 'undefined' && window.fbq) {
    window.fbq('trackCustom', eventName, params)
  }
}

/**
 * Standard Facebook Pixel Events
 */
export const FB_EVENTS = {
  // Conversion Events
  LEAD: 'Lead',
  COMPLETE_REGISTRATION: 'CompleteRegistration',
  SUBSCRIBE: 'Subscribe',
  START_TRIAL: 'StartTrial',
  PURCHASE: 'Purchase',

  // Engagement Events
  VIEW_CONTENT: 'ViewContent',
  SEARCH: 'Search',
  ADD_TO_CART: 'AddToCart',
  ADD_TO_WISHLIST: 'AddToWishlist',
  INITIATE_CHECKOUT: 'InitiateCheckout',
  ADD_PAYMENT_INFO: 'AddPaymentInfo',

  // Custom
  CONTACT: 'Contact',
  CUSTOMIZE_PRODUCT: 'CustomizeProduct',
  DONATE: 'Donate',
  FIND_LOCATION: 'FindLocation',
  SCHEDULE: 'Schedule',
  SUBMIT_APPLICATION: 'SubmitApplication',
} as const

/**
 * Hook for tracking Facebook events with attribution
 */
export function useFacebookPixel() {
  const track = (eventName: string, params?: Record<string, unknown>) => {
    fbTrackEvent(eventName, params)
  }

  const trackCustom = (eventName: string, params?: Record<string, unknown>) => {
    fbTrackCustomEvent(eventName, params)
  }

  const trackLead = (params?: { value?: number; currency?: string }) => {
    fbTrackEvent(FB_EVENTS.LEAD, params)
  }

  const trackSubscribe = (params?: {
    value?: number
    currency?: string
    predicted_ltv?: number
  }) => {
    fbTrackEvent(FB_EVENTS.SUBSCRIBE, params)
  }

  const trackCompleteRegistration = (params?: {
    value?: number
    currency?: string
    content_name?: string
  }) => {
    fbTrackEvent(FB_EVENTS.COMPLETE_REGISTRATION, params)
  }

  return {
    track,
    trackCustom,
    trackLead,
    trackSubscribe,
    trackCompleteRegistration,
  }
}

export default FacebookPixel
