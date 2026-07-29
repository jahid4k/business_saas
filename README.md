# BusinessSAAS

A modular, multi-tenant B2B SaaS platform built as a unified business operating system. One auth
layer, one RBAC system, one engagement layer — every business module sits on the same foundation
rather than reimplementing it.

Three clients against one Go backend: a Next.js web dashboard, an Expo mobile app, and a public
capture API.

> **Documentation:** the [project wiki](../../wiki) holds guides and architecture. Code-coupled
> reference — modules, database, ADRs — lives in [`docs/`](docs/).

---

## Status

| Component             | State                                    |
| --------------------- | ---------------------------------------- |
| Backend               | 🔵 Active — 391 routes across 12 domains |
| Web frontend          | 🔵 Active                                |
| Mobile                | 🔵 Active — auth, dashboard, tasks, CRM  |
| Production deployment | ⚪ Not started                           |

**Shipped:** auth, organizations, RBAC, security, tasks, platform layer (contacts and engagement),
CRM, HRM.

**In progress:** lead capture, mobile.

Capture is written end-to-end but **not functional** — three of its five sources cannot create a
lead, and both webhook endpoints ship without signature verification. It must not be exposed to
the public internet in its current state. See [Known Issues](../../wiki/Known-Issues).

---

## Tech stack

**Backend** — Go, Fiber v3, PostgreSQL, Redis, Goose migrations
**Frontend** — Next.js App Router, TypeScript, Tailwind CSS v4, TanStack Query, Zustand
**Mobile** — Expo, React Native, Expo Router, expo-secure-store
**Infrastructure** — Docker Compose, GitHub Actions

Exact versions live in [`backend/go.mod`](backend/go.mod),
[`frontend/package.json`](frontend/package.json), and
[`mobile/package.json`](mobile/package.json). Those files are the source of truth — this README
deliberately does not restate version numbers, because copied numbers drift. (An earlier revision
of this file claimed Go 1.23 and Next.js 15.1 long after both had moved on.)

---

## Getting started

### Prerequisites

- Docker Desktop, or Docker with Compose
- Go — for backend work outside Docker; version in `backend/go.mod`
- Bun — for frontend work outside Docker

### Run it

```bash
git clone <repository-url>
cd BusinessSAAS

cp .env.example .env
```

Open `.env` and set `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, and `JWT_SECRET`. The template values
are fine locally and must never reach production — `JWT_SECRET` in particular should be at least
64 random characters.

```bash
docker compose up --build
```

Four services start in dependency order:

| Service    | URL                   | Purpose                                |
| ---------- | --------------------- | -------------------------------------- |
| Frontend   | http://localhost:3000 | Next.js dashboard                      |
| Backend    | http://localhost:8080 | Go / Fiber API                         |
| PostgreSQL | localhost:5432        | Primary database                       |
| Redis      | localhost:6379        | Cache, rate limiting, permission cache |

### Verify

```bash
curl http://localhost:8080/api/v1/health
```

Checks both PostgreSQL and Redis. In development, `GET /api/v1/routes` lists every registered
route — useful for confirming what actually mounted.

---

## Working on it

### Docker

```bash
docker compose up --build          # start everything, live reload
docker compose logs -f backend     # tail one service
docker compose restart backend     # restart one service
docker compose down                # stop, keep data
docker compose down -v             # stop and wipe data
```

### Backend

The [`Makefile`](backend/Makefile) is self-documenting — `make help` lists every target with a
description. The ones used most:

```bash
cd backend

make dev               # run with Air live reload
make test              # all tests
make test-unit         # unit only
make test-integration  # integration only — needs Postgres and Redis
make vet               # go vet
make lint              # golangci-lint

make migrate-up        # apply pending migrations
make migrate-status    # what's applied
make migrate-down      # roll back one
make migrate-create NAME=add_widget_table

make db-shell          # psql
make redis-shell       # redis-cli
make docs              # regenerate Swagger from handler annotations
```

**Every schema change goes through a migration.** Never alter the database by hand — the
integration job in CI runs migrations against a real Postgres on every PR, so a hand-made change
that isn't in a migration will pass locally and fail there.

### Frontend

```bash
cd frontend
bun install
bun run dev
bun run lint
bun run build
```

### Mobile

```bash
cd mobile
bun install
bun run start        # Expo dev server
bun run ios
bun run android
```

Expo Go is fine for early work, but move to a development build before anything production-like —
Expo Go tracks only the latest SDK and cannot load custom native modules.

---

## Architecture

### Layering

The backend is strictly layered, enforced by convention:

- **Handler** — HTTP only. Reads the request, calls a service, writes a response. No SQL, no
  business logic.
- **Service** — business logic only. No HTTP types, no SQL.
- **Repository** — SQL only, always parameterized. No business logic.
- **Middleware** — cross-cutting concerns: auth, tenancy, permissions, rate limiting.
- **pkg/** — stateless utilities with zero domain knowledge.

The payoff is concrete rather than aesthetic: because services know nothing about HTTP, the mobile
client reuses every service unchanged and adds only four handlers that move the refresh token from
a cookie into a JSON body.

### Multi-tenancy

Every record belongs to an organization, and isolation is enforced in middleware.
`RequireOrganizationParam` compares the `:orgId` in the URL against the `bid` claim in the JWT
before any handler runs — so a repository bug alone cannot leak data across tenants.

Capture endpoints are the exception: they resolve the organization from an API key, an inbound
address, or a page ID instead of a JWT. Every capture query is still organization-scoped.

### Auth

A short-lived JWT access token plus an opaque refresh token. Only hashes of refresh tokens are
stored; the raw value is never logged or returned in a body.

Web keeps the refresh token in an httpOnly cookie. Mobile keeps it in the OS keychain via
`expo-secure-store`, because native has no cookie jar. **Both keep the access token in a plain
module variable, never in a store** — Zustand's `persist` middleware would write it to
localStorage or AsyncStorage.

### Authorization

Permission-based, not role-name-based. Roles bundle permission keys; individual members can carry
per-member overrides on top of their role.

**A new permission must also be added to `frontend/src/lib/permissionGroups.ts`.** A permission
enforced on a route but absent from that file is invisible in the role editor and cannot be
granted through the UI. This has shipped twice; see [KI-009](../../wiki/Known-Issues).

---

## Repository layout

```
BusinessSAAS/
├── .github/workflows/     ci.yml · mobile-ci.yml · deploy.yml (placeholder)
├── backend/
│   ├── cmd/server/        entry point and dependency wiring
│   ├── internal/
│   │   ├── auth/ user/ organizations/ authz/ security/ audit/
│   │   ├── platform/      contacts, engagement — shared across modules
│   │   ├── crm/           leads, pipeline, deals, reports, templates, settings
│   │   ├── hrm/           25 sub-modules
│   │   ├── capture/       apikeys, public, email, social, visitors
│   │   ├── task/ dashboard/ middleware/ database/ config/
│   │   └── migrations/    Goose SQL
│   ├── pkg/               jwt · password · token · response · logger · pagination
│   └── docs/              generated Swagger
├── frontend/src/
│   ├── app/               App Router — (auth), (onboarding), (dashboard)/[orgId]
│   ├── components/ lib/ stores/ hooks/ types/
├── mobile/src/
│   ├── app/               Expo Router — (auth), (dashboard)/[orgId]
│   ├── components/ lib/ stores/ hooks/ theme/ types/
├── docs/                  modules · database · decisions
└── docker-compose.yml
```

---

## Testing

```bash
make test-unit           # services, no external dependencies
make test-integration    # real Postgres + Redis, INTEGRATION=1
```

Integration tests cover auth flows and tenant isolation. CI runs both on every PR, with the
integration job applying migrations to a live database first.

---

## Documentation

| What                                   | Where                                                    |
| -------------------------------------- | -------------------------------------------------------- |
| Guides, architecture, conventions      | [Wiki](../../wiki)                                       |
| Module reference — routes, permissions | [`docs/modules/`](docs/modules/)                         |
| Database — tables, ERD, dictionary     | [`docs/database/`](docs/database/)                       |
| Architecture Decision Records          | [`docs/decisions/`](docs/decisions/)                     |
| Generated API reference                | [`backend/docs/swagger.json`](backend/docs/swagger.json) |

Before writing or editing documentation, read
[Documentation Conventions](../../wiki/Documentation-Conventions). It defines which file owns which
fact — the rule that keeps these from drifting apart.

---

## Contributing

Feature branches off `main`, [Conventional Commits](https://www.conventionalcommits.org), PR with
green CI before merge. Details in the [Git Guide](../../wiki/Git-Guide).

Non-negotiable, each for a reason:

- **Never store a raw secret.** Passwords bcrypt-hashed; refresh, reset, and verification tokens
  stored as hashes; API keys SHA-256 with a display prefix, raw value returned exactly once.
- **Parameterized SQL only.** No concatenation, anywhere.
- **Transactions for multi-step writes** — organization creation, lead conversion, approval
  decisions.
- **Never put the access token in a store.** `persist` would leak it to disk.
- **Review AI-assisted output before committing.** An unreviewed thinking-out-loud comment was
  once committed into the capture module and concealed a feature-breaking bug.

---

## License

Private and proprietary. Not licensed for distribution.
