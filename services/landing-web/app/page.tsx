import Link from 'next/link'

export default function HomePage() {
  return (
    <main className="container" role="main" style={{ textAlign: 'center', marginTop: '4rem' }}>
      <h1 style={{ marginBottom: '1rem' }}>Nouveauriche Subscription Platform</h1>
      <p style={{ color: '#6b7280', marginBottom: '2rem' }}>
        Welcome to the Nouveauriche subscription landing page service.
      </p>
      <p style={{ fontSize: '0.9rem', color: '#9ca3af' }}>
        Please use a campaign link to access subscription pages.
      </p>
      <p style={{ fontSize: '0.8rem', color: '#9ca3af', marginTop: '2rem' }}>
        Example: <code>/lp/your-campaign-slug</code>
      </p>
    </main>
  )
}