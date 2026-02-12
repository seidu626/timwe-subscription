# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy everything including vendor
COPY . .

# Build the application using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o postback-dispatcher ./cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/postback-dispatcher .

# Copy config
COPY config.yaml .

CMD ["./postback-dispatcher"]
