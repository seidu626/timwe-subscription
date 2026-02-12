'use client'

import React from 'react'

interface ErrorBoundaryState {
  hasError: boolean
  error?: Error
}

interface ErrorBoundaryProps {
  children: React.ReactNode
  fallback?: React.ComponentType<{ error?: Error; resetError: () => void }>
}

class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('Error caught by boundary:', error, errorInfo)
    // Here you could send error to logging service
  }

  resetError = () => {
    this.setState({ hasError: false, error: undefined })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        const FallbackComponent = this.props.fallback
        return <FallbackComponent error={this.state.error} resetError={this.resetError} />
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
            <h1 style={{ color: '#dc2626', marginBottom: '1rem' }}>Oops! Something went wrong</h1>
            <p style={{ color: '#6b7280', marginBottom: '2rem' }}>
              We're sorry, but something unexpected happened. Please try refreshing the page.
            </p>
            {process.env.NODE_ENV === 'development' && this.state.error && (
              <details style={{
                background: '#f3f4f6',
                padding: '1rem',
                borderRadius: '8px',
                marginBottom: '1rem',
                textAlign: 'left'
              }}>
                <summary style={{ cursor: 'pointer', fontWeight: 'bold' }}>
                  Error Details (Development)
                </summary>
                <pre style={{
                  whiteSpace: 'pre-wrap',
                  fontSize: '0.9rem',
                  color: '#dc2626',
                  marginTop: '0.5rem'
                }}>
                  {this.state.error.message}
                </pre>
              </details>
            )}
            <div style={{ display: 'flex', gap: '1rem', justifyContent: 'center' }}>
              <button
                onClick={this.resetError}
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

    return this.props.children
  }
}

export default ErrorBoundary