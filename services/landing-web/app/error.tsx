'use client'

import { useEffect } from 'react'

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    // Log the error for debugging
    console.error('App Error:', error)

    // Check if this is a server action mismatch error
    // These errors occur when client has stale JS from a previous deployment
    const isServerActionError = 
      error.message?.includes('Failed to find Server Action') ||
      error.message?.includes('older or newer deployment')

    if (isServerActionError) {
      console.log('Detected stale deployment - refreshing page...')
      // Clear any cached data and force reload
      if (typeof window !== 'undefined') {
        // Clear session storage to remove any cached state
        sessionStorage.clear()
        // Force a hard reload to get fresh assets
        window.location.reload()
      }
    }
  }, [error])

  // Check if this is a server action error
  const isServerActionError = 
    error.message?.includes('Failed to find Server Action') ||
    error.message?.includes('older or newer deployment')

  if (isServerActionError) {
    return (
      <div style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        padding: '2rem',
        fontFamily: 'system-ui, -apple-system, sans-serif',
        backgroundColor: '#f8f9fa'
      }}>
        <div style={{
          background: 'white',
          borderRadius: '12px',
          padding: '2rem',
          boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)',
          textAlign: 'center',
          maxWidth: '500px'
        }}>
          <div style={{ fontSize: '3rem', marginBottom: '1rem' }}>🔄</div>
          <h1 style={{ color: '#3b82f6', marginBottom: '1rem' }}>Updating...</h1>
          <p style={{ color: '#6b7280', marginBottom: '1.5rem' }}>
            A new version is available. The page will refresh automatically.
          </p>
          <button
            onClick={() => window.location.reload()}
            style={{
              padding: '0.75rem 1.5rem',
              background: '#3b82f6',
              color: 'white',
              border: 'none',
              borderRadius: '8px',
              cursor: 'pointer',
              fontWeight: '500'
            }}
          >
            Refresh Now
          </button>
        </div>
      </div>
    )
  }

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: '100vh',
      padding: '2rem',
      fontFamily: 'system-ui, -apple-system, sans-serif',
      backgroundColor: '#f8f9fa'
    }}>
      <div style={{
        background: 'white',
        borderRadius: '12px',
        padding: '2rem',
        boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)',
        textAlign: 'center',
        maxWidth: '500px'
      }}>
        <h1 style={{ color: '#dc2626', marginBottom: '1rem' }}>Something went wrong</h1>
        <p style={{ color: '#6b7280', marginBottom: '2rem' }}>
          We're sorry, but something unexpected happened. Please try again.
        </p>
        <div style={{ display: 'flex', gap: '1rem', justifyContent: 'center' }}>
          <button
            onClick={reset}
            style={{
              padding: '0.75rem 1.5rem',
              background: '#3b82f6',
              color: 'white',
              border: 'none',
              borderRadius: '8px',
              cursor: 'pointer',
              fontWeight: '500'
            }}
          >
            Try Again
          </button>
          <button
            onClick={() => window.location.reload()}
            style={{
              padding: '0.75rem 1.5rem',
              background: 'white',
              color: '#374151',
              border: '2px solid #d1d5db',
              borderRadius: '8px',
              cursor: 'pointer',
              fontWeight: '500'
            }}
          >
            Refresh Page
          </button>
        </div>
      </div>
    </div>
  )
}
