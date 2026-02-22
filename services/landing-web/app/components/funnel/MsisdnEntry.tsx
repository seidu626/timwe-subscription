'use client'

import React from 'react'
import type { Campaign, LandingCopyLocale } from '../../types'

interface MsisdnEntryProps {
  text: LandingCopyLocale
  msisdn: string
  setMsisdn: (value: string) => void
  consentChecked: boolean
  setConsentChecked: (value: boolean) => void
  loading: boolean
  onSubmit: () => Promise<void>
  stepPrice: string
  campaign: Campaign
  isGhanaCampaign: boolean
  normalizeGhanaLocalInput: (value: string) => string
}

export function MsisdnEntry({
  text,
  msisdn,
  setMsisdn,
  consentChecked,
  setConsentChecked,
  loading,
  onSubmit,
  stepPrice,
  campaign,
  isGhanaCampaign,
  normalizeGhanaLocalInput,
}: MsisdnEntryProps) {
  return (
    <section className="lp-block animate-in">
      <p className="lp-copy">{text.msisdnDescription}</p>
      <form
        onSubmit={(event) => {
          event.preventDefault()
          void onSubmit()
        }}
        className="lp-form"
      >
        <div className="lp-field-wrapper">
          <svg className="lp-field-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <rect x="5" y="2" width="14" height="20" rx="2" ry="2" />
            <line x1="12" y1="18" x2="12.01" y2="18" />
          </svg>
          {isGhanaCampaign && <span className="lp-country-prefix">233</span>}
          <input
            type="tel"
            value={msisdn}
            onChange={(event) => {
              const nextValue = event.target.value
              if (isGhanaCampaign) {
                setMsisdn(normalizeGhanaLocalInput(nextValue))
                return
              }
              setMsisdn(nextValue)
            }}
            className={`lp-input ${isGhanaCampaign ? 'lp-input-with-prefix' : 'lp-input-with-icon'}`}
            placeholder={text.msisdnPlaceholder}
            autoComplete="tel"
            inputMode="tel"
            maxLength={isGhanaCampaign ? 9 : undefined}
            disabled={loading}
          />
        </div>

        {campaign.consent_required && (
          <label className="lp-consent">
            <input
              type="checkbox"
              checked={consentChecked}
              onChange={(event) => setConsentChecked(event.target.checked)}
              disabled={loading}
            />
            <span>
              {text.consentPrefix}{' '}
              <a href={campaign.terms_url || '#'} target="_blank" rel="noopener noreferrer">
                {text.consentTerms}
              </a>
            </span>
          </label>
        )}

        <button type="submit" className="lp-primary-btn" disabled={loading}>
          {loading ? (
            <div className="flex items-center justify-center gap-2">
              <span className="loading-spinner" />
              <span>{text.msisdnCta}</span>
            </div>
          ) : (
            text.msisdnCta
          )}
        </button>
        <p className="lp-price">{stepPrice}</p>
      </form>
    </section>
  )
}
