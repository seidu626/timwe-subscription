# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy everything including vendor
COPY . .

# Build the application using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o subscription-external ./cmd

# Stage 2: Run
FROM alpine:latest

# Install CA certificates for HTTPS calls, timezone data, and wget for health checks
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/subscription-external .

# Copy config file (secrets should come from environment variables)
COPY config.yaml .

EXPOSE 8083

CMD ["./subscription-external"]
