'use client'

import React from 'react'
import type { LandingCopyLocale, TransactionResponse } from '../../types'

interface SuccessStateProps {
  text: LandingCopyLocale
  transaction: TransactionResponse | null
}

export function SuccessState({ text, transaction }: SuccessStateProps) {
  return (
    <section className="lp-block success animate-in flex flex-col items-center">
      <div className="mb-6 relative">
        <div className="absolute inset-0 bg-yellow-400 opacity-20 blur-2xl rounded-full"></div>
        <svg className="lp-success-icon relative z-10" viewBox="0 0 48 48" width="80" height="80" aria-hidden="true">
          <circle cx="24" cy="24" r="24" fill="var(--campaign-theme-color, #ffcc00)" />
          <path d="M14 24l7 7 13-13" stroke="#1a1a1a" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" fill="none" />
        </svg>
      </div>
      <h2 className="text-3xl font-bold mb-2 tracking-tight">{text.successTitle}</h2>
      <p className="text-slate-600 max-w-xs mx-auto leading-relaxed">
        {text.successBody || transaction?.payload?.message}
      </p>
    </section>
  )
}
