import { redirect, notFound } from 'next/navigation'

/**
 * /c/:slug -> /lp/:slug redirect
 * 
 * This is an alias route for backward compatibility.
 * HE bootstrap on HTTP uses /c/:slug for capture, but the canonical landing
 * path is /lp/:slug. This redirect preserves query params (including he_token).
 */
export default function CampaignRedirect({
  params,
  searchParams,
}: {
  params: { slug: string }
  searchParams: { [key: string]: string | string[] | undefined }
}) {
  const slug = params?.slug

  // Validate slug exists and is non-empty
  if (!slug || typeof slug !== 'string' || slug.trim() === '') {
    notFound()
  }

  // Build query string from searchParams (URLSearchParams handles encoding)
  const queryString = new URLSearchParams()
  for (const [key, value] of Object.entries(searchParams)) {
    if (Array.isArray(value)) {
      value.forEach(v => queryString.append(key, v))
    } else if (value !== undefined) {
      queryString.set(key, value)
    }
  }

  // URL-encode the slug to handle special characters safely
  const encodedSlug = encodeURIComponent(slug.trim())
  const query = queryString.toString()
  const targetUrl = `/lp/${encodedSlug}${query ? `?${query}` : ''}`

  redirect(targetUrl)
}
