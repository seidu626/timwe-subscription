'use client'

import React, { useEffect, useState, useMemo, useCallback, Suspense } from 'react'
import { useParams, useSearchParams } from 'next/navigation'
import dynamic from 'next/dynamic'
import ErrorBoundary from '../../components/ErrorBoundary'
import { LandingPageSkeleton } from '../../components/Skeleton'
import { PixelManager, usePixels } from '../../components/pixels'
import {
  useAttribution,
  useScrollTracking,
  useTimeTracking,
  useFormAnalytics,
  captureAttribution,
  storeAttribution,
} from '../../lib/analytics'
import {
  captureBootstrapTokenFromUrl,
  getBootstrapToken,
  clearBootstrapToken,
} from '@/lib/he-bootstrap'
import { HE_BOOTSTRAP_TOKEN_HEADER } from '@/lib/he-types'
import type {
  Campaign,
  TransactionResponse,
  AttributionData,
  ClickIdParam,
  PassthroughParam,
  PixelConfiguration,
} from '../../types'

// Lazy load the analytics debug panel (only used in development)
const AnalyticsDebugPanel = dynamic(
  () => import('../../components/AnalyticsDebugPanel'),
  { ssr: false }
)

// Click ID parameter aliases (Mobplus requirement)
// Canonical: click_id, Aliases: txid, clickid, cid, subid
const CLICK_ID_PARAMS: readonly ClickIdParam[] = ['click_id', 'txid', 'clickid', 'cid', 'subid'] as const

// Additional Mobplus passthrough fields
const PASSTHROUGH_PARAMS: readonly PassthroughParam[] = [
  'campaign_id', 'offer_id', 'adv_id', 'aff_id', 'pub_id',
  'sub1', 'sub2', 'sub3', 'sub4', 'sub5',
  'source', 'creative', 'placement'
] as const

// Phone number validation utility
const validatePhoneNumber = (phone: string): { isValid: boolean; error?: string } => {
  // Remove all non-digit characters except + at the beginning
  const cleaned = phone.replace(/[^\d+]/g, '')

  // Basic validation rules
  if (!cleaned) {
    return { isValid: false, error: 'Phone number is required' }
  }

  // Must start with exactly one + or digit
  if (!cleaned.startsWith('+') && !/^\d/.test(cleaned)) {
    return { isValid: false, error: 'Phone number must start with + or a digit' }
  }

  // Check for multiple + signs
  if ((cleaned.match(/\+/g) || []).length > 1) {
    return { isValid: false, error: 'Phone number can only contain one country code prefix (+)' }
  }

  // If starts with +, must have at least 7 digits after +
  if (cleaned.startsWith('+')) {
    const digitsAfterPlus = cleaned.slice(1).replace(/\D/g, '')
    if (digitsAfterPlus.length < 7) {
      return { isValid: false, error: 'Phone number must have at least 7 digits after the country code' }
    }
    if (digitsAfterPlus.length > 15) {
      return { isValid: false, error: 'Phone number is too long' }
    }
  } else {
    // If no +, must be 7-15 digits
    const digits = cleaned.replace(/\D/g, '')
    if (digits.length < 7) {
      return { isValid: false, error: 'Phone number must have at least 7 digits' }
    }
    if (digits.length > 15) {
      return { isValid: false, error: 'Phone number is too long' }
    }
  }

  return { isValid: true }
}

// Analytics event type mapping for acquisition-api
type AnalyticsEventType = 'landing_view' | 'landing_click' | 'form_submit'

const eventTypeMap: Record<string, AnalyticsEventType | null> = {
  'page_view': 'landing_view',
  'campaign_loaded': null, // Don't send to backend
  'campaign_load_error': null,
  'form_submit_attempt': 'form_submit',
  'form_validation_error': null,
  'transaction_created': null, // Already tracked via transactions endpoint
  'transaction_confirmed': null,
  'subscription_error': null,
}

// Send analytics event to acquisition-api (forwards HE bootstrap token when present)
const sendAnalyticsEvent = async (
  eventType: AnalyticsEventType,
  campaignSlug: string,
  clickId?: string | null,
  adProvider?: string | null,
  sessionId?: string | null,
  referrerDomain?: string | null
) => {
  try {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    const heToken = getBootstrapToken()
    if (heToken) {
      headers[HE_BOOTSTRAP_TOKEN_HEADER] = heToken
    }

    const response = await fetch('/api/analytics/landing', {
      method: 'POST',
      headers,
      body: JSON.stringify({
        event_type: eventType,
        campaign_slug: campaignSlug,
        click_id: clickId || undefined,
        ad_provider: adProvider || undefined,
        session_id: sessionId || undefined,
        referrer_domain: referrerDomain || undefined,
      }),
    })

    if (!response.ok) {
      console.warn('Analytics event failed:', await response.text())
    }
  } catch (error) {
    // Don't let analytics errors affect user experience
    console.warn('Failed to send analytics event:', error)
  }
}

// Get or create session ID for deduplication
const getSessionId = (): string => {
  if (typeof window === 'undefined') return ''
  
  let sessionId = sessionStorage.getItem('landing_session_id')
  if (!sessionId) {
    sessionId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
    sessionStorage.setItem('landing_session_id', sessionId)
  }
  return sessionId
}

// Read a cookie by name (used for click_id fallback when query params are stripped)
const getCookie = (name: string): string | null => {
  if (typeof document === 'undefined') return null
  const value = `; ${document.cookie}`
  const parts = value.split(`; ${name}=`)
  if (parts.length === 2) {
    const cookieValue = parts.pop()?.split(';').shift()
    return cookieValue || null
  }
  return null
}

// Extract referrer domain
const getReferrerDomain = (): string | null => {
  if (typeof window === 'undefined' || !document.referrer) return null
  try {
    const url = new URL(document.referrer)
    return url.hostname
  } catch {
    return null
  }
}

// Simple analytics utility
const trackEvent = (eventName: string, properties: Record<string, any> = {}) => {
  if (typeof window === 'undefined') return

  // Store in localStorage for debugging (dev only)
  if (process.env.NODE_ENV === 'development') {
    const events = JSON.parse(localStorage.getItem('analytics_events') || '[]')
    events.push({
      event: eventName,
      properties,
      timestamp: new Date().toISOString(),
      url: window.location.href
    })
    localStorage.setItem('analytics_events', JSON.stringify(events.slice(-100)))
  }

  // Send to acquisition-api if this event type should be tracked
  const apiEventType = eventTypeMap[eventName]
  if (apiEventType && properties.campaign_slug) {
    sendAnalyticsEvent(
      apiEventType,
      properties.campaign_slug,
      properties.click_id,
      properties.provider,
      getSessionId(),
      getReferrerDomain()
    )
  }
}

// Inner component that uses useSearchParams - must be wrapped in Suspense
function LandingPageWithSearchParams() {
  const params = useParams()
  const searchParams = useSearchParams()
  const slug = params?.slug as string
  const [pixelConfig, setPixelConfig] = useState<PixelConfiguration | undefined>(undefined)

  // Capture and store attribution on initial load
  useEffect(() => {
    const attribution = captureAttribution()
    if (attribution.click_id || attribution.utm_source || attribution.fbclid || attribution.gclid) {
      storeAttribution(attribution, 'last_touch')
    }
  }, [])

  // Capture HE bootstrap token from URL (from HE bootstrap redirect)
  useEffect(() => {
    captureBootstrapTokenFromUrl()
  }, [])

  return (
    <PixelManager config={pixelConfig}>
      <LandingPageContent
        params={params}
        searchParams={searchParams}
        slug={slug}
        onPixelConfigLoad={setPixelConfig}
      />
    </PixelManager>
  )
}

// Main export with Suspense boundary for useSearchParams
// This prevents hydration issues and server action errors in Next.js 14
export default function LandingPage() {
  return (
    <ErrorBoundary>
      <Suspense fallback={<LandingPageSkeleton />}>
        <LandingPageWithSearchParams />
      </Suspense>
    </ErrorBoundary>
  )
}

function LandingPageContent({
  params,
  searchParams,
  slug,
  onPixelConfigLoad,
}: {
  params: any
  searchParams: any
  slug: string
  onPixelConfigLoad?: (config: PixelConfiguration) => void
}) {
  // Pixel tracking context
  const { trackConversion, trackEvent: pixelTrackEvent } = usePixels()

  const [campaign, setCampaign] = useState<Campaign | null>(null)
  const [msisdn, setMsisdn] = useState('')
  const [consentChecked, setConsentChecked] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [transaction, setTransaction] = useState<TransactionResponse | null>(null)
  const [otpCode, setOtpCode] = useState('')
  const [phoneError, setPhoneError] = useState<string | null>(null)
  const [formTouched, setFormTouched] = useState(false)

  // Unified event tracking (backend + pixels)
  const trackEventWithPixels = useCallback((eventName: string, properties: Record<string, any> = {}) => {
    // Original tracking (console + localStorage + backend)
    trackEvent(eventName, properties)
    // Also track via pixels
    pixelTrackEvent(eventName, properties)
  }, [pixelTrackEvent])

  // Scroll depth tracking
  useScrollTracking({
    thresholds: [25, 50, 75, 100],
    onThreshold: (percent) => {
      trackEventWithPixels(`scroll_${percent}`, {
        campaign_slug: slug,
        percent_scrolled: percent,
      })
    },
  })

  // Time on page tracking
  useTimeTracking({
    checkpoints: [30, 60, 120, 300],
    onCheckpoint: (seconds) => {
      trackEventWithPixels(`time_${seconds}s`, {
        campaign_slug: slug,
        seconds_on_page: seconds,
      })
    },
  })

  // Form field analytics
  const formAnalytics = useFormAnalytics({
    onFieldFocus: (fieldName) => {
      if (fieldName === 'msisdn') {
        trackEventWithPixels('form_focus', {
          campaign_slug: slug,
          field: fieldName,
        })
      }
    },
    onFieldBlur: (fieldName, duration, hasValue) => {
      if (fieldName === 'msisdn' && hasValue) {
        trackEventWithPixels('phone_entered', {
          campaign_slug: slug,
          field: fieldName,
          duration_ms: duration,
        })
      }
    },
  })

  // Extract click_id from URL params (canonical + aliases), fallback to cookie
  // Cookie fallback is needed when URL params are stripped during redirects
  const clickId = useMemo(() => {
    // First, try URL params (higher priority)
    if (searchParams) {
      for (const param of CLICK_ID_PARAMS) {
        const value = searchParams.get(param)
        if (value) return value
      }
    }
    // Fallback to cookie (set by /v1/click/out redirect endpoint)
    return getCookie('click_id')
  }, [searchParams])

  // Extract provider from URL, fallback to cookie, default based on click_id presence
  const provider = useMemo(() => {
    // First, check explicit URL param
    if (searchParams) {
      const explicit = searchParams.get('provider')
      if (explicit) return explicit
      // Auto-detect mobplus if txid param is present
      if (searchParams.get('txid')) return 'mobplus'
    }
    // Fallback to cookie (set by /v1/click/out redirect endpoint)
    const cookiePartner = getCookie('click_partner')
    if (cookiePartner) return cookiePartner
    // Default: if click_id present assume mobplus, otherwise generic
    return clickId ? 'mobplus' : 'generic'
  }, [searchParams, clickId])

  // Build attribution data from URL params
  const attributionData = useMemo((): AttributionData => {
    const data: AttributionData = {}
    if (!searchParams) return data

    // Add click_id (canonical)
    if (clickId) {
      data['click_id'] = clickId
    }

    // Add all passthrough params that exist
    for (const param of PASSTHROUGH_PARAMS) {
      const value = searchParams.get(param)
      if (value) {
        data[param] = value
      }
    }

    // Also capture any custom sub* params
    searchParams.forEach((value: string, key: string) => {
      if (key.startsWith('sub') && !data[key]) {
        data[key] = value
      }
    })

    return data
  }, [searchParams, clickId])

  useEffect(() => {
    // Track page view with pixels
    trackEventWithPixels('page_view', {
      campaign_slug: slug,
      click_id: clickId,
      provider: provider
    })

    // Load campaign config via Next.js API route (proxies to acquisition API)
    fetch(`/api/campaigns/${slug}`)
      .then(res => res.json())
      .then(data => {
        setCampaign(data)

        // Load pixel configuration if available
        if (data.tracking_config?.pixels && onPixelConfigLoad) {
          onPixelConfigLoad(data.tracking_config.pixels)
        }

        trackEventWithPixels('campaign_loaded', {
          campaign_slug: slug,
          has_price: !!data.price,
          requires_consent: data.consent_required
        })
      })
      .catch(err => {
        console.error('Failed to load campaign', err)
        setError('Failed to load campaign')
        trackEventWithPixels('campaign_load_error', {
          campaign_slug: slug,
          error: err.message
        })
      })
  }, [slug, clickId, provider, trackEventWithPixels, onPixelConfigLoad])

  // Update document metadata when campaign loads
  useEffect(() => {
    if (campaign) {
      // Update page title
      document.title = `${campaign.slug} - Subscribe Now`

      // Add structured data for SEO
      const structuredData = {
        "@context": "https://schema.org",
        "@type": "WebApplication",
        "name": `Subscribe to ${campaign.slug}`,
        "description": `Subscribe to ${campaign.slug} for ${campaign.price || 'premium'} services`,
        "url": window.location.href,
        "applicationCategory": "BusinessApplication",
        "operatingSystem": "Web Browser",
        "offers": campaign.price ? {
          "@type": "Offer",
          "price": campaign.price.toString().replace(/[^\d.]/g, ''),
          "priceCurrency": "USD",
          "priceValidUntil": new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]
        } : undefined
      }

      // Remove existing structured data
      const existingScript = document.querySelector('script[type="application/ld+json"]')
      if (existingScript) {
        existingScript.remove()
      }

      // Add new structured data
      const script = document.createElement('script')
      script.type = 'application/ld+json'
      script.textContent = JSON.stringify(structuredData)
      document.head.appendChild(script)

      // Update meta description
      let metaDescription = document.querySelector('meta[name="description"]')
      if (!metaDescription) {
        metaDescription = document.createElement('meta')
        metaDescription.setAttribute('name', 'description')
        document.head.appendChild(metaDescription)
      }
      metaDescription.setAttribute('content', `Subscribe to premium mobile services. ${campaign.price ? `Only ${campaign.price} ${campaign.billing_cycle || 'per month'}.` : ''} Fast and secure subscription process.`)

      // Update Open Graph tags
      const updateMetaTag = (property: string, content: string) => {
        let meta = document.querySelector(`meta[property="${property}"]`)
        if (!meta) {
          meta = document.createElement('meta')
          meta.setAttribute('property', property)
          document.head.appendChild(meta)
        }
        meta.setAttribute('content', content)
      }

      updateMetaTag('og:title', `${campaign.slug} - Subscribe Now`)
      updateMetaTag('og:description', `Subscribe to premium mobile services${campaign.price ? ` for only ${campaign.price} ${campaign.billing_cycle || 'per month'}` : ''}`)
      updateMetaTag('og:url', window.location.href)
      updateMetaTag('og:type', 'website')
    }
  }, [campaign])

  // Phone number input handler with validation
  const handlePhoneChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value
    setMsisdn(value)

    if (formTouched) {
      const validation = validatePhoneNumber(value)
      setPhoneError(validation.isValid ? null : validation.error || null)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormTouched(true)

    // Track form submission attempt
    trackEventWithPixels('form_submit_attempt', {
      campaign_slug: slug,
      has_phone: !!msisdn.trim(),
      consent_checked: consentChecked,
      form_time_ms: formAnalytics.getTotalFormTime(),
    })

    // Validate phone number
    const phoneValidation = validatePhoneNumber(msisdn)
    if (!phoneValidation.isValid) {
      setPhoneError(phoneValidation.error || 'Invalid phone number')
      trackEventWithPixels('form_validation_error', {
        campaign_slug: slug,
        error_type: 'phone_validation',
        error_message: phoneValidation.error
      })
      return
    }

    setLoading(true)
    setError(null)
    setPhoneError(null)

    try {
      // Build headers - include HE bootstrap token if available
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      }
      const heToken = getBootstrapToken()
      if (heToken) {
        headers[HE_BOOTSTRAP_TOKEN_HEADER] = heToken
      }

      const response = await fetch(`/api/transactions`, {
        method: 'POST',
        headers,
        body: JSON.stringify({
          campaign_slug: slug,
          msisdn: msisdn,
          provider: provider,
          click_id: clickId,
          attribution_data: attributionData,
          consent_checked: consentChecked,
        }),
      })

      if (!response.ok) {
        const errorData = await response.text()
        throw new Error(errorData || 'Failed to create transaction')
      }

      const data: TransactionResponse = await response.json()
      setTransaction(data)

      // Clear HE bootstrap token after successful use (single-use)
      if (heToken) {
        clearBootstrapToken()
      }

      // Track successful transaction creation (form submit = lead)
      trackEventWithPixels('transaction_created', {
        campaign_slug: slug,
        transaction_id: data.transaction_id,
        correlation_id: data.correlation_id,
        next_action: data.next_action,
        status: data.status
      })

      // Track form submit conversion
      trackConversion({
        event: 'form_submit',
        transaction_id: data.transaction_id,
        content_name: slug,
      })

      // Handle next action
      if (data.next_action === 'OPEN_SMS' && data.payload.sms_link) {
        trackEventWithPixels('sms_redirect', {
          campaign_slug: slug,
          transaction_id: data.transaction_id
        })
        window.location.href = data.payload.sms_link
      } else if (data.next_action === 'OTP') {
        trackEventWithPixels('otp_required', {
          campaign_slug: slug,
          transaction_id: data.transaction_id
        })
        // Show OTP input (state update triggers re-render)
      }
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'An error occurred'
      setError(errorMessage)
      trackEventWithPixels('transaction_error', {
        campaign_slug: slug,
        error: errorMessage,
        msisdn_provided: !!msisdn
      })
    } finally {
      setLoading(false)
    }
  }

  const handleConfirmOTP = async () => {
    if (!transaction) return

    setLoading(true)
    setError(null)

    try {
      const response = await fetch(
        `/api/transactions/${transaction.transaction_id}/confirm`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            transaction_id: transaction.transaction_id,
            auth_code: otpCode,
          }),
        }
      )

      if (!response.ok) {
        const errorData = await response.text()
        throw new Error(errorData || 'Failed to confirm transaction')
      }

      const data = await response.json()
      setTransaction(data)

      trackEventWithPixels('otp_confirmed', {
        campaign_slug: slug,
        transaction_id: data.transaction_id,
        status: data.status
      })

      // Track OTP conversion if subscription is now complete
      if (data.status === 'SUBSCRIBED') {
        trackConversion({
          event: 'subscription_success',
          transaction_id: data.transaction_id,
          content_name: slug,
          value: campaign?.price ? parseFloat(String(campaign.price).replace(/[^\d.]/g, '')) : undefined,
          currency: campaign?.currency || 'USD',
        })
      }
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to confirm'
      setError(errorMessage)
      trackEventWithPixels('otp_confirmation_error', {
        campaign_slug: slug,
        transaction_id: transaction.transaction_id,
        error: errorMessage
      })
    } finally {
      setLoading(false)
    }
  }

  if (!campaign) {
    return <LandingPageSkeleton />
  }

  // Show OTP input if needed
  if (transaction?.next_action === 'OTP') {
    return (
      <div className="otp-section">
        <h1>Enter Confirmation Code</h1>
        <p>{transaction.payload.prompt || 'Please enter the confirmation code sent to your phone'}</p>
        <div className="otp-input-group">
          <input
            type="text"
            value={otpCode}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setOtpCode(e.target.value)}
            placeholder="Enter code"
            className="otp-input form-input"
            maxLength={6}
            autoFocus
          />
          <button
            onClick={handleConfirmOTP}
            disabled={loading || !otpCode.trim()}
            className="btn-primary"
            style={{ width: 'auto', padding: '0.75rem 2rem' }}
          >
            {loading ? (
              <>
                <span className="loading-spinner" style={{ marginRight: '0.5rem' }} />
                Confirming...
              </>
            ) : (
              'Confirm'
            )}
          </button>
        </div>
        {error && <div className="error-message">{error}</div>}
      </div>
    )
  }

  // Show success message
  if (transaction?.status === 'SUBSCRIBED') {
    // Track subscription success (may fire twice but pixels dedupe)
    trackEventWithPixels('subscription_success', {
      campaign_slug: slug,
      transaction_id: transaction.transaction_id
    })

    // Track conversion for pixels
    trackConversion({
      event: 'subscription_success',
      transaction_id: transaction.transaction_id,
      content_name: slug,
      value: campaign?.price ? parseFloat(String(campaign.price).replace(/[^\d.]/g, '')) : undefined,
      currency: campaign?.currency || 'USD',
    })

    return (
      <div className="container">
        <div className="success-message">
          <h1 style={{ color: '#166534', margin: '0 0 1rem 0', fontSize: '2rem' }}>🎉 Success!</h1>
          <p style={{ margin: 0, fontSize: '1.1rem' }}>
            {transaction.payload.message || 'Your subscription has been activated successfully.'}
          </p>
        </div>
        <div style={{ textAlign: 'center', marginTop: '2rem' }}>
          <button
            onClick={() => {
              trackEventWithPixels('subscribe_another_click', { campaign_slug: slug })
              window.location.href = '/'
            }}
            className="btn-secondary"
          >
            Subscribe Another Number
          </button>
        </div>
        {process.env.NODE_ENV === 'development' && <AnalyticsDebugPanel />}
      </div>
    )
  }

  // Main landing page
  return (
    <main className="container" role="main" aria-labelledby="main-heading">
      <div className="header">
        <h1 id="main-heading">Subscribe Now</h1>
        {campaign.price && (
          <div className="price-display" aria-label={`Price: ${campaign.price} ${campaign.billing_cycle || 'per month'}`}>
            {campaign.price} {campaign.billing_cycle || 'per month'}
          </div>
        )}
      </div>

      <form onSubmit={handleSubmit} noValidate aria-describedby={error ? "form-error" : undefined}>
        <div className="form-group">
          <label htmlFor="msisdn" className="form-label">
            Phone Number <span aria-hidden="true">*</span>
          </label>
          <input
            id="msisdn"
            type="tel"
            value={msisdn}
            onChange={handlePhoneChange}
            onFocus={() => formAnalytics.trackFieldFocus('msisdn')}
            onBlur={() => {
              setFormTouched(true)
              formAnalytics.trackFieldBlur('msisdn', !!msisdn.trim())
            }}
            required
            placeholder="Enter your phone number (e.g., 233241234567)"
            className={`form-input ${phoneError ? 'invalid' : ''}`}
            autoComplete="tel"
            inputMode="tel"
            aria-describedby="msisdn-help msisdn-error"
            aria-invalid={phoneError ? "true" : "false"}
          />
          <div id="msisdn-help" className="sr-only">
            Enter your mobile phone number to subscribe to the service
          </div>
          {phoneError && (
            <div id="msisdn-error" className="error-message" role="alert" style={{ marginTop: '0.5rem', fontSize: '0.9rem' }}>
              {phoneError}
            </div>
          )}
        </div>

        {campaign.consent_required && (
          <div className="form-group">
            <div className="checkbox-group">
              <input
                id="consent"
                type="checkbox"
                checked={consentChecked}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  setConsentChecked(e.target.checked)
                  if (e.target.checked) {
                    trackEventWithPixels('terms_accepted', { campaign_slug: slug })
                  }
                }}
                required
                aria-describedby="consent-help"
              />
              <label htmlFor="consent">
                I agree to the{' '}
                <a
                  href={campaign.terms_url || '#'}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="terms-link"
                  aria-label="View Terms and Conditions (opens in new tab)"
                  onClick={() => trackEventWithPixels('terms_viewed', { campaign_slug: slug })}
                >
                  Terms and Conditions
                </a>
              </label>
            </div>
            <div id="consent-help" className="sr-only">
              You must agree to the terms and conditions to proceed with subscription
            </div>
          </div>
        )}

        {error && (
          <div className="error-message" id="form-error" role="alert" aria-live="polite">
            {error}
          </div>
        )}

        <button
          type="submit"
          disabled={loading || (campaign.consent_required && !consentChecked)}
          className="btn-primary"
          aria-describedby={loading ? "loading-status" : undefined}
        >
          {loading ? (
            <>
              <span className="loading-spinner" aria-hidden="true" style={{ marginRight: '0.5rem' }} />
              <span id="loading-status">Processing your subscription...</span>
            </>
          ) : (
            'Subscribe Now'
          )}
        </button>
      </form>

      {campaign.inline_terms_text && (
        <section className="terms-section" aria-labelledby="terms-heading">
          <h3 id="terms-heading">Terms and Conditions</h3>
          <div className="terms-text">{campaign.inline_terms_text}</div>
        </section>
      )}
      {process.env.NODE_ENV === 'development' && <AnalyticsDebugPanel />}
    </main>
  )
}
