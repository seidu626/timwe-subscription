'use client'

import React from 'react'
import type { LandingCopyLocale, TransactionResponse } from '../../types'

interface ConfirmPromptProps {
  text: LandingCopyLocale
  transaction: TransactionResponse | null
  loading: boolean
  onSubmit: () => Promise<void>
}

const DEFAULT_DESCRIPTION = 'One step left. Confirm to activate your subscription.'
const DEFAULT_CTA = 'Confirm subscription'

/**
 * Double opt-in confirmation. Unlike OtpEntry there is no code to collect - the
 * provider holds the subscription pre-active and a bare confirm request
 * activates it.
 */
export function ConfirmPrompt({
  text,
  transaction,
  loading,
  onSubmit,
}: ConfirmPromptProps) {
  const description =
    text.confirmDescription?.trim() ||
    transaction?.payload?.prompt?.trim() ||
    DEFAULT_DESCRIPTION
  const cta = text.confirmCta?.trim() || DEFAULT_CTA

  return (
    <section className="lp-block animate-in">
      <p className="lp-copy">{description}</p>
      <form
        onSubmit={(event) => {
          event.preventDefault()
          void onSubmit()
        }}
        className="lp-form"
      >
        <button type="submit" className="lp-primary-btn" disabled={loading}>
          {loading ? (
            <div className="flex items-center justify-center gap-2">
              <span className="loading-spinner" />
              <span>{cta}</span>
            </div>
          ) : (
            cta
          )}
        </button>
      </form>
    </section>
  )
}
