'use client'

import React from 'react'
import type { LandingCopyLocale } from '../../types'

interface HEPromptProps {
  text: LandingCopyLocale
  stepPrice: string
  loading: boolean
  onAction: () => void
}

export function HEPrompt({ text, stepPrice, loading, onAction }: HEPromptProps) {
  return (
    <section className="lp-block animate-in">
      <p className="lp-copy">{text.heDescription}</p>
      <button
        type="button"
        className="lp-primary-btn"
        onClick={onAction}
        disabled={loading}
      >
        {text.heCta}
      </button>
      <p className="lp-price">{stepPrice}</p>
    </section>
  )
}
