# ============================================================
# BusinessSAAS Frontend — Dockerfile
# Next.js Test Interface using Bun
# ============================================================
#
# Stages:
#   deps          — Install dependencies using Bun
#   development   — Next.js dev server with hot reload using Bun
#   builder       — Build production Next.js output using Bun
#   production    — Minimal Node runtime with standalone output
# ============================================================

# ----------------------------------------------------------
# Stage 1: deps — Install dependencies with Bun
# ----------------------------------------------------------
FROM oven/bun:1-alpine AS deps

RUN apk add --no-cache libc6-compat

WORKDIR /app

# Copy Bun package manifests
COPY package.json bun.lock* ./

# Install dependencies
RUN bun install --frozen-lockfile

# ----------------------------------------------------------
# Stage 2: development — Hot reload dev server
# ----------------------------------------------------------
FROM oven/bun:1-alpine AS development

RUN apk add --no-cache libc6-compat

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules

COPY . .

EXPOSE 3000

ENV HOSTNAME="0.0.0.0"
ENV PORT=3000

CMD ["bun", "run", "dev"]

# ----------------------------------------------------------
# Stage 3: builder — Build production output
# ----------------------------------------------------------
FROM oven/bun:1-alpine AS builder

RUN apk add --no-cache libc6-compat

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules

COPY . .

ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production

RUN bun run build

# ----------------------------------------------------------
# Stage 4: production — Minimal runtime image
# ----------------------------------------------------------
FROM node:20-alpine AS production

RUN apk add --no-cache libc6-compat

RUN addgroup -g 1001 -S nodejs && \
    adduser -u 1001 -S nextjs -G nodejs

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public

USER nextjs

EXPOSE 3000

HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:3000/ || exit 1

CMD ["node", "server.js"]