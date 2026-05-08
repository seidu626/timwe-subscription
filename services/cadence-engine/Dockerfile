# Stage 1: Build
FROM docker.io/library/golang:1.24-alpine AS builder
WORKDIR /app

# Copy everything including vendor
COPY . .

# Build the application using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o cadence-engine ./cmd/cadence-engine

# Stage 2: Run
FROM docker.io/library/alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/cadence-engine .
COPY --from=builder /app/config.yaml .
CMD ["./cadence-engine"]
