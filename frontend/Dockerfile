# ============================================================
# BusinessSAAS Frontend — Dockerfile
# Next.js 15 + Bun package manager
# ============================================================
# Stages:
#   deps        — Install dependencies with Bun
#   development — Dev server with hot reload
#   builder     — Production build
#   production  — Minimal Node.js runtime (standalone output)
# ============================================================

# ----------------------------------------------------------
# Stage 1: deps
# ----------------------------------------------------------
FROM oven/bun:1-alpine AS deps

RUN apk add --no-cache libc6-compat

WORKDIR /app

COPY package.json bun.lock* ./
RUN bun install --frozen-lockfile

# ----------------------------------------------------------
# Stage 2: development
# ----------------------------------------------------------
FROM oven/bun:1-alpine AS development

RUN apk add --no-cache libc6-compat

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

ENV HOSTNAME="0.0.0.0"
ENV PORT=3000

EXPOSE 3000

CMD ["bun", "run", "dev"]

# ----------------------------------------------------------
# Stage 3: builder
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
# Stage 4: production
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

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://localhost:3000/ || exit 1

CMD ["node", "server.js"]
