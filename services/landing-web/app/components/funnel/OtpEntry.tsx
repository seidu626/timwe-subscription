'use client'

import React from 'react'
import type { LandingCopyLocale } from '../../types'

interface OtpEntryProps {
  text: LandingCopyLocale
  otpCode: string
  setOtpCode: (value: string) => void
  loading: boolean
  onSubmit: () => Promise<void>
}

export function OtpEntry({
  text,
  otpCode,
  setOtpCode,
  loading,
  onSubmit,
}: OtpEntryProps) {
  return (
    <section className="lp-block animate-in">
      <p className="lp-copy">{text.otpDescription}</p>
      <form
        onSubmit={(event) => {
          event.preventDefault()
          void onSubmit()
        }}
        className="lp-form"
      >
        <div className="lp-field-wrapper">
          <svg className="lp-field-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
          <input
            type="text"
            value={otpCode}
            onChange={(event) => setOtpCode(event.target.value.replace(/\D/g, '').slice(0, 4))}
            className="lp-input lp-otp-input lp-input-with-icon"
            placeholder={text.otpPlaceholder}
            inputMode="numeric"
            maxLength={4}
            disabled={loading}
          />
        </div>
        <button type="submit" className="lp-primary-btn" disabled={loading}>
          {loading ? (
            <div className="flex items-center justify-center gap-2">
              <span className="loading-spinner" />
              <span>{text.otpCta}</span>
            </div>
          ) : (
            text.otpCta
          )}
        </button>
      </form>
    </section>
  )
}
