/**
 * HE Simulation Clear Route - Staging/Local Only
 *
 * POST /__he/clear - Clear the simulation cookie
 *
 * SECURITY: Returns 404 when HE_SIMULATION_ENABLED !== 'true'
 */

import { NextResponse } from 'next/server'
import { getHESimConfig } from '@/lib/he-simulation'

/**
 * POST /__he/clear - Clear simulation cookie
 */
export async function POST() {
  const config = getHESimConfig()

  if (!config.enabled) {
    return new NextResponse('Not Found', { status: 404 })
  }

  const response = new NextResponse(null, { status: 204 })

  response.cookies.set(config.cookieName, '', {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    maxAge: 0,
    path: '/',
  })

  return response
}
