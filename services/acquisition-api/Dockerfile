# Build stage
FROM docker.io/library/golang:1.24-alpine AS builder

WORKDIR /build

# Copy everything including vendor
COPY . .

# Build the application using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o acquisition-api ./cmd/main.go

# Final stage
FROM docker.io/library/alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/acquisition-api .

# Copy config
COPY config.yaml .
COPY migrations ./migrations

EXPOSE 8084

CMD ["./acquisition-api"]
