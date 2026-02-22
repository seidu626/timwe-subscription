'use client'

import React from 'react'
import type { Campaign, LandingCopyLocale } from '../../types'

const themeColorPattern = /^#[0-9A-Fa-f]{6}$/
const REQUIRED_COPY_FIELDS: ReadonlyArray<keyof LandingCopyLocale> = [
  'heroTitle', 'heDescription', 'heCta', 'heModalTitle', 'heModalConfirm',
  'msisdnDescription', 'msisdnPlaceholder', 'msisdnCta',
  'otpDescription', 'otpPlaceholder', 'otpCta',
  'successTitle', 'successBody',
  'consentPrefix', 'consentTerms', 'termsHeading', 'legal',
  'phoneRequired', 'phoneInvalid', 'otpInvalid', 'consentRequired',
] as const

export function getConfiguredCopy(campaign: Campaign | null): LandingCopyLocale | null {
  if (!campaign?.lp_copy?.en) return null
  for (const key of REQUIRED_COPY_FIELDS) {
    const value = campaign.lp_copy.en[key]
    if (typeof value !== 'string' || value.trim() === '') return null
  }
  return campaign.lp_copy.en
}

export function sanitizeThemeColor(value: string | undefined): string | null {
  if (!value) return null
  const trimmed = value.trim()
  if (!themeColorPattern.test(trimmed)) return null
  return trimmed
}

export function sanitizeBackgroundImageURL(value: string | undefined): string | null {
  if (!value) return null
  const trimmed = value.trim()
  if (!trimmed) return null
  try {
    const parsed = new URL(trimmed)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null
    return parsed.toString()
  } catch {
    return null
  }
}

export function getCampaignVisualStyle(campaign: Campaign | null): React.CSSProperties {
  const style: React.CSSProperties = {}
  const themeColor = sanitizeThemeColor(campaign?.tracking_config?.visual?.theme_color)
  const backgroundImageURL = sanitizeBackgroundImageURL(campaign?.tracking_config?.visual?.background_image_url)

  if (themeColor) {
    ;(style as Record<string, string>)['--campaign-theme-color'] = themeColor
  }

  if (backgroundImageURL) {
    style.backgroundImage = `linear-gradient(rgba(10, 10, 26, 0.72), rgba(10, 10, 26, 0.72)), url('${backgroundImageURL}')`
    style.backgroundSize = 'cover'
    style.backgroundPosition = 'center'
    style.backgroundRepeat = 'no-repeat'
  }

  return style
}
