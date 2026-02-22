'use client'

import React from 'react'

interface SkeletonProps {
  width?: string | number
  height?: string | number
  borderRadius?: string | number
  className?: string
  style?: React.CSSProperties
}

export function Skeleton({
  width = '100%',
  height = '1rem',
  borderRadius = '4px',
  className = '',
  style = {}
}: SkeletonProps) {
  return (
    <div
      className={`skeleton ${className}`}
      style={{
        width: typeof width === 'number' ? `${width}px` : width,
        height: typeof height === 'number' ? `${height}px` : height,
        borderRadius: typeof borderRadius === 'number' ? `${borderRadius}px` : borderRadius,
        background: 'linear-gradient(90deg, #e5e7eb 25%, #f3f4f6 50%, #e5e7eb 75%)',
        backgroundSize: '200% 100%',
        animation: 'shimmer 1.5s ease-in-out infinite',
        ...style
      }}
      aria-hidden="true"
    />
  )
}

export function SkeletonText({
  lines = 3,
  className = ''
}: {
  lines?: number
  className?: string
}) {
  return (
    <div className={className}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          width={i === lines - 1 ? '70%' : '100%'}
          height="1rem"
          style={{ marginBottom: i === lines - 1 ? 0 : '0.5rem' }}
        />
      ))}
    </div>
  )
}

export function SkeletonButton({ className = '' }: { className?: string }) {
  return (
    <Skeleton
      width="100%"
      height="3rem"
      borderRadius="8px"
      className={className}
    />
  )
}

export function SkeletonInput({ className = '' }: { className?: string }) {
  return (
    <div className={className} style={{ marginBottom: '1.5rem' }}>
      <Skeleton width="30%" height="1rem" style={{ marginBottom: '0.5rem' }} />
      <Skeleton width="100%" height="3rem" borderRadius="8px" />
    </div>
  )
}

function MtnShimmer({ width = '100%', height = '1rem', borderRadius = '8px', style = {} }: {
  width?: string
  height?: string
  borderRadius?: string
  style?: React.CSSProperties
}) {
  return (
    <div
      aria-hidden="true"
      style={{
        width,
        height,
        borderRadius,
        background: 'linear-gradient(90deg, #1a1a2e 25%, #2a2a4a 50%, #1a1a2e 75%)',
        backgroundSize: '200% 100%',
        animation: 'shimmer 1.5s ease-in-out infinite',
        ...style,
      }}
    />
  )
}

export function LandingPageSkeleton() {
  return (
    <div
      className="lp-template-shell"
      role="status"
      aria-label="Loading campaign"
      style={{ minHeight: '100vh', padding: '24px 12px 80px', background: '#0a0a1a' }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: 560,
          margin: '0 auto',
          background: '#ffffff',
          borderTop: '6px solid #ffcc00',
          borderRadius: '0 0 16px 16px',
          padding: '22px 18px',
          boxShadow: '0 20px 30px rgba(0,0,0,0.25)',
        }}
      >
        {/* Hero title */}
        <MtnShimmer width="70%" height="2rem" style={{ margin: '0 auto 16px' }} />

        {/* Description */}
        <MtnShimmer width="90%" height="1rem" style={{ margin: '0 auto 8px' }} />
        <MtnShimmer width="60%" height="1rem" style={{ margin: '0 auto 20px' }} />

        {/* Phone input */}
        <MtnShimmer width="100%" height="48px" borderRadius="13px" style={{ marginBottom: '12px' }} />

        {/* Consent row */}
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12 }}>
          <MtnShimmer width="18px" height="18px" borderRadius="4px" />
          <MtnShimmer width="80%" height="1rem" />
        </div>

        {/* CTA button */}
        <MtnShimmer
          width="100%"
          height="50px"
          borderRadius="19px"
          style={{ background: 'linear-gradient(90deg, #b8960080 25%, #ffcc0040 50%, #b8960080 75%)', backgroundSize: '200% 100%', animation: 'shimmer 1.5s ease-in-out infinite' }}
        />

        {/* Price */}
        <MtnShimmer width="40%" height="0.875rem" style={{ margin: '12px auto 0' }} />
      </div>

      <span className="sr-only">Loading campaign details...</span>
    </div>
  )
}

export default Skeleton
