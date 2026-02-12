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

export function LandingPageSkeleton() {
  return (
    <div className="container" role="status" aria-label="Loading campaign">
      <div className="header" style={{ textAlign: 'center', marginBottom: '2rem' }}>
        <Skeleton width="60%" height="2.5rem" style={{ margin: '0 auto 1rem' }} />
        <Skeleton width="40%" height="1.5rem" style={{ margin: '0 auto' }} />
      </div>

      <div style={{ marginBottom: '1.5rem' }}>
        <SkeletonInput />
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '1.5rem' }}>
          <Skeleton width="1.25rem" height="1.25rem" borderRadius="4px" />
          <Skeleton width="80%" height="1rem" />
        </div>
        <SkeletonButton />
      </div>

      <div style={{ marginTop: '2rem', paddingTop: '2rem', borderTop: '1px solid #e5e7eb' }}>
        <Skeleton width="50%" height="1.5rem" style={{ marginBottom: '1rem' }} />
        <SkeletonText lines={4} />
      </div>

      <style>{`
        @keyframes shimmer {
          0% { background-position: 200% 0; }
          100% { background-position: -200% 0; }
        }
      `}</style>
      <span className="sr-only">Loading campaign details...</span>
    </div>
  )
}

export default Skeleton