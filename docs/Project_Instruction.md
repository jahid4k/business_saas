# BUSINESSSAAS — PROJECT MASTER INSTRUCTION

> Last updated: 2026-07 (r8 — audit pass: fixed two dangling "Section 9" references left over from the r5 renumbering that should've said Section 11, and split the reused 🟡 emoji so PAUSED and PARTIAL aren't the same symbol)
> This document is both a Claude system instruction and a personal project reference.
> Update the STATUS blocks and MODULE REGISTRY whenever the project state changes.

---

## 1. PROJECT OVERVIEW

**BusinessSAAS** is a modular, multi-tenant SaaS platform built from scratch. It starts with a complete auth + RBAC foundation and a CRM module, and will expand into HRM, project management, e-commerce admin, and more business modules over time.

**Core principles:**

- Quality over scope. Build fewer things, but build them properly.
- Security is non-negotiable. Never cut security corners for speed.
- Design matters. Enterprise Minimalist — clean, trustworthy, information-dense done well. Not flashy, not decorative. Every screen earns its place through clarity and scannability, not visual flourish.
- Multi-tenancy is the foundation. Every feature lives inside an organization context.
- Backend stays stable. Frontend drives feature prioritization, not the other way around.

---

## 2. CURRENT STATUS

### Phase 1 — Backend Foundation: ✅ COMPLETE

Everything in this phase is done and should not be rebuilt or refactored unless there is a specific bug or a frontend integration demand.

Done:

- Docker Compose setup (backend, frontend placeholder, PostgreSQL, Redis)
- Go backend with clean layered architecture (handler → service → repository)
- 17 database migrations (users, organizations, roles, permissions, memberships, sessions, audit logs, CRM, tasks, platform tables)
- Auth: signup, login, logout, logout-all, refresh token, password reset, OAuth sync
- JWT access token (short TTL) + opaque refresh token stored in httpOnly cookie (cookie name: `bsaas_refresh`, path: `/api/v1/auth`)
- RBAC: roles (owner, admin, manager, member, viewer), permissions, membership, custom/denied permission overrides
- Organization (workspace) model with multi-tenancy and context switching
- Security: session management, login event logging
- Task module (CRUD, permission-gated — used for RBAC testing)
- CRM module: leads, contacts, companies, pipeline, stages, deals, notes, activities, email logs, timeline, reports
- Platform engagement layer (notes, tasks, activities, emails)
- Audit logging (append-only)
- Rate limiting (Redis-backed, on auth endpoints)
- Tests: unit (auth, authz, user, orgs, CRM, pkg) + integration (auth flows, tenant isolation)
- CI workflow (GitHub Actions)

### Phase 2 — Frontend: 🔵 ACTIVE

Building the full admin dashboard frontend. This is not a test interface — it is the real product UI, Enterprise Minimalist quality: clean, scannable, built the way an actual B2B SaaS dashboard should look.

Active work:

- Auth pages (login, signup, password reset)
- Dashboard shell (sidebar, topbar, org switcher, user menu)
- Organization setup and switching
- RBAC management (roles, permissions, members, invitations)
- Task module UI (CRUD, permission-gated actions)
- CRM module UI (leads, contacts, companies, pipeline board, deals, reports)
- Profile and settings pages

_This list hasn't been checked against the real codebase in a while — several of these are probably done. Don't trust it blindly; "Current Focus" below is what's actually being worked on right now._

### Current Focus (r6)

1. **User permission overrides UI** — frontend for the existing per-member override endpoint (Section 5, AUTHZ/RBAC: `rbac/members/:memberId/permissions`). Distinct from full custom-role-from-scratch UI, which stays queued (Section 11).
2. **Complete CRM** — true up Section 8's CRM statuses against real code, close the known Contacts integration gap (see Section 8, CRM — CONTACTS).
3. **Complete HRM frontend** — backend already has 31 routes (departments, positions, employees, leave, reports); nothing built on the frontend yet.

Mobile App is paused, not abandoned — architecture is decided and documented (Section 9/10), implementation just isn't the current priority. See Section 11.

Backend may be modified or extended during Phase 2 if:

- A frontend flow reveals a missing endpoint
- Business logic needs to change to support a UI pattern
- A new CRM sub-feature is needed

### Upcoming — Build Queue

Full list with status and scope: **Section 11 — Upcoming Modules Build Queue**. Nothing there is off-limits — it's next up, one at a time. When one starts, give it a proper entry in Section 5 and/or Section 8, the same as any other module.

---

## 3. TECH STACK

### Backend

| Concern        | Choice                                                |
| -------------- | ----------------------------------------------------- |
| Language       | Go 1.25+                                              |
| HTTP Framework | Fiber v3 (`github.com/gofiber/fiber/v3`)              |
| Database       | PostgreSQL 16+                                        |
| DB Driver      | pgx v5 (`github.com/jackc/pgx/v5`)                    |
| Cache / Rate   | Redis 7+ (`github.com/redis/go-redis/v9`)             |
| Migrations     | Goose (SQL migration files in `internal/migrations/`) |
| JWT            | `github.com/golang-jwt/jwt/v5`                        |
| Password hash  | bcrypt via `golang.org/x/crypto`                      |
| UUID           | `github.com/google/uuid`                              |
| Logger         | `log/slog` + `github.com/lmittmann/tint`              |
| Config         | `github.com/joho/godotenv`                            |
| Module path    | `github.com/mridha/businesssaas`                      |

### Frontend

| Concern       | Choice                                                                                                     |
| ------------- | ---------------------------------------------------------------------------------------------------------- |
| Framework     | Next.js 16.2.9 (latest stable, App Router)                                                                 |
| Language      | TypeScript (strict mode)                                                                                   |
| CSS + styling | Tailwind CSS v4                                                                                            |
| HTTP client   | Axios (single API client with interceptors)                                                                |
| State         | Zustand (three stores — see Section 7)                                                                     |
| Theme         | `next-themes` (light/dark, default light)                                                                  |
| Forms         | React Hook Form + Zod validation                                                                           |
| Animation     | GSAP — used sparingly now (skeleton shimmer, number count-ups, subtle transitions), not as a design pillar |
| Icons         | Lucide React                                                                                               |

### Design System

**Enterprise Minimalist.** Clean, scannable, information-dense done well — the opposite of decorative. If a choice doesn't help someone read data faster, it doesn't earn a place.

| Concern         | Spec                                                                                                                                                                           |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Theme mode      | Light + Dark both. Default: **light**. Token variables structured so dark is a straight swap, not an afterthought.                                                             |
| Light bg        | `#ffffff` surface · `#f8fafc` canvas (page background, one step off-white)                                                                                                     |
| Dark bg         | `#0f172a` surface · `#020617` canvas (slate-900/950 — cool dark, not pure black)                                                                                               |
| Primary/Action  | Indigo/slate-blue — `#4f46e5` primary · `#4338ca` hover/active · same hue in both modes                                                                                        |
| Semantic states | Success `#10b981` (emerald) · Warning `#f59e0b` (amber) · Destructive `#ef4444` (crimson) — used sparingly, only for real signal                                               |
| Borders         | Thin, low-contrast — `#e2e8f0` light · `#1e293b` dark. Dividers, not boxes.                                                                                                    |
| Typography      | One family only — Inter or Geist Sans, no separate display font. Strict scale: large tabular numbers for metrics, bold labels for card headers, muted small text for metadata. |
| Quality target  | Enterprise Minimalist — every screen reads clearly at a glance, nothing competes with the data                                                                                 |
| Border radius   | Subtle (4–8px), never rounded-full on blocks                                                                                                                                   |
| Motion          | Restrained and functional — skeleton loading, hover/focus states, smooth number transitions. No entrance choreography, nothing decorative.                                     |
| Density         | Medium-high — dashboards reward information density over whitespace, but never cramped                                                                                         |
| Implementation  | CSS variables per theme, consumed via Tailwind `dark:` classes                                                                                                                 |

### Infrastructure

| Concern    | Choice                                          |
| ---------- | ----------------------------------------------- |
| Container  | Docker + Docker Compose                         |
| CI         | GitHub Actions                                  |
| Deployment | VPS via Docker Compose (planned)                |
| Secrets    | GitHub Secrets + `.env` files (never committed) |

---

## 4. BACKEND ARCHITECTURE

### Folder Structure

```
backend/
  cmd/server/main.go          ← app entry point, DI wiring
  internal/
    auth/                     ← auth domain (signup, login, tokens)
    user/                     ← user profile
    organizations/            ← org/workspace CRUD and context switching
    authz/                    ← RBAC (roles, permissions, memberships, invitations)
    security/                 ← session and login event management
    task/                     ← task CRUD (permission-gated test module)
    platform/
      contacts/               ← shared contacts + companies (used by CRM and future modules)
      engagement/             ← shared notes, tasks, activities, emails, timeline
    crm/
      leads/                  ← CRM lead management
      pipeline/               ← pipeline and stages
      deals/                  ← deal CRUD + board view
      reports/                ← CRM analytics endpoints
    middleware/               ← auth, business context, logger, rate limit, permission, recover
    database/                 ← postgres pool + redis client
    config/                   ← env loading and validation
    audit/                    ← append-only audit log
    migrations/               ← Goose SQL migration files
    tests/
      unit/                   ← service-level unit tests
      integration/            ← API + DB integration tests
  pkg/
    jwt/                      ← JWT manager
    token/                    ← opaque token generation
    password/                 ← bcrypt helpers
    response/                 ← standard JSON response helpers
```

### Layer Rules

- **Handler**: HTTP only. Reads request, calls service, writes response. No SQL, no business logic.
- **Service**: Business logic only. No HTTP types, no SQL queries. Takes context.
- **Repository**: SQL only. No business logic. Uses parameterized queries always.
- **Middleware**: Request-level cross-cutting concerns (auth check, rate limit, permission check).
- **Pkg**: Stateless utilities with zero domain knowledge (jwt, password, token, response).

Never put business logic in handlers. Never put SQL in services. Never put HTTP types in services.

### Middleware Chain (per protected route)

```
RequireAuth → RequireOrganizationParam(:orgId) → RequirePermission(perm)
```

`RequireAuth` validates JWT, sets `user_id` and `business_id` in context.
`RequireOrganizationParam` validates that the `:orgId` param matches `business_id` in the JWT claims (tenant isolation).
`RequirePermission` resolves the user's effective permissions (role perms + custom + denied) via Redis cache, then checks.

### Key Patterns

**PermissionFunc pattern** — avoids import cycles between route files and middleware:

```go
permFn := func(perm string) fiber.Handler {
    return middleware.RequirePermission(authzSvc, perm)
}
// Usage in routes:
tasks.Get("", permFn("tasks.view"), handler.List)
```

**Opaque refresh tokens** — raw token is returned once to handler (for httpOnly cookie), only the hash is stored in DB. Token is never logged or included in JSON body.

**JWT claims** include: `user_id`, `business_id` (org context), `email`, `role`. Business context is set when user selects/switches org.

**Tenant isolation** — `RequireOrganizationParam` compares `:orgId` in URL against `business_id` in JWT. Cross-org access is blocked at middleware, not just at repository level.

### Error Handling

All errors use a consistent model:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "Invalid email or password"
  },
  "request_id": "..."
}
```

Internal errors (SQL errors, stack traces, token internals) are never exposed to clients. They are logged server-side with `slog`.

### API Response Format

```json
{
  "success": true,
  "data": {},
  "message": "OK",
  "request_id": "..."
}
```

Use `pkg/response` helpers for all responses. Never write raw JSON responses in handlers.

---

## 5. BACKEND MODULE REGISTRY

Each module entry format: `MODULE [Status] — route prefix — key permissions`

---

### SYSTEM [✅ DONE]

Routes: `GET /api/v1/health` · `GET /api/v1/hello` · `GET /api/v1/routes` (dev only)
Notes: Health checks PostgreSQL + Redis. `/routes` lists all registered routes in dev.

---

### AUTH [✅ DONE]

Routes:

```
POST /api/v1/auth/signup
POST /api/v1/auth/login
POST /api/v1/auth/logout
POST /api/v1/auth/logout-all
POST /api/v1/auth/refresh
POST /api/v1/auth/password-reset/request
POST /api/v1/auth/password-reset/confirm
POST /api/v1/auth/oauth/sync
GET  /api/v1/auth/me
```

Token contract:

- Access token → `Authorization: Bearer <token>` header (short TTL, default 15m)
- Refresh token → httpOnly cookie `bsaas_refresh`, path `/api/v1/auth` (long TTL, default 7d)
- Frontend never touches the refresh token directly
- `/refresh` sends cookie, receives new access token in body

**Mobile extension [🔵 ACTIVE]:**

```
POST /api/v1/auth/mobile/signup
POST /api/v1/auth/mobile/login
POST /api/v1/auth/mobile/logout
POST /api/v1/auth/mobile/refresh
```

Same underlying service and repository as above — only the handler differs. Web's `login`/`signup`/`logout`/`refresh` keep setting/reading the httpOnly cookie exactly as before, completely unchanged. The `mobile/*` variants return the refresh token in the JSON response body (for `expo-secure-store`) instead of a cookie, and accept it back in the request body on `mobile/refresh` and `mobile/logout`. `logout-all`, `password-reset/*`, and `me` are shared as-is by both clients — none of them depend on the cookie, so no mobile variant needed.
Assumption to confirm: this assumes `signup` currently auto-authenticates (sets the cookie) the same way `login` does. If it doesn't, `mobile/signup` can just create the user and internally call the same login path before returning.

---

### USER [✅ DONE]

Routes:

```
GET   /api/v1/me
PATCH /api/v1/me
PATCH /api/v1/me/settings
PATCH /api/v1/me/preferences
POST  /api/v1/me/avatar
```

Key type: `SafeUser` (never expose `User` directly — it contains `password_hash`)

---

### ORGANIZATIONS [✅ DONE]

Routes:

```
POST /api/v1/organizations
GET  /api/v1/organizations
GET  /api/v1/organizations/:id
POST /api/v1/organizations/:id/switch   ← issues new JWT with org context
```

Notes: `/businesses` is a backward-compatible alias for all routes. Prefer `/organizations`.
`switch` returns a new access token with `business_id` set. Frontend must store new token.

---

### AUTHZ / RBAC [✅ DONE]

Routes:

```
GET   /api/v1/roles
GET   /api/v1/permissions
GET   /api/v1/members/me
GET   /api/v1/members
POST  /api/v1/members/:userId/role

GET   /api/v1/organizations/:orgId/members
POST  /api/v1/organizations/:orgId/members/invite
GET   /api/v1/organizations/:orgId/members/:memberId
PATCH /api/v1/organizations/:orgId/members/:memberId
PATCH /api/v1/organizations/:orgId/members/:memberId/role
PATCH /api/v1/organizations/:orgId/members/:memberId/status

POST   /api/v1/organizations/:orgId/invitations/:invitationId/resend
DELETE /api/v1/organizations/:orgId/invitations/:invitationId
POST   /api/v1/organizations/:orgId/invitations/:token/accept

GET    /api/v1/organizations/:orgId/rbac/permissions
GET    /api/v1/organizations/:orgId/rbac/permissions/grouped
GET    /api/v1/organizations/:orgId/rbac/permissions/matrix
POST   /api/v1/organizations/:orgId/rbac/check

GET    /api/v1/organizations/:orgId/rbac/roles
POST   /api/v1/organizations/:orgId/rbac/roles
GET    /api/v1/organizations/:orgId/rbac/roles/:roleId
PATCH  /api/v1/organizations/:orgId/rbac/roles/:roleId
DELETE /api/v1/organizations/:orgId/rbac/roles/:roleId
PATCH  /api/v1/organizations/:orgId/rbac/roles/:roleId/permissions
POST   /api/v1/organizations/:orgId/rbac/roles/:roleId/clone

GET   /api/v1/organizations/:orgId/rbac/members/:memberId/permissions
PATCH /api/v1/organizations/:orgId/rbac/members/:memberId/permissions
```

Roles: `owner` · `admin` · `manager` · `member` · `viewer`
Key permissions: `members.view` · `members.update` · `members.invite` · `members.remove` · `roles.view` · `roles.create` · `roles.update` · `roles.delete` · `roles.clone` · `roles.permissions.update` · `members.permissions.view` · `members.permissions.update`

---

### SECURITY [✅ DONE]

Routes:

```
GET    /api/v1/organizations/:orgId/security/sessions
DELETE /api/v1/organizations/:orgId/security/sessions/:sessionId
GET    /api/v1/organizations/:orgId/security/login-events
```

Permissions: `security.sessions.view` · `security.sessions.revoke` · `security.login_events.view`

---

### TASK [✅ DONE]

Routes:

```
GET    /api/v1/organizations/:orgId/tasks
POST   /api/v1/organizations/:orgId/tasks
GET    /api/v1/organizations/:orgId/tasks/:taskId
PATCH  /api/v1/organizations/:orgId/tasks/:taskId
DELETE /api/v1/organizations/:orgId/tasks/:taskId
```

Permissions: `tasks.view` · `tasks.create` · `tasks.update` · `tasks.delete`
Statuses: `todo` · `in_progress` · `done` · `cancelled`
Fields: `title`, `description`, `status`, `dueDate` (RFC3339), `assignedTo` (user id or email)

---

### PLATFORM — CONTACTS [✅ DONE]

Routes:

```
GET    /api/v1/organizations/:orgId/crm/contacts
POST   /api/v1/organizations/:orgId/crm/contacts
GET    /api/v1/organizations/:orgId/crm/contacts/:contactId
PATCH  /api/v1/organizations/:orgId/crm/contacts/:contactId
DELETE /api/v1/organizations/:orgId/crm/contacts/:contactId

GET    /api/v1/organizations/:orgId/crm/companies
POST   /api/v1/organizations/:orgId/crm/companies
GET    /api/v1/organizations/:orgId/crm/companies/:companyId
PATCH  /api/v1/organizations/:orgId/crm/companies/:companyId
DELETE /api/v1/organizations/:orgId/crm/companies/:companyId
GET    /api/v1/organizations/:orgId/crm/companies/:companyId/contacts
```

Permissions: `crm.contacts.view` · `crm.contacts.create` · `crm.contacts.update` · `crm.contacts.delete` · `crm.companies.view` · `crm.companies.create` · `crm.companies.update` · `crm.companies.delete`

---

### PLATFORM — ENGAGEMENT [✅ DONE]

Routes (all under `/organizations/:orgId/crm/`):

```
Notes:      GET/POST /notes  · GET/PATCH/DELETE /notes/:noteId
Tasks:      GET/POST /crm/tasks · GET/PATCH/DELETE/POST(complete/reopen/assign) /crm/tasks/:taskId
Activities: GET/POST /activities · GET/PATCH/DELETE /activities/:activityId
Emails:     GET/POST /emails · GET/DELETE /emails/:emailId
Timeline:   GET /timeline?related_type=&related_id=
```

Permissions: `crm.notes.*` · `crm.tasks.*` · `crm.activities.*` · `crm.emails.*`
Note: These are CRM-scoped. When HRM arrives, same handler is reused with a different module tag.

---

### CRM — LEADS [✅ DONE]

Routes:

```
GET    /api/v1/organizations/:orgId/crm/leads
POST   /api/v1/organizations/:orgId/crm/leads
GET    /api/v1/organizations/:orgId/crm/leads/:leadId
PATCH  /api/v1/organizations/:orgId/crm/leads/:leadId
DELETE /api/v1/organizations/:orgId/crm/leads/:leadId
POST   /api/v1/organizations/:orgId/crm/leads/:leadId/convert
```

Permissions: `crm.leads.view` · `crm.leads.create` · `crm.leads.update` · `crm.leads.delete` · `crm.leads.convert`
Statuses: `new` · `contacted` · `qualified` · `unqualified` · `converted`

---

### CRM — PIPELINE & STAGES [✅ DONE]

Routes:

```
GET/POST    /api/v1/organizations/:orgId/crm/pipelines
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/pipelines/:pipelineId
GET/POST    /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages
POST        /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/reorder
PATCH/DELETE /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/:stageId
```

Permissions: `crm.deals.view` · `crm.deals.create` · `crm.deals.update` · `crm.deals.delete`

---

### CRM — DEALS [✅ DONE]

Routes:

```
GET/POST    /api/v1/organizations/:orgId/crm/deals
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/deals/:dealId
POST        /api/v1/organizations/:orgId/crm/deals/:dealId/move
POST        /api/v1/organizations/:orgId/crm/deals/:dealId/won
POST        /api/v1/organizations/:orgId/crm/deals/:dealId/lost
GET         /api/v1/organizations/:orgId/crm/deals/:dealId/board
```

Permissions: `crm.deals.view` · `crm.deals.create` · `crm.deals.update` · `crm.deals.delete` · `crm.deals.move_stage`

---

### CRM — REPORTS [✅ DONE]

Routes:

```
GET /api/v1/organizations/:orgId/crm/reports/overview
GET /api/v1/organizations/:orgId/crm/reports/summary
GET /api/v1/organizations/:orgId/crm/reports/deals/by-stage
GET /api/v1/organizations/:orgId/crm/reports/deals/by-owner
GET /api/v1/organizations/:orgId/crm/reports/leads/by-source
GET /api/v1/organizations/:orgId/crm/reports/tasks/overdue
GET /api/v1/organizations/:orgId/crm/reports/activities/stats
```

Permissions: `crm.reports.view`

---

### AUDIT [✅ DONE]

Internal only. Append-only log for security-sensitive events. No public API endpoints.
Written to by auth service (login, logout, password reset) and task service.

---

## 6. DATABASE

### Conventions

- UUID primary keys everywhere (`id` = internal, `public_id` = API-safe)
- `created_at`, `updated_at` on every table
- Indexes on lookup-heavy columns (org_id, user_id, email, status)
- All schema changes via Goose migration files — never manually
- Transactions for multi-step operations (org creation, membership changes, password reset)
- Audit logs are append-only (no update/delete)

### Migration Count: 20

Files live in `backend/internal/migrations/`. Run via `goose` or `make migrate`.

### Key Tables

`users` · `organizations` · `permissions` · `roles` · `organization_members` · `auth_accounts` · `sessions` · `verification_tokens` · `subscriptions` · `organization_usage` · `audit_logs` · `tasks` · `crm_leads` · `crm_contacts` · `crm_companies` · `crm_pipelines` · `crm_stages` · `crm_deals` · `crm_notes` · `crm_tasks` · `crm_activities` · `crm_email_logs`

---

## 7. FRONTEND ARCHITECTURE

### Folder Structure

```
frontend/
  src/
    app/                    ← Next.js App Router pages
      (auth)/               ← auth layout group (login, signup, reset)
      (dashboard)/          ← dashboard layout group (all protected pages)
        [orgId]/            ← org-scoped pages
          crm/
            leads/
            contacts/
            companies/
            pipeline/
            deals/
            reports/
          tasks/
          settings/
            members/
            roles/
          security/
      page.tsx              ← redirect to login or dashboard
    components/
      ui/                   ← primitive components (button, input, badge, table, modal)
      layout/               ← sidebar, topbar, breadcrumb, org switcher
      auth/                 ← auth-specific components
      crm/                  ← CRM-specific components
      tasks/                ← task-specific components
      rbac/                 ← roles/permissions/members components
    lib/
      api.ts                ← single Axios client with token interceptor + auto-refresh
      auth.ts               ← auth helpers (silent refresh, logout)
      constants.ts          ← API base URL, app constants
      token.ts              ← module-level token variable (never Zustand, never localStorage)
    stores/
      authStore.ts          ← user profile, org context, auth status
      permissionStore.ts    ← current user's effective permissions list
      uiStore.ts            ← sidebar state, theme preference, transient UI state
    hooks/
      useAuth.ts            ← reads authStore, exposes user + org
      usePermission.ts      ← reads permissionStore, exposes can(perm) helper
      useOrg.ts             ← reads authStore.currentOrg
    types/
      auth.ts               ← AuthUser, TokenPair, SafeUser
      org.ts                ← Business, Membership, MemberWithUser
      rbac.ts               ← Role, Permission, Membership models
      task.ts               ← Task, TaskStatus, TaskListResponse
      crm.ts                ← Lead, Contact, Company, Pipeline, Deal, etc.
      api.ts                ← ApiResponse<T>, ApiError
```

### Auth Flow (frontend)

1. Login → POST `/api/v1/auth/login` → receive `{ access_token, expires_in }` in body, refresh cookie set by server automatically
2. Store `access_token` in `lib/token.ts` module variable — never in Zustand, never in localStorage
3. Set user + org in `authStore`, permissions in `permissionStore`
4. Every API request adds `Authorization: Bearer <token>` via Axios request interceptor
5. On 401 → Axios response interceptor calls POST `/api/v1/auth/refresh` → receive new `access_token` → update module variable → retry original request
6. Logout → POST `/api/v1/auth/logout` → clear token variable → reset all stores
7. On page refresh → silent refresh on app mount: call `/auth/refresh` before rendering any protected UI; if it fails, redirect to login

### Zustand Store Definitions

Three stores. Each is small and focused.

**`authStore`** — who the user is and which org they're in:

```ts
interface AuthStore {
  user: SafeUser | null;
  currentOrg: Business | null;
  membership: MyMembershipResponse | null;
  status: "idle" | "loading" | "authenticated" | "unauthenticated";
  setUser: (user: SafeUser) => void;
  setOrg: (org: Business, membership: MyMembershipResponse) => void;
  reset: () => void;
}
```

**`permissionStore`** — what the user can do in the current org:

```ts
interface PermissionStore {
  permissions: string[]; // e.g. ["tasks.view", "tasks.create", "crm.leads.view"]
  setPermissions: (perms: string[]) => void;
  can: (perm: string) => boolean;
  canAny: (perms: string[]) => boolean;
  reset: () => void;
}
```

**`uiStore`** — sidebar and theme, nothing else:

```ts
interface UIStore {
  sidebarOpen: boolean;
  theme: "dark" | "light";
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;
  setTheme: (theme: "dark" | "light") => void;
}
```

**Hard rules for Zustand:**

- Never add `persist` middleware to `authStore` or `permissionStore` — persisted auth state is a security risk
- `persist` is allowed on `uiStore` only (sidebar state, theme preference)
- The access token lives in `lib/token.ts` as a plain module variable — not in any store
- If you see a PR that puts a token into Zustand, reject it

### Theme Implementation

`next-themes` wraps the app at the root layout level with `defaultTheme="light"`. Tailwind's `dark:` classes handle all visual switching. CSS variables define the token values per theme in `globals.css`:

```css
:root {
  --bg-canvas: #f8fafc;
  --bg-surface: #ffffff;
  --border: #e2e8f0;
  --accent: #4f46e5;
  --accent-hover: #4338ca;
  --success: #10b981;
  --warning: #f59e0b;
  --destructive: #ef4444;
}

.dark {
  --bg-canvas: #020617;
  --bg-surface: #0f172a;
  --border: #1e293b;
  --accent: #4f46e5;
  --accent-hover: #4338ca;
  --success: #10b981;
  --warning: #f59e0b;
  --destructive: #ef4444;
}
```

The indigo accent and semantic colors are identical in both modes — only backgrounds, surfaces, and borders shift. Components use `dark:` Tailwind classes — never inline theme checks in JS.

`uiStore.theme` drives the `next-themes` `setTheme()` call and is persisted to localStorage (theme preference is safe to persist).

### Organization Context Flow

1. After login, user lands on org selection screen if no org is active
2. User selects org → POST `/api/v1/organizations/:id/switch` → receive new `access_token` with `business_id` claim
3. Store new token, store `orgId` in context and URL (`/[orgId]/...`)
4. All subsequent API calls use the org-scoped URL pattern

### Permission Pattern (frontend)

```tsx
const { can } = usePermission(); // reads from permissionStore — no API call

// Gates UI — does NOT replace backend enforcement
{
  can("tasks.create") && <Button onClick={openCreateModal}>New Task</Button>;
}
{
  can("tasks.delete") && <Button onClick={handleDelete}>Delete</Button>;
}
```

`permissionStore` is populated after org switch by calling `GET /api/v1/members/me`, which returns `{ permissions: string[] }`. It is reset when the user switches org or logs out. Backend enforces on every request — frontend gates are UX only.

### API Client Contract

```ts
// lib/token.ts — access token lives here, nowhere else
let accessToken: string | null = null;
export const getToken = () => accessToken;
export const setToken = (t: string | null) => {
  accessToken = t;
};

// lib/api.ts
const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL,
  withCredentials: true,
});

// Request interceptor → attach Authorization: Bearer <token> from lib/token.ts
// Response interceptor → on 401: call /auth/refresh → setToken(newToken) → retry
// withCredentials: true is required for the httpOnly refresh cookie to be sent cross-origin
```

**Why not Zustand for the token:** Zustand state can be accidentally persisted via the `persist` middleware, putting the token in `localStorage`. A plain module variable has no such risk — it lives only in memory and is cleared on page refresh (triggering the silent refresh flow).

### Component Quality Standard

Every component must meet these standards:

- Fully themed — light and dark both look intentional, not just inverted
- Indigo accent on interactive elements (buttons, links, focus rings) in both modes — used for actions and active states, not decoration
- One typeface family (Inter or Geist Sans) with a strict scale — large tabular numbers for metrics, bold labels for headers, muted small text for metadata. No separate display font.
- Clear action hierarchy: primary filled buttons, secondary outlined buttons, tertiary text links — never more than one primary action visible at once
- Skeleton loading states on every data-fetching component — never a blank box, never a bare spinner where a skeleton would show shape
- Keyboard accessible: visible focus rings, tab order that matches visual order, escape closes modals
- Strong text contrast ratios (WCAG AA minimum) and explicit semantic ARIA landmarks on layout regions
- Responsive (works on 1280px minimum, designed for 1440px)
- No placeholder "coming soon" states — if a page is in scope, it is built properly
- Error states and empty states are designed, not default browser behavior

### Dashboard Page Pattern

The default anatomy for any data-overview page (CRM Reports, future HRM Reports, org dashboard home) — not a rule for every page, but the template when a page's job is "show me how things are going":

1. **KPI strip** — 3–4 metric cards in a row: absolute value, small sparkline, percentage change with a directional arrow (`+12.4%` in emerald, `-3.1%` in crimson)
2. **Data visualization** — split-pane below the strip: a primary time-series line/area chart (the main metric over time) next to a secondary horizontal bar chart (category breakdown)
3. **Data table** — full-width below the charts: search input, multi-select filters, export action in the header; rows get hover states, status badges, and a `⋯` context menu for inline actions

List views that aren't overview pages (Leads, Tasks, Members) skip the KPI/chart layers and go straight to the table — don't force the full pattern where it doesn't fit.

---

## 8. FRONTEND MODULE REGISTRY

### AUTH PAGES [🔵 ACTIVE]

- `/login` — Email + password login form, link to signup
- `/signup` — Signup form (firstName, lastName, email, password)
- `/forgot-password` — Request password reset (email input)
- `/reset-password` — Confirm password reset (token + new password)

Auth pages have their own layout. No sidebar. Full-page design — light mode uses the white/slate canvas, dark mode uses the slate-900/950 surfaces. Indigo accents in both.

### ONBOARDING [🔵 ACTIVE]

- `/create-organization` — Create first org (name, slug, optional fields)
- `/select-organization` — Select active org if user has multiple

These are shown after login when no org context exists.

### DASHBOARD SHELL [🔵 ACTIVE]

- Sidebar with module navigation
- Topbar with org switcher, user menu, notifications placeholder
- Org context persisted in URL (`/[orgId]/...`)
- Collapsible sidebar

### TASKS [🔵 ACTIVE]

- `/[orgId]/tasks` — List with filters (status, assignee, sort), permission-gated actions
- Inline create form, inline edit, delete with confirmation
- Permission gates: create, update, delete based on user role

### SETTINGS — MEMBERS [🔵 ACTIVE]

- `/[orgId]/settings/members` — Members list + invite + role assignment
- Invitation list with resend/revoke actions
- Role badge, status badge, action menu per member

### SETTINGS — ROLES & PERMISSIONS [🔵 ACTIVE]

- `/[orgId]/settings/roles` — Role list, create role, clone role, delete role
- `/[orgId]/settings/roles/:roleId` — Role detail with permission toggles
- Permission matrix view (all roles × all permissions)
- **Current focus:** per-member permission override screen — likely on the member's row/detail in `/[orgId]/settings/members`, showing their role's base permissions plus grant/deny toggles that call `GET/PATCH .../rbac/members/:memberId/permissions` (Section 5). Smaller and narrower than the role list above — this overrides individual grants on top of a role, it doesn't build new roles.

### PROFILE & SETTINGS [🔵 ACTIVE]

- `/settings/profile` — Edit display name, avatar, timezone
- `/settings/security` — Active sessions list, revoke session, login history

### CRM — LEADS [🔵 ACTIVE]

- `/[orgId]/crm/leads` — Lead list with filters (status, source, owner)
- Lead detail page or side panel
- Convert lead flow → creates contact + deal

### CRM — CONTACTS [🔵 ACTIVE — known gap, see below]

- `/[orgId]/crm/contacts` — Contacts list with search
- `/[orgId]/crm/companies` — Companies list
- Company detail with linked contacts
- **Known pending integration work:** `main.go` wiring, a type conflict in `crm.ts` still unresolved, and a migration that hasn't been run. Backend itself is delivered — this is purely the last mile of hooking it up. Needs the current codebase to confirm exact state before fixing.

### CRM — PIPELINE & DEALS [🔵 ACTIVE]

- `/[orgId]/crm/pipeline` — Kanban board view (deals by stage), drag-to-move
- Deal detail side panel or page
- Mark won/lost

### CRM — REPORTS [🔵 ACTIVE]

- `/[orgId]/crm/reports` — Overview dashboard
  - Deals by stage (bar chart)
  - Deals by owner (table)
  - Leads by source (pie/donut)
  - Overdue tasks (list)
  - Activity stats (summary cards)

### SECURITY [🔵 ACTIVE]

- `/[orgId]/security/sessions` — Session list, revoke session
- `/[orgId]/security/events` — Login event log

---

## 9. MOBILE ARCHITECTURE

_Status: ⏸️ paused, see Section 11 — architecture below is decided and still valid, implementation just isn't the current priority._

Expo + React Native client. Ports the same architecture as the web frontend (Section 7) wherever the platform allows, and only diverges where native constraints force it — token storage is the main one.

### Folder Structure

```
mobile/
  app/                        ← Expo Router file-based routes
    (auth)/
      login.tsx
      signup.tsx
      forgot-password.tsx
      reset-password.tsx
    (dashboard)/
      [orgId]/
        index.tsx             ← dashboard home
        tasks/
        crm/
          leads/
          contacts/
          companies/
          pipeline/
          deals/
          reports/
        settings/
          members/
          roles/
        security/
    create-organization.tsx
    select-organization.tsx
    _layout.tsx                ← root layout: theme provider + auth gate
  components/
    ui/                        ← primitive components (button, input, badge, etc.)
    layout/                    ← tab bar / header, org switcher
    crm/
    tasks/
    rbac/
  lib/
    api.ts                     ← Axios client, same interceptor shape as frontend/lib/api.ts
    auth.ts                    ← auth helpers, silent refresh, logout
    secureToken.ts             ← SecureStore-backed refresh token + in-memory access token
    constants.ts
  stores/
    authStore.ts                ← same shape as frontend
    permissionStore.ts
    uiStore.ts                  ← persisted via AsyncStorage instead of localStorage
  hooks/
    useAuth.ts
    usePermission.ts
    useOrg.ts
  theme/
    tokens.ts                   ← same colors as globals.css, as a plain JS object
    ThemeProvider.tsx
  types/                        ← mirrors frontend/src/types/*
```

### Auth Flow (mobile)

1. Login → POST `/api/v1/auth/mobile/login` → receive `{ access_token, refresh_token, expires_in }` in the body — no cookie involved
2. Store `refresh_token` via `expo-secure-store`; keep `access_token` in an in-memory module variable (`lib/secureToken.ts`) — same separation principle as web, different storage mechanism
3. Set user + org in `authStore`, permissions in `permissionStore` — identical shape to web
4. Axios request interceptor attaches `Authorization: Bearer <token>`
5. On 401 → response interceptor reads the refresh token from SecureStore → POST `/api/v1/auth/mobile/refresh` with `{ refresh_token }` → store the new (possibly rotated) tokens → retry the original request
6. Logout → read refresh token from SecureStore → POST `/api/v1/auth/mobile/logout` with `{ refresh_token }` → clear SecureStore + in-memory token → reset all stores
7. On cold start → read refresh token from SecureStore → if present, call `mobile/refresh` before rendering any protected route; if it fails or is absent, route into `(auth)`

### Navigation & Route Protection

- Expo Router — file-based, same mental model as the Next.js App Router already in use
- `(auth)` and `(dashboard)` groups mirror the web's layout groups; `[orgId]` dynamic segment mirrors the web's URL-based org context
- Gate the `(dashboard)` group with Expo Router's Protected Routes, keyed off `authStore.status === 'authenticated'`

### State Management

Same three Zustand stores as web, ported with identical interfaces:

- `authStore`, `permissionStore` — same hard rule: never add `persist` middleware, ever
- `uiStore` may persist (theme, nav state) — via `@react-native-async-storage/async-storage`, since `localStorage` doesn't exist in React Native

### Theming

- No CSS variables on native. Port the same values from `globals.css` into a plain `theme/tokens.ts` object with `dark` and `light` variants
- RN's built-in `useColorScheme()` supplies the OS-level default; `uiStore.theme` overrides it once the user picks explicitly — same behavior as `next-themes`, different mechanism
- Load Inter via `expo-font` (`useFonts`) — one typeface family, matching web. Without this, text silently falls back to system fonts and breaks visual consistency.
- Plain `StyleSheet.create` for a first pass rather than pulling in a Tailwind-for-RN library — keeps the surface area small; revisit if styling velocity becomes a problem

### API Client Contract

```ts
// lib/secureToken.ts
import * as SecureStore from "expo-secure-store";

let accessToken: string | null = null;
export const getAccessToken = () => accessToken;
export const setAccessToken = (t: string | null) => {
  accessToken = t;
};

const REFRESH_KEY = "bsaas_refresh_token";
export const getRefreshToken = () => SecureStore.getItemAsync(REFRESH_KEY);
export const setRefreshToken = (t: string | null) =>
  t
    ? SecureStore.setItemAsync(REFRESH_KEY, t)
    : SecureStore.deleteItemAsync(REFRESH_KEY);

// lib/api.ts
const api = axios.create({ baseURL: process.env.EXPO_PUBLIC_API_URL });
// Request interceptor  → attach Authorization: Bearer <accessToken>
// Response interceptor → on 401: getRefreshToken() → POST /auth/mobile/refresh → store new tokens → retry
```

### Component Quality Standard (mobile)

- Fully themed dark/light, same design tokens as web
- Indigo accent on interactive elements
- One typeface (Inter), same scale hierarchy as web — no separate display font
- Native feel over pixel-parity — respect iOS vs Android navigation conventions, safe areas, haptics (`expo-haptics`) on key actions
- Designed loading/empty/error states, not default RN placeholders
- A screen isn't done until it's been checked on both iOS and Android

### Deployment

- EAS Build for iOS/Android binaries, EAS Submit for store submission, EAS Update for OTA JS updates between store releases
- Expo Go is fine for early development only — move to development builds well before anything resembling production; Expo Go tracks only the latest SDK and isn't meant for shipped apps

---

## 10. MOBILE MODULE REGISTRY

Same status convention as Section 5/8. Nothing here is built yet — everything starts `⚪ QUEUED` and flips to `🔵 ACTIVE` the moment work starts on it, one screen group at a time.

Suggested v1 scope below is the highest-value, most mobile-native slice rather than a full port of every admin screen — Settings/Roles and Security are flagged lower priority. Override freely.

---

### AUTH SCREENS [⚪ QUEUED]

Login, signup, forgot password, reset password — same fields as web (Section 8, AUTH PAGES).

---

### ONBOARDING [⚪ QUEUED]

Create organization, select organization — shown after login when no org context exists.

---

### DASHBOARD SHELL [⚪ QUEUED]

Tab bar or drawer navigation (mobile equivalent of the web sidebar), org switcher, profile menu.

---

### TASKS [⚪ QUEUED]

List with filters, create/edit, permission-gated actions — same permission set as web (`tasks.*`).

---

### CRM — LEADS & PIPELINE [⚪ QUEUED]

Lead list + detail, convert flow, pipeline board (list view first — a full drag-to-reorder Kanban is a lot of native gesture work for v1; move-via-menu instead of drag, revisit later).

---

### CRM — CONTACTS [⚪ QUEUED]

Contacts and companies list + detail.

---

### CRM — REPORTS [⚪ QUEUED — v1: summary cards only]

Full charts (Section 8's bar/pie breakdowns) are a lot of screen real estate for mobile — start with summary numbers, add charts later if it earns its place.

---

### SETTINGS — MEMBERS, ROLES & PERMISSIONS [⚪ QUEUED — lower priority]

Admin-heavy screens, arguably fine to stay web-only for v1. Revisit after the above ships.

---

### SECURITY [⚪ QUEUED — lower priority]

Session list, revoke, login events. Likely fine as web-only initially too.

---

## 11. UPCOMING MODULES — BUILD QUEUE

Everything below is real, planned work — not a do-not-touch list. `⚪ QUEUED` means no code exists yet; the moment one of these starts, it gets promoted here to `🔵 ACTIVE` and gets a full entry in Section 5 and/or Section 8 with real routes and permissions, the same as any other module. Working through this list is the current priority, one item at a time — see Section 2 for which one is active now.

---

### MFA / 2FA [⚪ QUEUED]

TOTP enrollment + verification, backup codes, per-org enforcement policy. Would live in `internal/auth/`; needs a new table for secrets and a frontend enrollment/verify flow. Nothing blocking a start.

---

### SOCIAL LOGIN / SSO [⚪ QUEUED]

Google / Microsoft / GitHub OAuth flows — consent screen, token exchange. `POST /api/v1/auth/oauth/sync` already exists as a backend hook (Section 5, AUTH) but no provider-specific flow is wired end-to-end. This item is that remaining work, not a from-scratch build.

---

### EMAIL SENDING [⚪ QUEUED]

Transactional email provider integration (SES / Postmark / Resend / etc.) for verification, invites, and password reset — currently all token-only. Distinct from the CRM lead auto-capture system's inbound email parsing, which is a separate, already-designed initiative. Unblocks real invite delivery and real password-reset UX once built.

---

### COMPLEX CUSTOM ROLE UI [⚪ QUEUED — frontend only]

A role-builder UI for creating fully arbitrary custom roles from scratch. Backend already supports this in full: `POST/PATCH/DELETE /organizations/:orgId/rbac/roles`, `/clone`, `/permissions` (Section 5, AUTHZ/RBAC). This is purely a frontend build.
Not to be confused with the per-member permission override UI, which is current focus (Section 2, Section 8 — SETTINGS ROLES & PERMISSIONS). That one adjusts individual grants on top of a role; this one builds whole new roles. They may turn out to overlap enough that this entry becomes unnecessary — revisit once overrides ship.

---

### RESOURCE-LEVEL PERMISSIONS [⚪ QUEUED]

Per-record access control (e.g. "can only see deals they own"), beyond today's module/action-level RBAC. Large surface area — touches every repository's query layer. Give this its own ADR when it starts.

---

### BILLING & SUBSCRIPTION MANAGEMENT [⚪ QUEUED]

Payment provider integration, plan tiers, usage limits, invoices, billing settings UI. The `organization_usage` table already exists in the schema (Section 6) — likely a head start for usage-based limits.

---

### HRM MODULE [🟡 PARTIAL — backend done, frontend not started]

Backend complete: departments, positions, employees, leave management, reports — 31 routes. Frontend is the only remaining piece — closer to done than anything else in this queue.

---

### PROJECT MANAGEMENT MODULE [⚪ QUEUED]

A full projects module (milestones, dependencies, etc.) — broader than the existing generic `task` module, which stays a simple RBAC-testing CRUD.

---

### E-COMMERCE ADMIN MODULE [⚪ QUEUED]

Per the original long-term vision (CRM, HRM, Accounting, Projects, E-commerce).

---

### ACCOUNTING MODULE [⚪ QUEUED]

Part of the stated long-term vision; added here alongside the rest — drop it if it shouldn't be tracked.

---

### ERP MODULE [⚪ QUEUED — scope undefined]

"ERP" is an umbrella term rather than a concretely scoped module like HRM or CRM. It may end up being the combination of HRM + Accounting + Projects + Inventory rather than a standalone build — worth pinning down scope before this one gets picked up.

---

### FULL PRODUCTION DEPLOYMENT [🟠 IN PROGRESS]

Month 1 production roadmap is active: `docker-compose.prod.yml`, Caddy TLS config, `.env.production` template, Sentry wiring, and a nightly rclone backup script (14-day retention) are done. Remaining steps are Mridha's direct actions: VPS purchase, DNS, secret generation, storage account, live deploy, restore drill.

---

### MOBILE APP [⏸️ PAUSED]

Expo + React Native. Architecture decided — see Section 9 (Mobile Architecture) and Section 10 (Mobile Module Registry), both still valid for whenever this resumes. Paused, not abandoned: no code written yet, so nothing is being left mid-flight. Deprioritized in favor of RBAC overrides + CRM/HRM completion (Section 2, Current Focus).

---

## 12. CORE SECURITY STANDARDS

These apply to every line of code in this project. Never compromise on these.

**Backend:**

- Never store plaintext passwords — bcrypt always
- Never store raw refresh/reset/email tokens — hash first, store hash
- Never log passwords, tokens, secrets, or any sensitive data
- Parameterized SQL only — never string-concatenate SQL
- Validate all request bodies at handler layer
- Sanitize and normalize email (lowercase, trim)
- Return generic errors for login and auth operations — never reveal why login failed
- Log detailed errors server-side only
- Secure CORS (explicit origins, no wildcard in production)
- `HttpOnly: true` on refresh cookie — always, not configurable
- Transactions for multi-step write operations
- Audit log for: login attempts, logout, password reset, role changes, permission changes

**Frontend:**

- Access token in `lib/token.ts` module variable only — never Zustand, never localStorage, never sessionStorage
- Refresh token is httpOnly cookie — frontend never reads it, never stores it
- Never add `persist` middleware to `authStore` or `permissionStore`
- `withCredentials: true` on all API requests (needed for cookie to send)
- Never show stack traces or backend error internals in UI
- Permission checks are UX only — backend enforces on every request
- No sensitive data in URL query params

---

## 13. DEVELOPMENT WORKFLOW

### Before implementing any feature

1. State what the feature is and which backend routes it uses
2. Identify which frontend files to create or modify
3. Identify any backend changes needed (new endpoint, field added, etc.)
4. Then provide code — complete files, not snippets, unless snippet is explicitly asked for

### When writing code

- Always include file paths as comments at top of file
- Prefer complete files when practical
- Use idiomatic Go for backend (`context.Context` in service/repo, explicit error handling)
- Use idiomatic Next.js + TypeScript for frontend (no `any`, proper async/await)
- Keep API calls in `lib/api.ts` or module-specific API helpers — not inside components
- Keep business logic out of components — components should be dumb consumers

### When reviewing code

1. Score out of 10
2. What is good
3. Problems found
4. Security risks
5. Refactor suggestions
6. Corrected code if needed
7. Final recommendation

### When debugging

1. Ask for or look at the exact error, relevant file, and context
2. Explain root cause
3. Provide the smallest safe fix first
4. Then suggest a better long-term fix if different

---

## 14. RESPONSE FORMAT

### For implementation tasks

1. Goal
2. Files to create or modify
3. Code (complete files with path comments)
4. Explanation of key decisions
5. Security notes (if relevant)
6. What to do next

### For architecture questions

1. Simple explanation
2. Recommended choice for this project
3. Alternatives considered
4. Trade-offs
5. Final decision

### For feature planning / brainstorming

1. Summarize what we're solving
2. Options or approaches
3. Recommended approach
4. What it unlocks next

---

## 15. VERSION AWARENESS

Before writing code that depends on any of these, check for version-specific API differences:

| Package / Framework | Version in use                                                           |
| ------------------- | ------------------------------------------------------------------------ |
| Go                  | 1.25                                                                     |
| Fiber               | v3.2.0                                                                   |
| pgx                 | v5.6.0                                                                   |
| go-redis            | v9.19.0                                                                  |
| golang-jwt/jwt      | v5.3.1                                                                   |
| Next.js             | latest stable (check if unsure)                                          |
| Tailwind CSS        | v4                                                                       |
| React               | latest stable                                                            |
| Zustand             | v5 (latest stable)                                                       |
| next-themes         | latest stable                                                            |
| React Hook Form     | v7                                                                       |
| Axios               | v1                                                                       |
| Expo SDK            | 57 (confirm exact patch via `npx create-expo-app` at scaffold time)      |
| Expo Router         | bundled with Expo SDK 57                                                 |
| React Native        | whatever SDK 57 pins — check `package.json` after scaffold, don't assume |
| expo-secure-store   | latest compatible with SDK 57                                            |

Do not use Fiber v2 API patterns with Fiber v3. Do not use Tailwind v3 class patterns that don't exist in v4. Expo SDK numbers move fast (three releases a year) — re-check the current version before scaffolding `mobile/`, don't trust this table blindly by the time you get to it.

---

## HOW TO UPDATE THIS DOCUMENT

When phase status changes:

- Update Section 2 STATUS blocks
- Update MODULE REGISTRY entries (change `🔵 ACTIVE` to `✅ DONE`)

When a new backend module is added:

- Add a new entry to Section 5 (Backend Module Registry) with its routes and permissions

When a new frontend page is built:

- Update Section 8 (Frontend Module Registry)

When a new mobile screen is built:

- Update Section 10 (Mobile Module Registry), same status convention as Section 5/8
- Note any deviation from the architecture in Section 9 (Mobile Architecture) if the plan changes mid-build

When a Zustand store gains new fields:

- Update the store interface in Section 7 (Zustand Store Definitions)
- Never add token or sensitive credential fields — see hard rules

When a major architectural decision is made:

- Add a note to the relevant section (3, 4, or 7)

When design tokens change (colors, spacing, radius):

- Update Section 3 (Design System) and `globals.css` together — keep them in sync

When something in the build queue starts real work:

- Update its status from `⚪ QUEUED` to `🔵 ACTIVE` in Section 11 (Upcoming Modules — Build Queue)
- Add a proper entry to Section 5 and/or Section 8 with real routes and permissions
- Mark it `✅ DONE` in Section 11 (or remove the entry) once it ships
- Update Section 2's phase status if it changes the active phase

Keep this document current. A stale instruction is worse than no instruction.
