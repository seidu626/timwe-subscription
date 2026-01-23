'use client'

import { useEffect, useRef, useCallback } from 'react'
import type { AttributionData, FunnelEvent, UTMParameters, AdPlatformClickIds } from '@/app/types'

// ============================================
// Constants
// ============================================

const ATTRIBUTION_STORAGE_KEY = 'lp_attribution'
const SESSION_ID_KEY = 'lp_session_id'
const FIRST_TOUCH_KEY = 'lp_first_touch'

// Click ID parameter aliases
const CLICK_ID_PARAMS = ['click_id', 'txid', 'clickid', 'cid', 'subid'] as const

// UTM parameter names
const UTM_PARAMS = ['utm_source', 'utm_medium', 'utm_campaign', 'utm_content', 'utm_term'] as const

// Ad platform click ID params
const AD_PLATFORM_PARAMS = ['fbclid', 'gclid', 'ttclid', 'msclkid', 'li_fat_id', 'wbraid', 'gbraid'] as const

// Passthrough params
const PASSTHROUGH_PARAMS = [
  'campaign_id', 'offer_id', 'adv_id', 'aff_id',
  'sub1', 'sub2', 'sub3', 'sub4', 'sub5',
  'source', 'creative', 'placement'
] as const

// ============================================
// Session Management
// ============================================

/**
 * Get or create a session ID
 */
export function getSessionId(): string {
  if (typeof window === 'undefined') return ''

  let sessionId = sessionStorage.getItem(SESSION_ID_KEY)
  if (!sessionId) {
    sessionId = `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`
    sessionStorage.setItem(SESSION_ID_KEY, sessionId)
  }
  return sessionId
}

// ============================================
// Attribution Capture
// ============================================

/**
 * Capture all attribution parameters from URL
 */
export function captureAttribution(): AttributionData {
  if (typeof window === 'undefined') {
    return {}
  }

  const params = new URLSearchParams(window.location.search)
  const attribution: AttributionData = {}

  // Capture click ID (with aliases)
  for (const param of CLICK_ID_PARAMS) {
    const value = params.get(param)
    if (value) {
      attribution.click_id = value
      break
    }
  }

  // Capture UTM parameters
  for (const param of UTM_PARAMS) {
    const value = params.get(param)
    if (value) {
      attribution[param] = value
    }
  }

  // Capture ad platform click IDs
  for (const param of AD_PLATFORM_PARAMS) {
    const value = params.get(param)
    if (value) {
      attribution[param] = value
    }
  }

  // Capture passthrough params
  for (const param of PASSTHROUGH_PARAMS) {
    const value = params.get(param)
    if (value) {
      attribution[param] = value
    }
  }

  // Capture any additional sub* params
  params.forEach((value, key) => {
    if (key.startsWith('sub') && !attribution[key]) {
      attribution[key] = value
    }
  })

  // Add session metadata
  attribution.session_id = getSessionId()
  attribution.landing_url = window.location.href
  attribution.referrer = document.referrer || undefined
  attribution.user_agent = navigator.userAgent

  return attribution
}

/**
 * Store attribution with first-touch / last-touch logic
 */
export function storeAttribution(
  attribution: AttributionData,
  model: 'first_touch' | 'last_touch' = 'last_touch'
): void {
  if (typeof window === 'undefined') return

  const now = new Date().toISOString()

  // Check for existing first touch
  const existingFirstTouch = localStorage.getItem(FIRST_TOUCH_KEY)

  if (!existingFirstTouch) {
    // First visit - store as first touch
    attribution.first_touch_at = now
    localStorage.setItem(FIRST_TOUCH_KEY, JSON.stringify(attribution))
  }

  // Always update last touch timestamp
  attribution.last_touch_at = now

  // Store based on model
  if (model === 'last_touch' || !existingFirstTouch) {
    sessionStorage.setItem(ATTRIBUTION_STORAGE_KEY, JSON.stringify(attribution))

    // Also store in cookie for cross-page persistence
    const expires = new Date()
    expires.setDate(expires.getDate() + 7)
    document.cookie = `${ATTRIBUTION_STORAGE_KEY}=${encodeURIComponent(
      JSON.stringify(attribution)
    )}; expires=${expires.toUTCString()}; path=/; SameSite=Lax`
  }
}

/**
 * Get stored attribution
 */
export function getAttribution(): AttributionData | null {
  if (typeof window === 'undefined') return null

  // Try sessionStorage first
  try {
    const session = sessionStorage.getItem(ATTRIBUTION_STORAGE_KEY)
    if (session) return JSON.parse(session)
  } catch {}

  // Try cookie
  try {
    const cookies = document.cookie.split(';')
    for (const cookie of cookies) {
      const [name, value] = cookie.trim().split('=')
      if (name === ATTRIBUTION_STORAGE_KEY && value) {
        return JSON.parse(decodeURIComponent(value))
      }
    }
  } catch {}

  return null
}

/**
 * Get first touch attribution
 */
export function getFirstTouchAttribution(): AttributionData | null {
  if (typeof window === 'undefined') return null

  try {
    const data = localStorage.getItem(FIRST_TOUCH_KEY)
    if (data) return JSON.parse(data)
  } catch {}

  return null
}

// ============================================
// Attribution Hook
// ============================================

/**
 * Hook to capture and store attribution on page load
 */
export function useAttribution(
  model: 'first_touch' | 'last_touch' = 'last_touch'
): AttributionData | null {
  const attributionRef = useRef<AttributionData | null>(null)

  useEffect(() => {
    const attribution = captureAttribution()

    // Only store if we have meaningful attribution
    const hasAttribution =
      attribution.click_id ||
      attribution.utm_source ||
      attribution.fbclid ||
      attribution.gclid ||
      attribution.ttclid

    if (hasAttribution) {
      storeAttribution(attribution, model)
      attributionRef.current = attribution
    } else {
      // Try to get existing attribution
      attributionRef.current = getAttribution()
    }
  }, [model])

  return attributionRef.current
}

// ============================================
// Scroll Tracking Hook
// ============================================

interface ScrollTrackingOptions {
  thresholds?: number[]
  onThreshold?: (percent: number) => void
}

/**
 * Hook to track scroll depth
 */
export function useScrollTracking(options: ScrollTrackingOptions = {}) {
  const { thresholds = [25, 50, 75, 100], onThreshold } = options
  const trackedRef = useRef<Set<number>>(new Set())

  useEffect(() => {
    const handleScroll = () => {
      const scrollHeight = document.documentElement.scrollHeight - window.innerHeight
      if (scrollHeight <= 0) return

      const scrollPercent = Math.round((window.scrollY / scrollHeight) * 100)

      for (const threshold of thresholds) {
        if (scrollPercent >= threshold && !trackedRef.current.has(threshold)) {
          trackedRef.current.add(threshold)
          onThreshold?.(threshold)
        }
      }
    }

    window.addEventListener('scroll', handleScroll, { passive: true })

    // Check initial scroll position
    handleScroll()

    return () => window.removeEventListener('scroll', handleScroll)
  }, [thresholds, onThreshold])

  return trackedRef.current
}

// ============================================
// Time on Page Hook
// ============================================

interface TimeTrackingOptions {
  checkpoints?: number[] // seconds
  onCheckpoint?: (seconds: number) => void
  intervalMs?: number
}

/**
 * Hook to track time spent on page
 */
export function useTimeTracking(options: TimeTrackingOptions = {}) {
  const {
    checkpoints = [30, 60, 120, 300],
    onCheckpoint,
    intervalMs = 5000
  } = options

  const trackedRef = useRef<Set<number>>(new Set())
  const startTimeRef = useRef<number>(Date.now())

  useEffect(() => {
    startTimeRef.current = Date.now()

    const interval = setInterval(() => {
      const elapsed = Math.floor((Date.now() - startTimeRef.current) / 1000)

      for (const checkpoint of checkpoints) {
        if (elapsed >= checkpoint && !trackedRef.current.has(checkpoint)) {
          trackedRef.current.add(checkpoint)
          onCheckpoint?.(checkpoint)
        }
      }
    }, intervalMs)

    return () => clearInterval(interval)
  }, [checkpoints, onCheckpoint, intervalMs])

  return trackedRef.current
}

// ============================================
// Form Analytics Hook
// ============================================

interface FormAnalyticsOptions {
  onFieldFocus?: (fieldName: string) => void
  onFieldBlur?: (fieldName: string, duration: number, hasValue: boolean) => void
  onFieldError?: (fieldName: string, error: string) => void
}

interface FormAnalyticsReturn {
  trackFieldFocus: (fieldName: string) => void
  trackFieldBlur: (fieldName: string, hasValue: boolean) => void
  trackFieldError: (fieldName: string, error: string) => void
  getFieldDuration: (fieldName: string) => number
  getAbandonedFields: () => string[]
  getTotalFormTime: () => number
}

/**
 * Hook to track form field interactions
 */
export function useFormAnalytics(options: FormAnalyticsOptions = {}): FormAnalyticsReturn {
  const { onFieldFocus, onFieldBlur, onFieldError } = options

  const fieldTimesRef = useRef<Map<string, number>>(new Map())
  const fieldDurationsRef = useRef<Map<string, number>>(new Map())
  const abandonedFieldsRef = useRef<string[]>([])
  const formStartRef = useRef<number | null>(null)

  const trackFieldFocus = useCallback((fieldName: string) => {
    if (!formStartRef.current) {
      formStartRef.current = Date.now()
    }
    fieldTimesRef.current.set(fieldName, Date.now())
    onFieldFocus?.(fieldName)
  }, [onFieldFocus])

  const trackFieldBlur = useCallback((fieldName: string, hasValue: boolean) => {
    const startTime = fieldTimesRef.current.get(fieldName)
    if (startTime) {
      const duration = Date.now() - startTime
      fieldDurationsRef.current.set(fieldName, duration)

      if (!hasValue) {
        abandonedFieldsRef.current.push(fieldName)
      }

      onFieldBlur?.(fieldName, duration, hasValue)
    }
  }, [onFieldBlur])

  const trackFieldError = useCallback((fieldName: string, error: string) => {
    onFieldError?.(fieldName, error)
  }, [onFieldError])

  const getFieldDuration = useCallback((fieldName: string): number => {
    return fieldDurationsRef.current.get(fieldName) || 0
  }, [])

  const getAbandonedFields = useCallback((): string[] => {
    return [...abandonedFieldsRef.current]
  }, [])

  const getTotalFormTime = useCallback((): number => {
    if (!formStartRef.current) return 0
    return Date.now() - formStartRef.current
  }, [])

  return {
    trackFieldFocus,
    trackFieldBlur,
    trackFieldError,
    getFieldDuration,
    getAbandonedFields,
    getTotalFormTime,
  }
}

// ============================================
// Visibility Tracking Hook
// ============================================

/**
 * Hook to track page visibility changes
 */
export function useVisibilityTracking(
  onVisibilityChange?: (isVisible: boolean) => void
) {
  const hiddenTimeRef = useRef<number | null>(null)
  const totalHiddenRef = useRef<number>(0)

  useEffect(() => {
    const handleVisibilityChange = () => {
      const isVisible = document.visibilityState === 'visible'

      if (!isVisible) {
        hiddenTimeRef.current = Date.now()
      } else if (hiddenTimeRef.current) {
        totalHiddenRef.current += Date.now() - hiddenTimeRef.current
        hiddenTimeRef.current = null
      }

      onVisibilityChange?.(isVisible)
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [onVisibilityChange])

  return {
    getTotalHiddenTime: () => totalHiddenRef.current,
    isCurrentlyVisible: () => document.visibilityState === 'visible',
  }
}

// ============================================
// Combined Analytics Hook
// ============================================

interface UseAnalyticsOptions {
  campaignSlug: string
  onEvent?: (event: FunnelEvent | string, params?: Record<string, unknown>) => void
}

/**
 * Combined analytics hook with all tracking capabilities
 */
export function useAnalytics(options: UseAnalyticsOptions) {
  const { campaignSlug, onEvent } = options

  // Capture attribution on mount
  const attribution = useAttribution()

  // Scroll tracking
  useScrollTracking({
    onThreshold: (percent) => {
      onEvent?.(`scroll_${percent}` as FunnelEvent, {
        campaign_slug: campaignSlug,
        percent_scrolled: percent,
      })
    },
  })

  // Time tracking
  useTimeTracking({
    onCheckpoint: (seconds) => {
      onEvent?.(`time_${seconds}s` as FunnelEvent, {
        campaign_slug: campaignSlug,
        seconds_on_page: seconds,
      })
    },
  })

  // Form analytics
  const formAnalytics = useFormAnalytics({
    onFieldFocus: (fieldName) => {
      if (fieldName === 'msisdn' || fieldName === 'phone') {
        onEvent?.('form_focus', {
          campaign_slug: campaignSlug,
          field: fieldName,
        })
      }
    },
    onFieldBlur: (fieldName, duration, hasValue) => {
      if (fieldName === 'msisdn' || fieldName === 'phone') {
        if (hasValue) {
          onEvent?.('phone_entered', {
            campaign_slug: campaignSlug,
            field: fieldName,
            duration_ms: duration,
          })
        }
      }
    },
  })

  // Track specific events
  const trackFormSubmit = useCallback(() => {
    onEvent?.('form_submit', {
      campaign_slug: campaignSlug,
      form_time_ms: formAnalytics.getTotalFormTime(),
      abandoned_fields: formAnalytics.getAbandonedFields(),
    })
  }, [campaignSlug, formAnalytics, onEvent])

  const trackSubscriptionSuccess = useCallback((transactionId: string) => {
    onEvent?.('subscription_success', {
      campaign_slug: campaignSlug,
      transaction_id: transactionId,
      attribution,
    })
  }, [campaignSlug, attribution, onEvent])

  const trackSubscriptionError = useCallback((error: string) => {
    onEvent?.('subscription_error', {
      campaign_slug: campaignSlug,
      error,
    })
  }, [campaignSlug, onEvent])

  const trackOtpRequested = useCallback(() => {
    onEvent?.('otp_requested', {
      campaign_slug: campaignSlug,
    })
  }, [campaignSlug, onEvent])

  const trackOtpEntered = useCallback(() => {
    onEvent?.('otp_entered', {
      campaign_slug: campaignSlug,
    })
  }, [campaignSlug, onEvent])

  const trackTermsViewed = useCallback(() => {
    onEvent?.('terms_viewed', {
      campaign_slug: campaignSlug,
    })
  }, [campaignSlug, onEvent])

  const trackTermsAccepted = useCallback(() => {
    onEvent?.('terms_accepted', {
      campaign_slug: campaignSlug,
    })
  }, [campaignSlug, onEvent])

  return {
    attribution,
    formAnalytics,
    trackFormSubmit,
    trackSubscriptionSuccess,
    trackSubscriptionError,
    trackOtpRequested,
    trackOtpEntered,
    trackTermsViewed,
    trackTermsAccepted,
  }
}

export default useAnalytics
