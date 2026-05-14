# BusinessSAAS

A modular SaaS platform with a secure, production-minded authentication and authorization system. Built on Go/Fiber for the backend and Next.js for the frontend test interface.

---

## What this is

BusinessSAAS is a greenfield SaaS foundation being built in phases. The goal is a multi-tenant platform that can grow into CRM, ERP, HRM, project management, and other business modules — all sharing a single, solid auth and permission layer.

**Phase 1 (current):** Auth foundation, RBAC, multi-tenant workspaces, and a permission-gated task module for testing. A test frontend validates everything in a real browser before the production Fuse React dashboard is integrated.

---

## Tech stack

### Backend

|                       |                                                    |
| --------------------- | -------------------------------------------------- |
| Language              | Go 1.23                                            |
| Framework             | Fiber v3                                           |
| Database              | PostgreSQL 16                                      |
| Cache / Rate limiting | Redis 7                                            |
| Migrations            | Goose                                              |
| Auth                  | JWT access tokens + opaque httpOnly refresh tokens |
| Authorization         | RBAC — roles, permissions, memberships             |

### Frontend (test interface)

|             |                                                               |
| ----------- | ------------------------------------------------------------- |
| Framework   | Next.js 15.1 (App Router)                                     |
| Language    | TypeScript 5.6                                                |
| Styling     | Tailwind CSS v4                                               |
| HTTP client | Axios                                                         |
| Mock layer  | axios-mock-adapter (auto-detects backend, falls back to mock) |

### Infrastructure

|            |                                |
| ---------- | ------------------------------ |
| Local dev  | Docker Compose                 |
| CI         | GitHub Actions                 |
| Deployment | VPS + Docker Compose (planned) |

---

## Project structure

```
BusinessSAAS/
├── docker-compose.yml
├── .env.example
├── .gitignore
│
├── .github/
│   └── workflows/
│       ├── ci.yml              # Runs on every push — build, test, lint
│       └── deploy.yml          # Manual VPS deployment (placeholder)
│
├── backend/
│   ├── Dockerfile
│   ├── .air.toml               # Live reload config for development
│   ├── Makefile
│   ├── go.mod
│   │
│   ├── cmd/server/
│   │   └── main.go             # Entry point — wires everything together
│   │
│   ├── internal/
│   │   ├── config/             # Environment loading
│   │   ├── database/           # PostgreSQL pool + Redis client
│   │   ├── middleware/         # Auth, permission, rate limit, logger, recover
│   │   ├── auth/               # Signup, login, refresh, logout
│   │   ├── user/               # User profile
│   │   ├── business/           # Workspace management
│   │   ├── authz/              # RBAC — roles, permissions, memberships
│   │   ├── task/               # Test CRUD module (permission validation)
│   │   └── audit/              # Append-only security event log
│   │
│   ├── pkg/
│   │   ├── jwt/                # Token issue + parse
│   │   ├── password/           # bcrypt hash + verify
│   │   ├── token/              # Opaque token generation + SHA-256 hash
│   │   └── response/           # Standard JSON response envelope
│   │
│   ├── migrations/             # Goose SQL migrations
│   └── tests/                  # Unit + integration tests
│
└── frontend/
    ├── Dockerfile
    │
    ├── app/                    # Next.js App Router pages
    │   ├── page.tsx            # Root — Hello World + connection status
    │   ├── (auth)/             # Login + signup (no layout wrapper)
    │   └── dashboard/          # Protected pages — overview, tasks, members, profile
    │
    ├── components/
    │   ├── ui/                 # Button, Input, Badge, Card, StatusDot
    │   ├── auth/               # LoginForm, SignupForm
    │   ├── layout/             # DashboardLayout (sidebar + nav)
    │   ├── tasks/              # TaskList (full CRUD with permission gating)
    │   ├── members/            # MemberList (role assignment + API preview)
    │   └── dev/                # MockToolbar + BackendProbe (dev only)
    │
    ├── hooks/                  # useAuth, usePermission, useBusiness
    ├── lib/                    # api.ts, auth.ts, permissions.ts, server-api.ts
    ├── lib/mock/               # Mock data, handlers, store (auto-detection)
    └── types/                  # TypeScript types for all domain entities
```

---

## Getting started

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (or Docker + Docker Compose)
- [Go 1.23+](https://go.dev/dl/) (for local backend development without Docker)
- [Node.js 20+](https://nodejs.org/) (for local frontend development without Docker)

### 1. Clone and configure

```bash
git clone https://github.com/yourusername/BusinessSAAS.git
cd BusinessSAAS

# Copy the environment template
cp .env.example .env
```

Open `.env` and set your own values for `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, and `JWT_SECRET`. The defaults work for local development but should never be used in production.

### 2. Start everything with Docker

```bash
docker compose up --build
```

This starts four services in the correct order:

| Service    | URL                   | Purpose                |
| ---------- | --------------------- | ---------------------- |
| Frontend   | http://localhost:3000 | Next.js test interface |
| Backend    | http://localhost:8080 | Go/Fiber API           |
| PostgreSQL | localhost:5432        | Primary database       |
| Redis      | localhost:6379        | Cache + rate limiting  |

### 3. Verify the connection

Open http://localhost:3000. The page fetches `GET /api/v1/hello` from the backend via the Docker network and displays the connection status. A green "Backend connected" indicator means everything is working.

Open http://localhost:8080/api/v1/health directly to verify the backend and its dependencies.

---

## Development workflow

### Docker (recommended)

```bash
# Start all services with live reload
docker compose up --build

# Tail logs for a specific service
docker compose logs -f backend
docker compose logs -f frontend

# Restart one service without stopping others
docker compose restart backend

# Stop everything (keeps database data)
docker compose down

# Stop and wipe all data
docker compose down -v
```

### Backend (local, without Docker)

```bash
cd backend

# Install dependencies
go mod tidy

# Run with live reload (requires Air)
make dev

# Or run directly
go run ./cmd/server

# Run tests
make test

# Run database migrations
make migrate-up

# Open a psql shell (requires Docker Compose running)
make db-shell
```

### Frontend (local, without Docker)

```bash
cd frontend

# Install dependencies
npm install

# Start dev server
npm run dev

# Type check
npm run type-check

# Lint
npm run lint
```

---

## Testing the frontend without a backend

The frontend includes an automatic mock layer. When the backend is unreachable, it intercepts all API calls and returns realistic mock data — no manual configuration needed.

**How it works:**

On every page load, the frontend probes `GET /api/v1/health`. If the backend does not respond within 3 seconds, the mock adapter activates automatically. When the backend starts, the next page load switches back to real API calls.

**Dev toolbar:**

A floating toolbar appears in the bottom-right corner in development mode. It shows:

- Whether mock or real mode is active
- A user switcher to test different roles
- The active user's permission set

**Mock users:**

| User  | Email                  | Role   | Permissions                  |
| ----- | ---------------------- | ------ | ---------------------------- |
| Alice | alice@businesssaas.dev | Owner  | Everything                   |
| Bob   | bob@businesssaas.dev   | Admin  | Tasks + manage members       |
| Carol | carol@businesssaas.dev | Member | Tasks read / create / update |
| Dave  | dave@businesssaas.dev  | Viewer | Tasks read only              |

Use any password when logging in during mock mode.

**Testing permission boundaries:**

1. Log in as Dave — the Tasks page shows no Create or Delete buttons
2. Switch to Alice via the toolbar — all buttons appear
3. Go to Members — change Carol from `member` to `viewer`
4. The change persists across page refreshes (saved to localStorage)
5. Click "Reset member roles" in the toolbar to restore defaults

---

## API overview

All responses follow a consistent envelope:

```json
// Success
{
  "success": true,
  "data": {},
  "message": "OK",
  "request_id": "uuid"
}

// Error
{
  "success": false,
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "Invalid email or password"
  },
  "request_id": "uuid"
}
```

### Public endpoints

```
GET  /api/v1/health
GET  /api/v1/hello
POST /api/v1/auth/signup
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/password-reset/request
POST /api/v1/auth/password-reset/confirm
```

### Authenticated endpoints (JWT required)

```
POST  /api/v1/auth/logout
POST  /api/v1/auth/logout-all
GET   /api/v1/users/me
PATCH /api/v1/users/me
```

### Business-scoped endpoints (JWT + business context required)

```
POST /api/v1/businesses
GET  /api/v1/businesses
GET  /api/v1/businesses/:id
POST /api/v1/businesses/:id/switch

GET  /api/v1/members
POST /api/v1/members/:userId/role

GET  /api/v1/roles
GET  /api/v1/permissions

GET    /api/v1/tasks          (requires tasks.read)
POST   /api/v1/tasks          (requires tasks.create)
GET    /api/v1/tasks/:id      (requires tasks.read)
PATCH  /api/v1/tasks/:id      (requires tasks.update)
DELETE /api/v1/tasks/:id      (requires tasks.delete)
```

---

## Permission model

Users belong to businesses through memberships. Each membership has a role. Roles have permissions.

```
User ──── Membership ────▶ Role ──── RolePermission ────▶ Permission
            (per business)
```

**System roles:**

| Role   | tasks.read | tasks.create | tasks.update | tasks.delete | members.manage | business.manage |
| ------ | :--------: | :----------: | :----------: | :----------: | :------------: | :-------------: |
| Owner  |     ✓      |      ✓       |      ✓       |      ✓       |       ✓        |        ✓        |
| Admin  |     ✓      |      ✓       |      ✓       |      ✓       |       ✓        |        ✗        |
| Member |     ✓      |      ✓       |      ✓       |      ✗       |       ✗        |        ✗        |
| Viewer |     ✓      |      ✗       |      ✗       |      ✗       |       ✗        |        ✗        |

The backend enforces permissions on every request. The frontend hides or disables UI elements based on the same rules, but this is for user experience only — the backend is always the authority.

---

## Token architecture

```
Login response:
  Body:    { access_token: "..." }      ← stored in memory (lost on refresh)
  Cookie:  bsaas_refresh=<token>        ← httpOnly, never readable by JS

On page load:
  POST /auth/refresh                    ← browser sends cookie automatically
  Body:    { access_token: "..." }      ← new access token stored in memory

Logout:
  POST /auth/logout                     ← backend clears cookie via Set-Cookie
  Memory:  access token cleared
```

The access token is never written to localStorage or any persistent storage. The refresh token is never readable by JavaScript.

---

## Database migrations

Migrations are managed with [Goose](https://github.com/pressly/goose) and live in `backend/migrations/`.

```bash
# Run all pending migrations
make migrate-up

# Roll back the last migration
make migrate-down

# Check migration status
make migrate-status

# Create a new migration
make migrate-create NAME=add_email_verification
```

Migration files follow the naming convention `NNNNN_description.sql` where `NNNNN` is a zero-padded sequence number.

---

## CI / CD

### CI (runs on every push and pull request)

GitHub Actions validates the project on every push to `main` or `develop`:

- **Backend:** `go vet`, `go test`, `go build`
- **Frontend:** ESLint, TypeScript type check, Next.js build
- **Docker:** builds both images to confirm they compile

See `.github/workflows/ci.yml`.

### Deployment (manual, VPS)

The deploy workflow at `.github/workflows/deploy.yml` is a placeholder with the full SSH + Docker Compose deployment steps defined and commented out. Connect it by:

1. Adding your VPS SSH key and credentials to GitHub Secrets
2. Uncommenting the deploy steps in `deploy.yml`
3. Triggering manually via GitHub Actions → Deploy → Run workflow

---

## Environment variables

### Root `.env`

| Variable                | Default                 | Description                                 |
| ----------------------- | ----------------------- | ------------------------------------------- |
| `POSTGRES_USER`         | `saas_user`             | PostgreSQL username                         |
| `POSTGRES_PASSWORD`     | —                       | PostgreSQL password (required)              |
| `POSTGRES_DB`           | `businesssaas`          | Database name                               |
| `REDIS_PASSWORD`        | —                       | Redis password (required)                   |
| `JWT_SECRET`            | —                       | JWT signing secret, min 32 chars (required) |
| `JWT_ACCESS_TOKEN_TTL`  | `15m`                   | Access token expiry                         |
| `JWT_REFRESH_TOKEN_TTL` | `7d`                    | Refresh token expiry                        |
| `CORS_ALLOWED_ORIGINS`  | `http://localhost:3000` | Comma-separated allowed origins             |
| `NEXT_PUBLIC_API_URL`   | `http://localhost:8080` | Backend URL for browser-side requests       |

### Backend `.env`

See `backend/.env.example` for the full list including database connection pool settings.

### Frontend `.env`

| Variable               | Description                                                                         |
| ---------------------- | ----------------------------------------------------------------------------------- |
| `BACKEND_INTERNAL_URL` | Backend URL for server-side Next.js requests (inside Docker: `http://backend:8080`) |
| `NEXT_PUBLIC_API_URL`  | Backend URL for browser-side requests                                               |

---

## Makefile reference

Run these from the `backend/` directory:

```bash
make help           # List all available commands
make dev            # Run with Air live reload
make build          # Compile production binary
make test           # Run all tests
make test-unit      # Run unit tests only
make lint           # Run golangci-lint
make migrate-up     # Apply pending migrations
make migrate-down   # Roll back last migration
make migrate-status # Show migration status
make db-shell       # Open psql (Docker must be running)
make redis-shell    # Open redis-cli (Docker must be running)
make clean          # Remove build artifacts
```

---

## Roadmap

### Phase 1 — Auth foundation (current)

- [x] Project structure and Docker Compose
- [x] Backend skeleton — all modules, handlers, services, repositories
- [x] Database connection — PostgreSQL pool + Redis client
- [x] Middleware — JWT auth, permission enforcement, rate limiting, logging
- [x] pkg layer — JWT, bcrypt password, opaque token, response helpers
- [x] Frontend test interface — all pages, components, hooks
- [x] Mock layer — auto-detects backend, falls back to realistic mock data
- [x] CI pipeline — GitHub Actions
- [ ] Auth implementation — signup, login, refresh, logout (Phase 1-B)
- [ ] Business/workspace CRUD (Phase 1-C)
- [ ] RBAC implementation — roles, permissions, membership (Phase 1-D)
- [ ] Task CRUD with permission enforcement (Phase 1-E)
- [ ] Rate limiting — Redis sliding window
- [ ] Audit logging
- [ ] Integration tests

### Phase 2 — Hardening

- [ ] Password reset flow
- [ ] Email verification
- [ ] Account lockout after failed attempts
- [ ] Full integration test suite
- [ ] VPS deployment pipeline

### Phase 3+ — Business modules

- [ ] Fuse React production dashboard
- [ ] CRM module
- [ ] HRM module
- [ ] Project management module
- [ ] Billing integration

---

## Contributing

This is a private project in active development. The architecture is intentionally strict — please follow the existing patterns for adding new modules:

1. Every module gets its own folder under `internal/`
2. Handler layer: HTTP request/response only
3. Service layer: business logic only
4. Repository layer: SQL queries only
5. No circular dependencies
6. No SQL string concatenation — parameterized queries only
7. All schema changes go through Goose migrations

---

## License

Private — all rights reserved.
