# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package.json package-lock.json* ./

# Install dependencies (npm ci is faster and more reliable for CI/CD)
RUN npm install

# Copy source code
COPY . .

# Clean any existing build artifacts to prevent stale server action IDs
RUN rm -rf .next

# Set build ID for cache invalidation (can be overridden via build arg)
ARG BUILD_ID=default
ENV BUILD_ID=${BUILD_ID}

# Build Next.js app with fresh build (generate unique build ID if not provided)
RUN if [ "$BUILD_ID" = "default" ]; then \
      BUILD_ID="build-$(date +%s)" npm run build; \
    else \
      npm run build; \
    fi

# Production stage
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production

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
