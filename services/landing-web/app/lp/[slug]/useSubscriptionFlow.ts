'use client'

import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  getBootstrapToken,
  clearBootstrapToken,
} from '@/lib/he-bootstrap'
import { HE_BOOTSTRAP_TOKEN_HEADER } from '@/lib/he-types'
import type {
  AttributionData,
  Campaign,
  FlowStep,
  TransactionResponse,
  AnalyticsEventType,
  PixelConfiguration,
  ConversionEvent,
} from '../../types'

interface UseSubscriptionFlowProps {
  slug: string
  searchParams: URLSearchParams
  trackConversion: (event: ConversionEvent) => void
  pixelTrackEvent: (eventName: string, params?: Record<string, any>) => void
  onPixelConfigLoad?: (config: PixelConfiguration | undefined) => void
}

const CLICK_ID_PARAMS = ['click_id', 'txid', 'clickid', 'cid', 'subid'] as const
const PASSTHROUGH_PARAMS = [
  'campaign_id', 'offer_id', 'adv_id', 'aff_id', 'pub_id',
  'sub1', 'sub2', 'sub3', 'sub4', 'sub5',
  'source', 'creative', 'placement',
  'utm_source', 'utm_medium', 'utm_campaign', 'utm_content', 'utm_term',
  'fbclid', 'gclid', 'ttclid', 'msclkid', 'li_fat_id', 'wbraid', 'gbraid',
] as const

const eventTypeMap: Record<string, AnalyticsEventType | null> = {
  page_view: 'landing_view',
  he_subscribe_click: 'landing_click',
  msisdn_submit: 'form_submit',
  campaign_loaded: null,
  campaign_load_error: null,
  transaction_created: null,
  otp_confirmed: null,
  subscription_success: null,
}

export function useSubscriptionFlow({
  slug,
  searchParams,
  trackConversion,
  pixelTrackEvent,
  onPixelConfigLoad,
}: UseSubscriptionFlowProps) {
  const [step, setStep] = useState<FlowStep>('HE_PROMPT')
  const [showHeModal, setShowHeModal] = useState(false)
  const [campaign, setCampaign] = useState<Campaign | null>(null)
  const [msisdn, setMsisdn] = useState('')
  const [consentChecked, setConsentChecked] = useState(false)
  const [otpCode, setOtpCode] = useState('')
  const [transaction, setTransaction] = useState<TransactionResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const isGhanaCampaign = useMemo(() => campaign?.country?.toUpperCase() === 'GH', [campaign?.country])

  const clickId = useMemo(() => {
    for (const param of CLICK_ID_PARAMS) {
      const value = searchParams.get(param)
      if (value) return value
    }
    return getCookie('click_id')
  }, [searchParams])

  const provider = useMemo(() => {
    const explicit = searchParams.get('provider')
    if (explicit) return explicit
    if (searchParams.get('txid')) return 'mobplus'
    const partnerCookie = getCookie('click_partner')
    if (partnerCookie) return partnerCookie
    return clickId ? 'mobplus' : 'generic'
  }, [searchParams, clickId])

  const attributionData = useMemo((): AttributionData => {
    const data: AttributionData = {}
    if (clickId) data.click_id = clickId
    for (const param of PASSTHROUGH_PARAMS) {
      const value = searchParams.get(param)
      if (value) data[param] = value
    }
    searchParams.forEach((value, key) => {
      if (key.startsWith('sub') && !data[key]) {
        data[key] = value
      }
    })
    return data
  }, [searchParams, clickId])

  const trackEvent = useCallback(
    (name: string, props: Record<string, unknown> = {}) => {
      if (process.env.NODE_ENV === 'development' && typeof window !== 'undefined') {
        const events = JSON.parse(localStorage.getItem('analytics_events') || '[]')
        events.push({
          event: name,
          properties: props,
          timestamp: new Date().toISOString(),
          url: window.location.href,
        })
        localStorage.setItem('analytics_events', JSON.stringify(events.slice(-100)))
      }

      pixelTrackEvent(name, props)

      const mapped = eventTypeMap[name]
      if (mapped && slug) {
        void sendAnalyticsEvent(
          mapped,
          slug,
          typeof props.click_id === 'string' ? props.click_id : clickId,
          typeof props.provider === 'string' ? props.provider : provider,
          getSessionId(),
          getReferrerDomain()
        )
      }
    },
    [pixelTrackEvent, slug, clickId, provider]
  )

  useEffect(() => {
    if (!slug) return

    setError(null)
    trackEvent('page_view', {
      campaign_slug: slug,
      click_id: clickId,
      provider,
    })

    fetch(`/api/campaigns/${slug}`)
      .then(async (response) => {
        if (!response.ok) throw new Error('Campaign not found')
        return response.json()
      })
      .then((data: Campaign) => {
        setCampaign(data)
        if (onPixelConfigLoad) {
          onPixelConfigLoad(data.tracking_config?.pixels)
        }
        trackEvent('campaign_loaded', { campaign_slug: slug })
      })
      .catch((err: Error) => {
        setError(err.message || 'Failed to load campaign')
        trackEvent('campaign_load_error', { campaign_slug: slug, error: err.message })
      })
  }, [slug, clickId, provider, onPixelConfigLoad, trackEvent])

  const submitTransaction = useCallback(async (text: any) => {
    if (!text) {
      setError('Campaign lp_copy is not configured. Please update campaign settings.')
      return
    }

    const phoneError = validatePhoneNumber(msisdn, text, isGhanaCampaign)
    if (phoneError) {
      setError(phoneError)
      return
    }

    const normalizedMsisdn = isGhanaCampaign ? normalizeGhanaMsisdn(msisdn) : msisdn.trim()
    if (!normalizedMsisdn) {
      setError(isGhanaCampaign ? text.phoneInvalid : text.phoneRequired)
      return
    }

    if (campaign?.consent_required && !consentChecked) {
      setError(text.consentRequired)
      return
    }

    setLoading(true)
    setError(null)

    try {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      const heToken = getBootstrapToken()
      if (heToken) headers[HE_BOOTSTRAP_TOKEN_HEADER] = heToken

      trackEvent('msisdn_submit', { campaign_slug: slug, click_id: clickId, provider })

      const response = await fetch('/api/transactions', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          campaign_slug: slug,
          msisdn: normalizedMsisdn,
          provider,
          click_id: clickId,
          attribution_data: attributionData,
          consent_checked: consentChecked,
        }),
      })

      const data = await parseApiResponse<TransactionResponse>(
        response,
        'Failed to create transaction'
      )
      setTransaction(data)

      if (heToken) clearBootstrapToken()

      trackEvent('transaction_created', {
        campaign_slug: slug,
        transaction_id: data.transaction_id,
        next_action: data.next_action,
        status: data.status,
      })

      if (data.next_action === 'OTP') {
        setOtpCode('')
        setStep('OTP_ENTRY')
        return
      }

      if (data.next_action === 'OPEN_SMS' && data.payload?.sms_link) {
        window.location.href = data.payload.sms_link
        return
      }

      if (data.next_action === 'REDIRECT') {
        const redirectURL = data.payload?.redirect_url || data.payload?.url
        if (redirectURL) {
          window.location.href = redirectURL
          return
        }
      }

      if (data.status === 'SUBSCRIBED') {
        trackConversion({
          event: 'subscription_success',
          transaction_id: data.transaction_id,
          content_name: slug,
          value: campaign?.price,
          currency: campaign?.currency || 'USD',
        })
        setStep('SUCCESS')
        return
      }

      setError(data.payload?.message || 'Subscription could not be completed.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An unknown error occurred')
    } finally {
      setLoading(false)
    }
  }, [msisdn, campaign, consentChecked, trackEvent, slug, clickId, provider, attributionData, trackConversion, isGhanaCampaign])

  const handleOtpConfirm = useCallback(async (text: any) => {
    if (!transaction) return

    if (!text) {
      setError('Campaign lp_copy is not configured. Please update campaign settings.')
      return
    }

    const otpError = validateOtp(otpCode, text)
    if (otpError) {
      setError(otpError)
      return
    }

    setLoading(true)
    setError(null)

    try {
      const response = await fetch(`/api/transactions/${transaction.transaction_id}/confirm`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          transaction_id: transaction.transaction_id,
          auth_code: otpCode.trim(),
        }),
      })

      const data = await parseApiResponse<TransactionResponse>(
        response,
        'Failed to confirm OTP'
      )
      setTransaction(data)

      trackEvent('otp_confirmed', {
        campaign_slug: slug,
        transaction_id: data.transaction_id,
        status: data.status,
      })

      if (data.status === 'SUBSCRIBED') {
        trackConversion({
          event: 'subscription_success',
          transaction_id: data.transaction_id,
          content_name: slug,
          value: campaign?.price,
          currency: campaign?.currency || 'USD',
        })
        setStep('SUCCESS')
      } else {
        setTransaction(null)
        setOtpCode('')
        setStep('MSISDN_ENTRY')
        setError(data.payload?.message || 'PIN validation failed. Please try again.')
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'An unknown error occurred'
      if (
        message.toLowerCase().includes('confirm_required') ||
        message.toLowerCase().includes('not in confirm_required')
      ) {
        setTransaction(null)
        setOtpCode('')
        setStep('MSISDN_ENTRY')
      }
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [transaction, otpCode, trackEvent, slug, trackConversion, campaign])

  return {
    step, setStep,
    showHeModal, setShowHeModal,
    campaign,
    msisdn, setMsisdn,
    consentChecked, setConsentChecked,
    otpCode, setOtpCode,
    transaction,
    loading,
    error, setError,
    isGhanaCampaign,
    clickId, provider,
    submitTransaction,
    handleOtpConfirm,
    trackEvent,
    normalizeGhanaLocalInput,
  }
}

// Helper functions (copied from page.tsx)
function normalizeGhanaLocalInput(value: string): string {
  const digits = value.replace(/\D/g, '')
  if (/^233\d{9,}$/.test(digits)) return digits.slice(3, 12)
  if (/^0\d{9,}$/.test(digits)) return digits.slice(1, 10)
  return digits.slice(0, 9)
}

function getSessionId(): string {
  if (typeof window === 'undefined') return ''
  let sessionId = sessionStorage.getItem('landing_session_id')
  if (!sessionId) {
    sessionId = `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`
    sessionStorage.setItem('landing_session_id', sessionId)
  }
  return sessionId
}

function getCookie(name: string): string | null {
  if (typeof document === 'undefined') return null
  const value = `; ${document.cookie}`
  const parts = value.split(`; ${name}=`)
  if (parts.length === 2) return parts.pop()?.split(';').shift() || null
  return null
}

function getReferrerDomain(): string | null {
  if (typeof window === 'undefined' || !document.referrer) return null
  try {
    return new URL(document.referrer).hostname
  } catch {
    return null
  }
}

const GHANA_MOBILE_PREFIXES = new Set(['24', '54', '55', '53', '20', '50', '26', '27', '56', '57'])

function normalizeGhanaMsisdn(value: string): string | null {
  const compact = value.trim().replace(/[\s\-()]/g, '').replace(/^\+/, '')
  if (!/^\d+$/.test(compact)) return null
  let normalized = ''
  if (/^233\d{9}$/.test(compact)) normalized = compact
  else if (/^0\d{9}$/.test(compact)) normalized = `233${compact.slice(1)}`
  else if (/^\d{9}$/.test(compact)) normalized = `233${compact}`
  else return null
  if (!GHANA_MOBILE_PREFIXES.has(normalized.slice(3, 5))) return null
  return normalized
}

function validatePhoneNumber(value: string, copy: any, isGhanaCampaign: boolean): string | null {
  if (value.trim().length === 0) return copy.phoneRequired
  if (isGhanaCampaign) {
    const localDigits = value.replace(/\D/g, '')
    if (localDigits.length !== 9) return copy.phoneInvalid
    if (!normalizeGhanaMsisdn(localDigits)) return copy.phoneInvalid
    return null
  }
  const digits = value.replace(/[^\d+]/g, '').replace(/\D/g, '')
  if (digits.length < 7 || digits.length > 15) return copy.phoneInvalid
  return null
}

function validateOtp(value: string, copy: any): string | null {
  if (!/^\d{4}$/.test(value.trim())) return copy.otpInvalid
  return null
}

async function parseApiResponse<T>(response: Response, fallbackMessage: string): Promise<T> {
  const raw = await response.text()
  let parsed: unknown = null

  if (raw) {
    try {
      parsed = JSON.parse(raw)
    } catch {
      parsed = null
    }
  }

  if (!response.ok) {
    throw new Error(extractErrorMessage(parsed, raw, fallbackMessage))
  }

  if (parsed && typeof parsed === 'object') {
    return parsed as T
  }

  throw new Error(fallbackMessage)
}

function extractErrorMessage(parsed: unknown, raw: string, fallbackMessage: string): string {
  if (parsed && typeof parsed === 'object') {
    const candidate = parsed as Record<string, unknown>
    if (typeof candidate.error === 'string' && candidate.error.trim()) return candidate.error
    if (typeof candidate.message === 'string' && candidate.message.trim()) return candidate.message
    if (candidate.payload && typeof candidate.payload === 'object') {
      const payload = candidate.payload as Record<string, unknown>
      if (typeof payload.message === 'string' && payload.message.trim()) return payload.message
    }
  }
  return raw || fallbackMessage
}

async function sendAnalyticsEvent(
  eventType: AnalyticsEventType,
  campaignSlug: string,
  clickId?: string | null,
  adProvider?: string | null,
  sessionId?: string | null,
  referrerDomain?: string | null
): Promise<void> {
  try {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    const heToken = getBootstrapToken()
    if (heToken) headers[HE_BOOTSTRAP_TOKEN_HEADER] = heToken
    await fetch('/api/analytics/landing', {
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
  } catch {}
}
