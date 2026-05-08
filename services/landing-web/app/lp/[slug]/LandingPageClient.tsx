'use client'

import React, { Suspense, useEffect, useMemo, useState } from 'react'
import dynamic from 'next/dynamic'
import { useParams, useSearchParams } from 'next/navigation'
import ErrorBoundary from '../../components/ErrorBoundary'
import { LandingPageSkeleton } from '../../components/Skeleton'
import { PixelManager, usePixels } from '../../components/pixels'
import { captureBootstrapTokenFromUrl } from '@/lib/he-bootstrap'
import type { PixelConfiguration } from '../../types'

// Funnel Components
import { HEPrompt } from '../../components/funnel/HEPrompt'
import { MsisdnEntry } from '../../components/funnel/MsisdnEntry'
import { OtpEntry } from '../../components/funnel/OtpEntry'
import { SuccessState } from '../../components/funnel/SuccessState'

// Logic & Helpers
import { useSubscriptionFlow } from './useSubscriptionFlow'
import { getConfiguredCopy, getCampaignVisualStyle } from './visual-helpers'

const AnalyticsDebugPanel = dynamic(() => import('../../components/AnalyticsDebugPanel'), { ssr: false })

function LandingPageWithSearchParams(): React.JSX.Element {
  const params = useParams()
  const searchParams = useSearchParams()
  const [pixelConfig, setPixelConfig] = useState<PixelConfiguration | undefined>(undefined)

  const slugValue = params?.slug
  const tenantValue = params?.tenant
  const slug = typeof slugValue === 'string' ? slugValue : Array.isArray(slugValue) ? slugValue[0] : ''
  const tenantKey = typeof tenantValue === 'string' ? tenantValue : Array.isArray(tenantValue) ? tenantValue[0] : ''

  useEffect(() => {
    captureBootstrapTokenFromUrl()
  }, [])

  return (
    <PixelManager config={pixelConfig}>
      <LandingPageContent 
        slug={slug} 
        tenantKey={tenantKey}
        searchParams={searchParams} 
        onPixelConfigLoad={setPixelConfig} 
      />
    </PixelManager>
  )
}

export default function LandingPage(): React.JSX.Element {
  return (
    <ErrorBoundary>
      <Suspense fallback={<LandingPageSkeleton />}>
        <LandingPageWithSearchParams />
      </Suspense>
    </ErrorBoundary>
  )
}

function LandingPageContent({
  slug,
  tenantKey,
  searchParams,
  onPixelConfigLoad,
}: {
  slug: string
  tenantKey?: string
  searchParams: URLSearchParams
  onPixelConfigLoad?: (config: PixelConfiguration | undefined) => void
}): React.JSX.Element {
  const { trackConversion, trackEvent: pixelTrackEvent } = usePixels()
  
  const {
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
  } = useSubscriptionFlow({
    slug,
    tenantKey,
    searchParams,
    trackConversion,
    pixelTrackEvent,
    onPixelConfigLoad,
  })

  const text = useMemo(() => getConfiguredCopy(campaign), [campaign])
  const campaignVisualStyle = useMemo(() => getCampaignVisualStyle(campaign), [campaign])

  if (!campaign) {
    return <LandingPageSkeleton />
  }

  if (!text) {
    return (
      <main className="lp-template-shell" style={campaignVisualStyle}>
        <section className="lp-template-card">
          <section className="lp-block">
            <h1>Campaign configuration error</h1>
            <p className="lp-copy">This campaign is missing `lp_copy.en` text configuration.</p>
          </section>
        </section>
      </main>
    )
  }

  const stepPrice = campaign.price
    ? `${campaign.price} ${campaign.billing_cycle ? `/ ${campaign.billing_cycle}` : ''}`
    : 'GHS / Day (auto-renewal)'

  return (
    <main className="lp-template-shell" style={campaignVisualStyle}>
      <section className="lp-template-card">
        <header className="lp-template-hero">
          <h1>{text.heroTitle}</h1>
        </header>

        {step === 'HE_PROMPT' && (
          <HEPrompt 
            text={text} 
            stepPrice={stepPrice} 
            loading={loading} 
            onAction={() => {
              setShowHeModal(true)
              trackEvent('he_subscribe_click', {
                campaign_slug: slug,
                click_id: clickId,
                provider,
              })
            }} 
          />
        )}

        {step === 'MSISDN_ENTRY' && (
          <MsisdnEntry
            text={text}
            msisdn={msisdn}
            setMsisdn={setMsisdn}
            consentChecked={consentChecked}
            setConsentChecked={setConsentChecked}
            loading={loading}
            onSubmit={() => submitTransaction(text)}
            stepPrice={stepPrice}
            campaign={campaign}
            isGhanaCampaign={isGhanaCampaign}
            normalizeGhanaLocalInput={normalizeGhanaLocalInput}
          />
        )}

        {step === 'OTP_ENTRY' && (
          <OtpEntry
            text={text}
            otpCode={otpCode}
            setOtpCode={setOtpCode}
            loading={loading}
            onSubmit={() => handleOtpConfirm(text)}
          />
        )}

        {step === 'SUCCESS' && (
          <SuccessState text={text} transaction={transaction} />
        )}

        {campaign.inline_terms_text && (
          <section className="lp-inline-terms">
            <h3>{text.termsHeading}</h3>
            <p>{campaign.inline_terms_text}</p>
          </section>
        )}

        {error && (
          <div className="lp-error animate-in">
            {error}
            <button
              type="button"
              className="lp-error-dismiss"
              onClick={() => setError(null)}
              aria-label="Dismiss error"
            >
              &times;
            </button>
          </div>
        )}
      </section>

      <footer className="lp-legal">
        <p>{text.legal}</p>
      </footer>

      {showHeModal && (
        <div className="lp-modal-overlay" role="dialog" aria-modal="true">
          <div className="lp-modal animate-in">
            <button type="button" className="lp-modal-close" onClick={() => setShowHeModal(false)} aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
            <p>{text.heModalTitle}</p>
            <button
              type="button"
              className="lp-primary-btn"
              onClick={() => {
                setShowHeModal(false)
                setStep('MSISDN_ENTRY')
              }}
            >
              {text.heModalConfirm}
            </button>
          </div>
        </div>
      )}

      {process.env.NODE_ENV === 'development' && <AnalyticsDebugPanel />}
    </main>
  )
}
