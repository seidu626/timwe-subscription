# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
ENV GOPROXY=https://goproxy.io,direct
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o billing-service ./cmd

# Stage 2: Run
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/billing-service .
EXPOSE 8083
CMD ["./billing-service"]