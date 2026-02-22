# Dependency stage (cached unless lockfile changes)
FROM node:20-alpine AS deps

WORKDIR /app

# Copy package files first for better layer caching
COPY package.json package-lock.json* ./

# npm ci is deterministic and faster for CI/build containers
RUN npm ci --no-audit --no-fund

# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

ENV NEXT_TELEMETRY_DISABLED=1

# Reuse dependency layer
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/package.json ./package.json
COPY --from=deps /app/package-lock.json ./package-lock.json

# Copy source code
COPY . .

# Set deterministic default build ID (override in CI for release cache-busting)
ARG BUILD_ID=build-local
ENV BUILD_ID=${BUILD_ID}

# Build Next.js app
RUN npm run build

# Production stage
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Don't run as root
RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# Copy the standalone build and static assets
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
