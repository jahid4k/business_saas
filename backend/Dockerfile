# ============================================================
# BusinessSAAS Backend — Dockerfile
# Go/Fiber API Server
# ============================================================
#
# Stages:
#   base        — Go toolchain + dependencies
#   development — Live reload with Air
#   builder     — Compile production binary
#   production  — Minimal runtime image
#
# Usage:
#   docker compose up --build                     (uses development target)
#   docker build --target production -t backend . (production image)
# ============================================================

# ----------------------------------------------------------
# Stage 1: base — Go toolchain + module cache
# ----------------------------------------------------------
FROM golang:1.25-alpine AS base

# Install system dependencies
# - git: needed by some Go tools
# - ca-certificates: needed for HTTPS calls
# - wget: used in Docker healthcheck
RUN apk add --no-cache \
    git \
    ca-certificates \
    wget \
    tzdata \
    build-base \
    libwebp-dev

WORKDIR /app

# Copy dependency manifests first for Docker layer cache
COPY go.mod go.sum ./

# Download modules (cached unless go.mod/go.sum change)
RUN go mod download && go mod verify

# ----------------------------------------------------------
# Stage 2: development — Live reload with Air
# ----------------------------------------------------------
FROM base AS development

# Install Air for live reload
# https://github.com/air-verse/air
RUN GOTOOLCHAIN=auto go install github.com/air-verse/air@latest

# Install Goose for running migrations from inside the container
# https://github.com/pressly/goose
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Copy full source (volume mount will override in compose)
COPY . .

# Expose API port
EXPOSE 8080

# Air reads .air.toml for config — we define it below
# Fallback: if no .air.toml, Air uses defaults watching .go files
CMD ["air", "-c", ".air.toml"]

# ----------------------------------------------------------
# Stage 3: builder — Compile final binary
# ----------------------------------------------------------
FROM base AS builder

# Copy full source
COPY . .

# Build the binary
# CGO_ENABLED=1 — required by github.com/kolesa-team/go-webp (CGo bindings
# to libwebp). This does mean the binary is no longer a static, dependency-
# free executable — production must have libwebp's runtime .so available
# (see the production stage below).
# -ldflags="-s -w" — strip debug info, reduce binary size
# -trimpath — remove local file paths from binary
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /bin/businesssaas \
    ./cmd/server

# ----------------------------------------------------------
# Stage 4: production — Minimal runtime image
# ----------------------------------------------------------
FROM alpine:3.20 AS production

# Security: run as non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Install runtime dependencies
# libwebp: runtime .so for the CGo-linked avatar encoder (see builder stage)
RUN apk add --no-cache \
    ca-certificates \
    wget \
    tzdata \
    libwebp

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /bin/businesssaas ./businesssaas

# Copy migrations directory (Goose needs these at runtime)
COPY --from=builder /app/migrations ./migrations

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/v1/health || exit 1

# Run the binary
CMD ["./businesssaas"]