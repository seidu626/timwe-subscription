import type { Metadata, Viewport } from 'next'
import './globals.css'

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || 'https://subscribe.nouveauriche.com'

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 5,
  themeColor: '#4f46e5',
}

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: 'Nouveauriche Subscription - Premium Mobile Services',
    template: '%s | Nouveauriche',
  },
  description: 'Subscribe to premium mobile services with flexible billing. Fast, secure, and reliable subscription management.',
  keywords: ['subscription', 'mobile', 'billing', 'premium services', 'nouveauriche', 'timwe', 'mobile payments'],
  authors: [{ name: 'Nouveauriche', url: siteUrl }],
  creator: 'Nouveauriche',
  publisher: 'Nouveauriche',
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      'max-video-preview': -1,
      'max-image-preview': 'large',
      'max-snippet': -1,
    },
  },
  icons: {
    icon: '/favicon.svg',
    apple: '/apple-touch-icon.png',
  },
  manifest: '/site.webmanifest',
  openGraph: {
    title: 'Nouveauriche Subscription - Premium Mobile Services',
    description: 'Subscribe to premium mobile services with flexible billing.',
    type: 'website',
    locale: 'en_US',
    url: siteUrl,
    siteName: 'Nouveauriche',
    images: [
      {
        url: '/og-image.png',
        width: 1200,
        height: 630,
        alt: 'Nouveauriche Subscription Services',
      },
    ],
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Nouveauriche Subscription - Premium Mobile Services',
    description: 'Subscribe to premium mobile services with flexible billing.',
    images: ['/og-image.png'],
    creator: '@nouveauriche',
  },
  alternates: {
    canonical: siteUrl,
  },
  category: 'technology',
  classification: 'Business',
  other: {
    'apple-mobile-web-app-capable': 'yes',
    'apple-mobile-web-app-status-bar-style': 'default',
    'apple-mobile-web-app-title': 'Nouveauriche',
    'format-detection': 'telephone=no',
  },
}

// Organization structured data for SEO
const organizationSchema = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  name: 'Nouveauriche',
  url: siteUrl,
  logo: `${siteUrl}/logo.png`,
  contactPoint: {
    '@type': 'ContactPoint',
    contactType: 'customer service',
    availableLanguage: ['English'],
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <head>
        {/* Organization Structured Data - static trusted content */}
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{
            __html: JSON.stringify(organizationSchema),
          }}
        />

        {/* Font Preconnect */}
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap"
          rel="stylesheet"
        />

        {/* DNS Prefetch for analytics domains */}
        <link rel="dns-prefetch" href="https://www.googletagmanager.com" />
        <link rel="dns-prefetch" href="https://connect.facebook.net" />
        <link rel="dns-prefetch" href="https://analytics.tiktok.com" />
      </head>
      <body style={{
        margin: 0,
        fontFamily: '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        lineHeight: 1.6,
        color: '#1a1a1a',
        backgroundColor: '#f8f9fa',
        minHeight: '100vh',
      }}>
        <a href="#main-content" className="skip-link">Skip to main content</a>
        <div id="main-content">
          {children}
        </div>
      </body>
    </html>
  )
}
