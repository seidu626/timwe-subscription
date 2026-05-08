/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Enable static export for CDN deployment
  output: 'standalone',
  // Generate a consistent build ID based on environment variable or timestamp
  // This helps with cache invalidation when deployments happen
  generateBuildId: async () => {
    // Use BUILD_ID env var if provided (for CI/CD), otherwise use timestamp
    return process.env.BUILD_ID || `build-${Date.now()}`
  },
  // Note: ACQUISITION_API_URL is read at runtime via process.env in API routes
  // Do NOT add it to the env block here - that inlines the value at build time
  // Performance optimizations
  poweredByHeader: false,
  compress: true,
  // Optimize images
  images: {
    formats: ['image/avif', 'image/webp'],
    minimumCacheTTL: 60,
  },
  // Experimental optimizations (disabled optimizeCss as it requires critters package)
  // experimental: {
  //   optimizeCss: true,
  // },
  // Headers for caching and security
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff',
          },
          {
            key: 'X-Frame-Options',
            value: 'DENY',
          },
          {
            key: 'X-XSS-Protection',
            value: '1; mode=block',
          },
          {
            key: 'Referrer-Policy',
            value: 'strict-origin-when-cross-origin',
          },
        ],
      },
      {
        // JavaScript and CSS files - use cache busting via build ID in filenames
        source: '/_next/static/:path*',
        headers: [
          {
            key: 'Cache-Control',
            value: 'public, max-age=31536000, immutable',
          },
        ],
      },
      {
        // HTML pages - short cache to ensure clients get fresh HTML with new JS references
        source: '/lp/:slug*',
        headers: [
          {
            key: 'Cache-Control',
            value: 'public, max-age=0, must-revalidate',
          },
        ],
      },
      {
        // Root page
        source: '/',
        headers: [
          {
            key: 'Cache-Control',
            value: 'public, max-age=0, must-revalidate',
          },
        ],
      },
    ]
  },
}

module.exports = nextConfig
