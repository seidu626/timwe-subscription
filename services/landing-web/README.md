# Landing Web

Next.js Landing Factory for serving campaign landing pages with modern UX and comprehensive feature set.

## Features

### Core Features
- **Campaign-driven landing pages** - Dynamic pages based on campaign configuration
- **Flexible subscription flows** - Supports OTP, SMS redirect, and direct subscription
- **Click ID tracking** - Full Mobplus attribution support (txid, clickid, cid, subid)
- **Passthrough parameters** - Supports campaign_id, offer_id, sub1-sub5, and more

### User Experience
- **Modern, responsive design** - Beautiful UI that works on all devices
- **Skeleton loading states** - Smooth loading experience with visual feedback
- **Form validation** - Client-side phone number validation with helpful error messages
- **Accessibility** - ARIA labels, keyboard navigation, screen reader support
- **Error boundaries** - Graceful error handling with user-friendly messages

### Performance
- **SEO optimized** - Meta tags, Open Graph tags, structured data
- **Lazy loading** - Dynamic imports for optional components
- **CDN-friendly** - Standalone output with caching headers
- **Security headers** - X-Content-Type-Options, X-Frame-Options, XSS protection

### Development Tools
- **Analytics tracking** - Event tracking for page views, form submissions, conversions
- **Debug panel** - Development-only analytics viewer
- **TypeScript** - Full type safety throughout the codebase

## Project Structure

```
app/
├── components/
│   ├── AnalyticsDebugPanel.tsx  # Dev-only analytics viewer
│   ├── ErrorBoundary.tsx        # Error handling wrapper
│   └── Skeleton.tsx             # Loading skeleton components
├── lp/
│   └── [slug]/
│       └── page.tsx             # Dynamic landing page
├── types/
│   └── index.ts                 # TypeScript type definitions
├── globals.css                  # Global styles
├── layout.tsx                   # Root layout with metadata
├── not-found.tsx               # 404 page
└── page.tsx                     # Home page
```

## Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Start production server
npm start

# Lint code
npm run lint
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NEXT_PUBLIC_ACQUISITION_API_URL` | Acquisition API base URL | `http://localhost:8084` |
| `NODE_ENV` | Environment (development/production) | `development` |

## URL Parameters

The landing page accepts the following URL parameters:

### Click ID (Canonical + Aliases)
- `click_id` - Canonical click ID
- `txid` - Mobplus transaction ID (alias)
- `clickid` - Alternative click ID format
- `cid` - Short click ID
- `subid` - Subscriber ID

### Attribution Parameters
- `campaign_id` - Campaign identifier
- `offer_id` - Offer identifier
- `adv_id` - Advertiser ID
- `aff_id` - Affiliate ID
- `sub1` through `sub5` - Custom sub-parameters
- `source` - Traffic source
- `creative` - Creative identifier
- `placement` - Placement identifier
- `provider` - Attribution provider (auto-detected as 'mobplus' if txid present)

## Example URLs

```
# Basic landing page
/lp/my-campaign

# With Mobplus attribution
/lp/my-campaign?txid=abc123&sub1=offer1&sub2=creative1

# With explicit click ID
/lp/my-campaign?click_id=xyz789&campaign_id=camp123&source=google
```

## API Integration

The landing page integrates with the Acquisition API:

- `GET /v1/campaigns/{slug}` - Fetch campaign configuration
- `POST /v1/acquisition/transactions` - Create subscription transaction
- `POST /v1/acquisition/transactions/{id}/confirm` - Confirm OTP

## Deployment

The app is configured for standalone deployment:

```bash
# Build the standalone output
npm run build

# The output is in .next/standalone
# Copy static files
cp -r .next/static .next/standalone/.next/static

# Run the server
cd .next/standalone && node server.js
```

### Docker

```bash
# Build the Docker image
docker build -t landing-web .

# Run the container
docker run -p 3000:3000 landing-web
```

## Security

The application includes security headers:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`

## License

Private - Timwe
