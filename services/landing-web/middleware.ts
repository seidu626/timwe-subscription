import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'
import { resolveHEIdentity, isValidMsisdn } from '@/lib/he-simulation'

// Custom headers used to pass HE identity to API routes (canonical source in lib/he-types.ts)
export const HE_HEADER_SOURCE = 'x-he-source'
export const HE_HEADER_MSISDN = 'x-he-msisdn'
export const HE_HEADER_OPERATOR = 'x-he-operator'
export const HE_HEADER_MCC = 'x-he-mcc'
export const HE_HEADER_MNC = 'x-he-mnc'

export async function middleware(request: NextRequest) {
  // Check if this is a server action request
  const isServerAction = request.headers.get('Next-Action') !== null

  if (isServerAction) {
    // For server action requests, we'll let them through but the error.tsx
    // will handle any "Failed to find Server Action" errors gracefully
    // by triggering a page refresh

    // Add cache-busting headers to the response
    const response = NextResponse.next()
    response.headers.set('Cache-Control', 'no-store, no-cache, must-revalidate')
    response.headers.set('Pragma', 'no-cache')
    response.headers.set('Expires', '0')
    return response
  }

  // For HTML page requests, ensure they're not cached
  const acceptHeader = request.headers.get('Accept') || ''
  if (acceptHeader.includes('text/html')) {
    const response = NextResponse.next()
    response.headers.set('Cache-Control', 'no-store, no-cache, must-revalidate')
    response.headers.set('Pragma', 'no-cache')
    response.headers.set('Expires', '0')
    return response
  }

  // For API routes, resolve HE identity and pass via headers
  if (request.nextUrl.pathname.startsWith('/api/')) {
    let identity = null

    try {
      identity = await resolveHEIdentity(request.headers, request.cookies)
    } catch {
      // HE resolution failed (e.g., JWT verification error)
      // Continue without HE identity; don't block the request
    }

    if (identity && identity.msisdn && isValidMsisdn(identity.msisdn)) {
      // Clone the request headers and add validated HE identity
      const requestHeaders = new Headers(request.headers)
      requestHeaders.set(HE_HEADER_SOURCE, identity.source)
      requestHeaders.set(HE_HEADER_MSISDN, identity.msisdn)
      if (identity.operatorId) {
        requestHeaders.set(HE_HEADER_OPERATOR, identity.operatorId)
      }
      if (identity.mcc) {
        requestHeaders.set(HE_HEADER_MCC, identity.mcc)
      }
      if (identity.mnc) {
        requestHeaders.set(HE_HEADER_MNC, identity.mnc)
      }

      return NextResponse.next({
        request: {
          headers: requestHeaders,
        },
      })
    }
  }

  return NextResponse.next()
}

export const config = {
  matcher: [
    // Match all paths except static files
    '/((?!_next/static|_next/image|favicon.ico|favicon.svg|site.webmanifest|apple-touch-icon.png|og-image.png|logo.png).*)',
  ],
}
