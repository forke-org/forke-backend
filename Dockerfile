# Multi-stage Docker build for Go Backend
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build tools and SSL certs
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile static Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/server ./cmd/api

# Final production stage
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bin/server /app/server

EXPOSE 8080

USER 1000:1000

CMD ["/app/server"]
