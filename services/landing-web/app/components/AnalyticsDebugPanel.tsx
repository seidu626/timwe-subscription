'use client'

import { useEffect, useState } from 'react'

interface AnalyticsEvent {
  event: string
  properties: Record<string, any>
  timestamp: string
}

export default function AnalyticsDebugPanel() {
  const [events, setEvents] = useState<AnalyticsEvent[]>([])
  const [isOpen, setIsOpen] = useState(false)

  useEffect(() => {
    const storedEvents = JSON.parse(localStorage.getItem('analytics_events') || '[]')
    setEvents(storedEvents)
  }, [])

  if (!isOpen) {
    return (
      <div style={{
        position: 'fixed',
        bottom: '20px',
        right: '20px',
        background: '#374151',
        color: 'white',
        padding: '0.5rem 1rem',
        borderRadius: '8px',
        cursor: 'pointer',
        fontSize: '0.9rem',
        zIndex: 1000
      }} onClick={() => setIsOpen(true)}>
        Analytics Debug ({events.length})
      </div>
    )
  }

  return (
    <div style={{
      position: 'fixed',
      bottom: '20px',
      right: '20px',
      background: 'white',
      border: '2px solid #374151',
      borderRadius: '8px',
      padding: '1rem',
      maxWidth: '400px',
      maxHeight: '300px',
      overflow: 'auto',
      zIndex: 1000,
      fontSize: '0.8rem'
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <strong>Analytics Events ({events.length})</strong>
        <button onClick={() => setIsOpen(false)} style={{ border: 'none', background: 'none', cursor: 'pointer' }}>×</button>
      </div>
      <div style={{ marginBottom: '1rem' }}>
        <button
          onClick={() => {
            localStorage.removeItem('analytics_events')
            setEvents([])
          }}
          style={{ fontSize: '0.8rem', padding: '0.25rem 0.5rem' }}
        >
          Clear Events
        </button>
      </div>
      {events.slice(-5).reverse().map((event, index) => (
        <div key={index} style={{ marginBottom: '0.5rem', padding: '0.5rem', background: '#f3f4f6', borderRadius: '4px' }}>
          <strong>{event.event}</strong>
          <div style={{ marginTop: '0.25rem', fontSize: '0.7rem', color: '#6b7280' }}>
            {new Date(event.timestamp).toLocaleTimeString()}
          </div>
          {Object.keys(event.properties).length > 0 && (
            <pre style={{ marginTop: '0.25rem', fontSize: '0.7rem', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {JSON.stringify(event.properties, null, 2)}
            </pre>
          )}
        </div>
      ))}
    </div>
  )
}