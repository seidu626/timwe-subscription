# Stage 1: Build
FROM docker.io/library/golang:1.24-alpine AS builder

WORKDIR /app

# Resolve the service module and its local common-module replacement from the
# repository-root build context. Vendor directories are intentionally untracked.
COPY common/go.mod common/go.sum /common/
COPY services/subscription-external/go.mod services/subscription-external/go.sum ./
RUN go mod download

COPY common /common
COPY services/subscription-external/ .

# Build without relying on machine-local vendor trees.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -a -installsuffix cgo -o subscription-external ./cmd

# Stage 2: Run
FROM docker.io/library/alpine:latest

# Install CA certificates for HTTPS calls, timezone data, and wget for health checks
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/subscription-external .

# Copy config file (secrets should come from environment variables)
COPY services/subscription-external/config.yaml .

EXPOSE 8083

CMD ["./subscription-external"]
