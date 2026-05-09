# Build stage
FROM docker.io/library/golang:1.24-alpine AS builder

WORKDIR /build

COPY common/go.mod common/go.sum /common/
COPY services/acquisition-api/go.mod services/acquisition-api/go.sum ./
RUN go mod download

COPY common /common
COPY services/acquisition-api/ .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -a -installsuffix cgo -o acquisition-api ./cmd/main.go

# Final stage
FROM docker.io/library/alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/acquisition-api .

# Copy config
COPY services/acquisition-api/config.yaml .
COPY services/acquisition-api/migrations ./migrations

EXPOSE 8084

CMD ["./acquisition-api"]
