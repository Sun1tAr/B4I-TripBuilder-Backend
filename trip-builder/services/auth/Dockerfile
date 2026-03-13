# Multi-stage build for Tasks service

# Stage 1: Builder
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Copy vendor directory (created in CI)
COPY vendor ./vendor

# Copy source code
COPY . .
COPY cmd/server/migrations /app/cmd/server/migrations

# Build the application using vendor
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/auth ./cmd/server

# Stage 2: Final image
FROM alpine:latest

# Add ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -u 1000 appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=appuser:appuser /app/bin/auth /app/auth

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8081

# Command to run
CMD ["/app/auth"]