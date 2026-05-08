import Link from 'next/link'

export default function NotFound() {
  return (
    <main className="container" role="main" style={{ textAlign: 'center', marginTop: '4rem' }}>
      <h1 style={{ marginBottom: '1rem', color: '#dc2626' }}>404 - Page Not Found</h1>
      <p style={{ color: '#6b7280', marginBottom: '2rem' }}>
        Sorry, the page you're looking for doesn't exist or has been moved.
      </p>
      <Link
        href="/"
        style={{
          display: 'inline-block',
          padding: '0.75rem 1.5rem',
          background: '#3b82f6',
          color: 'white',
          borderRadius: '8px',
          textDecoration: 'none',
          fontWeight: '500'
        }}
      >
        Go Home
      </Link>
    </main>
  )
}