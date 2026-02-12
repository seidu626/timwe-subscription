# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
ENV GOPROXY=https://goproxy.io,direct
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o notification-service ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o notification-worker ./cmd/notification-worker

# Stage 2: Run
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/notification-service .
COPY --from=builder /app/notification-worker .
COPY --from=builder /app/config.yaml .
EXPOSE 8082
CMD ["./notification-service"]