# BUSINESSSAAS — PROJECT MASTER INSTRUCTION

> Last updated: 2026-07-22 (r14 — Mobile App promoted ⚪ NOT STARTED → 🔵 ACTIVE. Section 9's
> entry slimmed to a pointer, same pattern as CAPTURE. Section 5 → AUTH gained a real MOBILE
> subsection with the actual route list, and resolved the r10 draft's one open question: checked
> against `frontend/src/app/(auth)/signup/page.tsx`, `Signup` does NOT auto-authenticate — the web
> client calls `login()` separately right after. `MobileSignup` mirrors that instead of inventing
> an auto-login path mobile alone would exercise. Mobile Architecture and Mobile Module Registry
> restored from the r10 archive as new Section 14 and Section 15 — appended at the end rather than
> reinstated at their old Section 9/10 slot, to avoid renumbering every cross-reference in Sections
> 1–13. Revisit if the old position is wanted back; it's a rename pass, not a content change.
> Section 13's version table gained Expo SDK 57 (57.0.7)/React Native 0.86/Expo Router/
> expo-secure-store rows, plus a scaffolding gotcha: `create-expo-app` without
> `--template default@sdk-57` currently still lands on SDK 54 during the transition window.
> Zero mobile code written — this revision is the doc-level start only, implementation is next.
> Note: a stale snapshot of an older doc draft had the mobile auth extension marked 🔵 ACTIVE with
> code implied; the real `backend/internal/auth/routes.go` has no `/mobile/*` group at all. That
> snapshot was planning text, not shipped state — flagging in case it resurfaces elsewhere.)
> Last updated: 2026-07-21 (r14 — introduced the Collection View Pattern (Section 7): a
> Notion-style grouped/collapsible list with borderless rows, full inline editing, and inline
> quick-add, replacing the bordered-table pattern for single-entity lists. Tasks is the first
> page rebuilt on it (Section 8). Design tokens unchanged — this is a layout/interaction
> decision, not a token change. Candidate for Leads/Members when next revisited; not a mandate
> to rewrite them now.)
> Last updated: 2026-07-20 (r13 — Section 13's Tailwind v4 reminder expanded from a vague "don't use v3 patterns" into six concrete, verified syntax rules: `@import "tailwindcss"` entry point, `@theme` CSS config over `tailwind.config.js`, `bg-linear-*` gradient naming, parens vs brackets for arbitrary CSS-variable references, the gray-200 default border color, and `@utility` for custom utilities.)
> Last updated: 2026-07-20 (r12 — reconciled against the real r11 after a conversation had drifted onto a stale, incorrect picture of HRM's status; see note at bottom of this entry. Structural changes: removed "Current Focus" from Section 2 — its content either lived in Section 5 CAPTURE already or was pure priority-ranking, so it's gone rather than moved. Folded the Phase-2 backend-modification carve-out into a new general principle in Section 1: the system is interconnected, touching a connected module to finish what you're building is normal, not an exception requiring special permission. Section 9 renamed from "Build Queue" to "Unbuilt Module Registry," `⚪ QUEUED` → `⚪ NOT STARTED` everywhere, "order is decided when the current focus ships" and the CRM→HRM priority ranking removed — nothing in it carries priority ordering. Real technical dependency notes were kept as-is throughout (Capture Fix Pass B blocking deployment, ERP needing HRM/Accounting/Projects scoped first, sales velocity needing the stage-history table) — those are facts about how the system works, not schedule decisions, and stay regardless of the no-priority-ordering rule. — Also: a prior chat session spent several turns planning HRM frontend architecture, an RBAC override UI, and a full Mobile Architecture rebuild, all based on a wrong claim that HRM's frontend hadn't been built and Contacts had an unresolved integration gap. Neither was true against this document. None of that work was carried in here.)
> Last updated: 2026-07-15 (r11 — CRM Auto Lead-Capture backend built end-to-end and audited: new `internal/capture/` tree (apikeys, public, email, social, visitors), `RequireAPIKey` middleware, migrations 00057–00064. **Audit verdict: architecture sound, not yet functional** — 3 of 5 capture sources cannot create leads (`created_by` empty-UUID bug), social connect hits a column-name mismatch, visitors dashboard 403s on a nonexistent permission, and both webhook endpoints ship with zero signature verification. Full list in Section 5 → CAPTURE → Known open items; Fix Pass A/B is the current work item. Capture frontend not started. r11 also folds in four items shipped after r10 that never made it into the doc: CRM Templates (00054–55), CRM Settings with lead round-robin routing (00056, wired into lead creation via `GetLastAssignedLeadOwner`), HRM dynamic employee statuses (00053, `status_id` FK on `hrm_employees`), and the CRM Agenda page. Migration count 52 → 64, table count → 74. Structural change per Mridha's decision: **the "deferred" concept is removed from this document** — no more "deliberately last", "paused", or "confirmed still deferred"; everything not shipped is simply ⚪ QUEUED in one flat build queue. Mobile Architecture and Mobile Module Registry sections (old 9 & 10) deleted — zero code exists; the decided architecture is preserved in git history (r10) and gets restored when mobile work actually starts. Sections renumbered accordingly.)
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
- The system is interconnected — CRM, HRM, RBAC, and the platform layer share models, permissions, and the engagement layer. Don't let scope discipline block necessary work: if finishing what you're building requires touching a connected module, touch it. Correct and complete beats narrowly scoped.

---

## 2. CURRENT STATUS

### Phase 1 — Backend Foundation: ✅ COMPLETE

Everything in this phase is done and should not be rebuilt or refactored unless there is a specific bug or a frontend integration demand.

Done:

- Docker Compose setup (backend, frontend placeholder, PostgreSQL, Redis)
- Go backend with clean layered architecture (handler → service → repository)
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
- Tests: unit (auth, authz, user, orgs, CRM, HRM, pkg) + integration (auth flows, tenant isolation)
- CI workflow (GitHub Actions)

### Phase 2 — Frontend + CRM buildout: 🔵 ACTIVE

Building the full admin dashboard frontend plus the CRM Advanced Functionality Pass. This is not a test interface — it is the real product UI, Enterprise Minimalist quality.

Shipped and verified (see Section 7 for the full frontend registry):

- Auth pages, onboarding, dashboard shell, org switching
- RBAC management (roles, permissions, members, invitations, per-member overrides, admin password reset)
- Task module UI
- CRM UI: leads, contacts, companies, pipeline board, deals, reports, agenda, setup (routing, templates)
- HRM UI: all 8 phases complete (verified r9), plus dynamic employee statuses setup page (post-r10)
- Profile and settings pages, security pages

Scope beyond what's shipped lives in Section 5 (backend), Section 8 (frontend), and Section 9 — status per item lives in those registries, not here, so there's one place to check, not two that can quietly disagree. The Capture module (Section 5 → CAPTURE) has the most detailed known-defect write-up in the whole doc right now — read it there rather than a restated summary here.

### Unbuilt Modules

Full list: **Section 9 — Unbuilt Module Registry**. Anything in it can be picked up any time.

---

## 3. TECH STACK

### Backend

| Concern        | Choice                                                                                                       |
| -------------- | ------------------------------------------------------------------------------------------------------------ |
| Language       | Go 1.25+                                                                                                     |
| HTTP Framework | Fiber v3 (`github.com/gofiber/fiber/v3`)                                                                     |
| Database       | PostgreSQL 16+                                                                                               |
| DB Driver      | pgx v5 (`github.com/jackc/pgx/v5`)                                                                           |
| Cache / Rate   | Redis 7+ (`github.com/redis/go-redis/v9`)                                                                    |
| Migrations     | Goose (SQL migration files in `internal/migrations/`)                                                        |
| JWT            | `github.com/golang-jwt/jwt/v5`                                                                               |
| Password hash  | bcrypt via `golang.org/x/crypto`                                                                             |
| API key hash   | SHA-256 (`crypto/sha256`) — high-entropy keys, indexed exact-match lookup; bcrypt deliberately NOT used here |
| UUID           | `github.com/google/uuid`                                                                                     |
| Logger         | `log/slog` + `github.com/lmittmann/tint`                                                                     |
| Config         | `github.com/joho/godotenv`                                                                                   |
| Module path    | `github.com/mridha/businesssaas`                                                                             |

### Frontend

| Concern       | Choice                                                                                                 |
| ------------- | ------------------------------------------------------------------------------------------------------ |
| Framework     | Next.js 16.2.9 (latest stable, App Router)                                                             |
| Language      | TypeScript (strict mode)                                                                               |
| CSS + styling | Tailwind CSS v4                                                                                        |
| HTTP client   | Axios (single API client with interceptors)                                                            |
| State         | Zustand (three stores — see Section 6)                                                                 |
| Theme         | `next-themes` (light/dark, default light)                                                              |
| Forms         | React Hook Form + Zod validation                                                                       |
| Animation     | GSAP — used sparingly (skeleton shimmer, number count-ups, subtle transitions), not as a design pillar |
| Icons         | Lucide React                                                                                           |

### Design System

**Enterprise Minimalist.** Clean, scannable, information-dense done well — the opposite of decorative. If a choice doesn't help someone read data faster, it doesn't earn a place.

| Concern         | Spec                                                                                                                                                                                           |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Theme mode      | Light + Dark both. Default: **light**. Token variables structured so dark is a straight swap, not an afterthought.                                                                             |
| Light bg        | `#ffffff` surface · `#f8fafc` canvas (page background, one step off-white)                                                                                                                     |
| Dark bg         | `#0f172a` surface · `#020617` canvas (slate-900/950 — cool dark, not pure black)                                                                                                               |
| Primary/Action  | Indigo/slate-blue — `#4f46e5` primary · `#4338ca` hover/active · same hue in both modes                                                                                                        |
| Semantic states | Success `#10b981` (emerald) · Warning `#f59e0b` (amber) · Destructive `#ef4444` (crimson) — used sparingly, only for real signal                                                               |
| Borders         | Thin, low-contrast — `#e2e8f0` light · `#1e293b` dark. Dividers, not boxes.                                                                                                                    |
| Typography      | One family only — Inter or Geist Sans, no separate display font. Strict scale: large tabular numbers for metrics, bold labels for card headers, muted small text for metadata.                 |
| Quality target  | Enterprise Minimalist — every screen reads clearly at a glance, nothing competes with the data                                                                                                 |
| Layout patterns | Two canonical page anatomies — Dashboard Page Pattern (overview/report pages) and Collection View Pattern (entity lists, Notion-style, borderless + inline-editable) — see Section 7 for both. |
| Border radius   | Subtle (4–8px), never rounded-full on blocks                                                                                                                                                   |
| Motion          | Restrained and functional — skeleton loading, hover/focus states, smooth number transitions. No entrance choreography, nothing decorative.                                                     |
| Density         | Medium-high — dashboards reward information density over whitespace, but never cramped                                                                                                         |
| Implementation  | CSS variables per theme, consumed via Tailwind `dark:` classes                                                                                                                                 |

### Infrastructure

| Concern    | Choice                                               |
| ---------- | ---------------------------------------------------- |
| Container  | Docker + Docker Compose                              |
| CI         | GitHub Actions                                       |
| Deployment | VPS via Docker Compose (not started — see Section 9) |
| Secrets    | GitHub Secrets + `.env` files (never committed)      |

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
    capture/                  ← lead auto-capture (r11)
      apikeys/                ← org API keys (generate, validate, revoke)
      public/                 ← /pub/leads public web-form capture endpoint
      email/                  ← inbound email webhook → lead + per-org addresses
      social/                 ← social lead-ad webhooks + integrations
      visitors/               ← website visitor identify + pageview log
    platform/
      contacts/               ← shared contacts + companies (used by CRM and future modules)
      engagement/              ← shared notes, tasks, activities, emails, timeline
    crm/
      leads/                  ← CRM lead management (now with capture fields + dedup + round-robin)
      pipeline/               ← pipeline and stages
      deals/                  ← deal CRUD + board view
      reports/                ← CRM analytics endpoints (incl. agenda)
      templates/              ← email/note snippet templates (post-r10)
      settings/               ← per-org CRM settings: lead routing round-robin (post-r10)
    middleware/               ← auth, business context, logger, rate limit, permission, recover, apikey
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
    logger/                   ← slog helpers
    pagination/               ← pagination helpers
```

### Layer Rules

- **Handler**: HTTP only. Reads request, calls service, writes response. No SQL, no business logic.
- **Service**: Business logic only. No HTTP types, no SQL queries. Takes context.
- **Repository**: SQL only. No business logic. Uses parameterized queries always.
- **Middleware**: Request-level cross-cutting concerns (auth check, rate limit, permission check, API key check).
- **Pkg**: Stateless utilities with zero domain knowledge (jwt, password, token, response).

Never put business logic in handlers. Never put SQL in services. Never put HTTP types in services.

### Middleware Chains

Org-scoped JWT routes (unchanged):

```
RequireAuth → RequireOrganizationParam(:orgId) → RequirePermission(perm)
```

Public capture routes (r11):

```
RequireAPIKey(apiKeySvc, scope) → handler
```

`RequireAPIKey` reads `X-API-Key`, SHA-256 hashes it, looks up `org_api_keys`, checks `is_active` + scope + (post-Fix-Pass-A) expiry + optional per-key origin whitelist, then sets `org_id` and `user_id` (= key creator) in `c.Locals`. It is the API-key parallel of `RequireAuth`.

Webhook routes (`/pub/email/webhook`, `/pub/social/:platform/webhook`) currently run with **no auth middleware** — provider signature verification is Fix Pass B. Until that lands these endpoints must not be exposed to the public internet.

### Key Patterns

**PermissionFunc pattern** — avoids import cycles between route files and middleware:

```go
permFn := func(perm string) fiber.Handler {
    return middleware.RequirePermission(authzSvc, perm)
}
```

**Opaque refresh tokens** — raw token is returned once to handler (for httpOnly cookie), only the hash is stored in DB. Token is never logged or included in JSON body.

**Raw API keys** — same show-once discipline: `GenerateKey` returns the raw `bs_live_…` value exactly once in `CreateKeyResponse`; only the SHA-256 hash and a 16-char display prefix are persisted. `KeyHash` carries `json:"-"`.

**Webhook processing pattern (r11)** — inbound webhooks log every payload to a `*_logs` table (`raw_payload` JSONB, `processed` flag, `error_message`), and return 200 even on business failures so providers don't retry-storm. Failures are diagnosed from the log table, not from webhook response codes.

**JWT claims** include: `user_id`, `business_id` (org context), `email`, `role`. Business context is set when user selects/switches org.

**Tenant isolation** — `RequireOrganizationParam` compares `:orgId` in URL against `business_id` in JWT. Cross-org access is blocked at middleware, not just at repository level. Capture endpoints resolve org from the API key / inbound address / page_id instead, and every capture-side query is still `org_id`-scoped.

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

**MOBILE [🔵 ACTIVE — contract defined, handlers not yet written]:**

Routes to add in `backend/internal/auth/routes.go`, wired into both `RegisterRoutes` and
`RegisterRoutesWithRateLimit` the same way the existing `/sign-up`, `/sign-in` aliases are —
rate-limited on the three public ones:

POST /api/v1/auth/mobile/signup
POST /api/v1/auth/mobile/login
POST /api/v1/auth/mobile/logout
POST /api/v1/auth/mobile/refresh

Same `Service`/`Repository` as web auth, zero new business logic — only new handler methods
(`MobileLogin`, `MobileRefresh`, etc.) that return the refresh token in the JSON body instead of
setting the `bsaas_refresh` httpOnly cookie, and read it back from the request body on
`mobile/refresh` / `mobile/logout` instead of the cookie jar. `logout-all`, `password-reset/*`,
and `me` stay shared as-is — none of them touch tokens, no mobile variant needed.

Resolved (was an open assumption in the r10 draft): `Signup` does not auto-authenticate.
`frontend/src/app/(auth)/signup/page.tsx` creates the user then calls `login()` separately to
bootstrap the session. `MobileSignup` should do the same — create the user, return it with no
tokens, let the client call `mobile/login` right after. Keeps both clients on identical semantics
instead of giving mobile a second, divergent signup path.

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
POST  /api/v1/organizations/:orgId/members/:memberId/reset-password

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
Key permissions: `members.view` · `members.update` · `members.invite` · `members.remove` · `members.password_reset` · `roles.view` · `roles.create` · `roles.update` · `roles.delete` · `roles.clone` · `roles.permissions.update` · `members.permissions.view` · `members.permissions.update`

Guards: `UpdateMemberPermissions` rejects self-targeting (`ErrCannotChangeOwnPermissions`) and owner-targeting (`ErrCannotModifyOwner`). `members.password_reset` (owner/admin) revokes the target's sessions and is audit-logged. `organizations.max_seats` (nullable, migration 00052) enforced in `InviteMember` → 409 `ErrSeatLimitReached`; no admin UI, direct DB write only.

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

---

### PLATFORM — CONTACTS [✅ DONE]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/contacts
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/contacts/:contactId

GET/POST         /api/v1/organizations/:orgId/crm/companies
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/companies/:companyId
GET              /api/v1/organizations/:orgId/crm/companies/:companyId/contacts
```

Permissions: `crm.contacts.*` · `crm.companies.*`

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
Notes are tagged with module `"crm"` — every writer must use that exact tag or the record is invisible to the timeline (the capture dedup path currently violates this; Fix Pass A).

---

### CRM — LEADS [✅ DONE — extended r11]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/leads
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/leads/:leadId
POST             /api/v1/organizations/:orgId/crm/leads/:leadId/convert
```

Permissions: `crm.leads.view` · `crm.leads.create` · `crm.leads.update` · `crm.leads.delete` · `crm.leads.convert`
Statuses: `new` · `contacted` · `qualified` · `unqualified` · `converted`

**Post-r10 extensions:**

- **Round-robin auto-assignment** (00056 + `crm/settings`): when `crm_settings.lead_routing_enabled` and `round_robin_assignees` is non-empty, `CreateLead` rotates `owner_id` via `GetLastAssignedLeadOwner`.
- **Capture fields** (00058): `custom_fields JSONB` · `capture_source TEXT` · `capture_metadata JSONB` on `crm_leads`, present in model/scan/insert.
- **Email dedup**: `CreateLead` checks `FindLeadByEmail` first; on match, appends an engagement note to the existing lead and returns it instead of inserting. ⚠️ Currently applies to ALL creates including manual UI creates, match is case-sensitive, and the note uses the wrong module tag — all three are Fix Pass A items (scope to `CaptureSource != nil`, `LOWER()` match, module `"crm"`).

---

### CRM — PIPELINE & STAGES [✅ DONE]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/pipelines
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/pipelines/:pipelineId
GET/POST         /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages
POST             /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/reorder
PATCH/DELETE     /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/:stageId
```

Permissions: `crm.deals.*`

---

### CRM — DEALS [✅ DONE]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/deals
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/deals/:dealId
POST             /api/v1/organizations/:orgId/crm/deals/:dealId/move
POST             /api/v1/organizations/:orgId/crm/deals/:dealId/won
POST             /api/v1/organizations/:orgId/crm/deals/:dealId/lost
GET              /api/v1/organizations/:orgId/crm/deals/:dealId/board
```

Permissions: `crm.deals.*` · `crm.deals.move_stage`

---

### CRM — TEMPLATES [✅ DONE — post-r10, folded into doc r11]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/templates
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/templates/:templateId
```

Permissions: `crm.templates.view` · `crm.templates.create` · `crm.templates.update` · `crm.templates.delete` (seeded 00055)
Table: `crm_templates` (00054) — `type IN ('email','note')`, optional `subject`, `body`. Email/note snippets for quick insertion by reps.

---

### CRM — SETTINGS [✅ DONE — post-r10, folded into doc r11]

Routes:

```
GET   /api/v1/organizations/:orgId/crm/settings     ← settings.view
PATCH /api/v1/organizations/:orgId/crm/settings     ← settings.update
```

Table: `crm_settings` (00056) — `lead_routing_enabled BOOLEAN`, `round_robin_assignees JSONB`. Backs the lead round-robin (see CRM — LEADS). Uses the generic `settings.*` permissions, not a `crm.settings.*` pair — acceptable for now, revisit if CRM settings grow.

---

### CRM — REPORTS [✅ DONE — agenda added post-r10]

Routes:

```
GET /api/v1/organizations/:orgId/crm/reports/overview
GET /api/v1/organizations/:orgId/crm/reports/summary
GET /api/v1/organizations/:orgId/crm/reports/deals/by-stage
GET /api/v1/organizations/:orgId/crm/reports/deals/by-owner
GET /api/v1/organizations/:orgId/crm/reports/leads/by-source
GET /api/v1/organizations/:orgId/crm/reports/tasks/overdue
GET /api/v1/organizations/:orgId/crm/reports/activities/stats
GET /api/v1/organizations/:orgId/crm/reports/agenda          ← post-r10, backs /crm/agenda page
```

Permissions: `crm.reports.view`

---

### CAPTURE [🔵 ACTIVE — backend written r11, Fix Pass A/B pending, frontend not started]

New top-level module: `internal/capture/`. All five sources wired in `main.go`; migrations 00057–00064.

**Auth-gated management routes (JWT + org match):**

```
GET/POST /api/v1/organizations/:orgId/capture/apikeys           ← capture.apikeys.view / .create
DELETE   /api/v1/organizations/:orgId/capture/apikeys/:keyId    ← capture.apikeys.delete
GET/POST /api/v1/organizations/:orgId/capture/email             ← ⚠️ uses settings.view; switch to capture.email.manage (Fix A)
DELETE   /api/v1/organizations/:orgId/capture/email/:id
GET/POST /api/v1/organizations/:orgId/capture/social            ← ⚠️ uses settings.view; switch to capture.social.manage (Fix A)
DELETE   /api/v1/organizations/:orgId/capture/social/:id
GET      /api/v1/organizations/:orgId/capture/visitors          ← ⚠️ uses nonexistent crm.view → 403s for everyone; switch to capture.visitors.view (Fix A)
```

**Public routes:**

```
POST /api/v1/pub/leads                       ← RequireAPIKey, scope capture:leads — WORKS (web form / chat)
POST /api/v1/pub/visitors/identify           ← RequireAPIKey, scope capture:visitors
POST /api/v1/pub/email/webhook               ← ⚠️ NO verification (Fix B) — do not expose publicly yet
GET/POST /api/v1/pub/social/:platform/webhook ← ⚠️ NO signature check; GET accepts any verify_token (Fix B)
```

**API key contract:** raw key `bs_live_<64 hex>`, shown exactly once at creation; SHA-256 hash + 16-char prefix stored; scopes `capture:leads` (+ `capture:visitors`, constant pending); optional per-key `allowed_origins` and `expires_at`.

**Permissions seeded (00062):** `capture.apikeys.view/create/delete` · `capture.email.manage` · `capture.social.manage` · `capture.visitors.view` — granted to owner/admin. Only the apikeys three are actually referenced by routes today (see warnings above).

**Deliberate scope reductions (documented, not bugs):**

- Social is a **webhook skeleton**, not a real Facebook integration — no Graph API `leadgen_id` fetch, no OAuth connect, no field-mapping engine; it maps flat payload fields, which real FB webhooks don't send. Real integration is tracked separately (Section 9).
- Visitors is **manual identify** (Segment-style: `traits.email/name/company` → lead), not IP→company enrichment. No IPinfo, no Redis queue, no background worker.
- No hosted form / JS embed endpoint yet (was Step 4 second half) — capture frontend work.

**Known open items (r11 audit — this list IS the current work):**

_Fix Pass A — feature-breaking + correctness:_

1. `CreateLead(ctx, orgID, "", …)` in email/social/visitors → `created_by` gets empty string → invalid-UUID error on every system-generated lead. Fix: migration 00065 drops NOT NULL on `crm_leads.created_by`; model `CreatedBy *string`; pass nil for system captures; UI renders null as "System".
2. `social/repository.go` INSERTs into `access_token_enc`; column is `access_token` (00060) → connect always fails. Fix repo SQL.
3. Route permissions → seeded `capture.*` keys (see warnings above); adds `capture:visitors` scope constant + scope/name validation in `GenerateKey`.
4. `ValidateKey` never checks `expires_at` (`ErrKeyExpired` sentinel unused). Add the check.
5. Dedup: scope to `req.CaptureSource != nil` only; `LOWER(email)` match; note module `"crm"`; skip note when `userID == ""`; don't swallow the note error silently.
6. Delete leftover AI-conversation comments in `public/handler.go` and `leads/service.go`.
7. Minor: `social` model `AccessToken` json tag → `"-"`; `org_api_keys.created_by` ON DELETE CASCADE → RESTRICT; UNIQUE `(org_id, session_id)` on `website_visitors`; UNIQUE on `org_api_keys.key_hash`.

_Fix Pass B — security, required before any public exposure:_ 8. Inbound email webhook HMAC verification (`WEBHOOK_EMAIL_SECRET`). 9. Facebook `X-Hub-Signature-256` verification (`FACEBOOK_APP_SECRET`) + real `hub.verify_token` comparison (`FACEBOOK_VERIFY_TOKEN`). 10. Redis rate limiting on every `/pub/*` route (new `NewPublicCaptureRateLimit` constructor; per-key where a key exists, per-IP on webhooks).

---

### HRM MODULE [✅ DONE — verified r9; dynamic statuses added post-r10]

All routes live under `/api/v1/organizations/:orgId/hrm/...`, permission-gated (`hrm.<submodule>.<action>`), 25 sub-modules, 201 routes (r9 count) **plus 4 employee-status routes added post-r10** (205 total). This entry summarizes; per-route detail belongs in a dedicated `docs/modules/hrm.md`.

**Database:** 41 tables. 40 verified in r9 (migrations `00020`–`00050`, of which `00048` is unrelated CRM seed data) + `hrm_employee_statuses` (00053).

**Group A — Setup/Config** (`departments, positions, salary, approvals, warningtypes, doctemplates, shifts, holidays, contracts`): 71 routes. Salary formula engine via `expr-lang/expr`. Approvals still missing an approval-instance list endpoint (open since r9).

**Group B — Lifecycle** (`employees, promotions, transfers, resignations, terminations`): 37 routes + 4 new:

```
GET/POST     /organizations/:orgId/hrm/employee-statuses        ← hrm.employees.view / hrm.employees.manage_setup
PATCH/DELETE /organizations/:orgId/hrm/employee-statuses/:id    ← hrm.employees.manage_setup
```

Dynamic statuses (00053): per-org status list with category (`active/inactive/on_leave/terminated`) + color token; `hrm_employees.status_id` FK; migration auto-seeded defaults and mapped existing employees.

**Group C — Disciplinary** (`warnings, complaints, employeedocs, acknowledgements`): 34 routes. Cross-module writes via `ON CONFLICT DO NOTHING` + direct `pgPool.Exec` to avoid import cycles.

**Group D — Time & Compensation** (`attendance, payslips`): 19 routes. Multi-punch attendance, `ComputeSlab` progressive tax, immutable finalized payslips.

**Group E — Recognition & Communication** (`awards, announcements, calendar, milestones`): 25 routes. Nightly crons for milestones/absences.

**Reports:** 3 routes.

**Approval chain wiring:** callback registry on the approvals service; promotions, transfers, terminations, warnings, awards each register `HandleApprovalDecision` in `main.go`.

**Known open item:** missing approval-instance list endpoint (carried from r9).

---

### AUDIT [✅ DONE]

Internal only. Append-only log for security-sensitive events. No public API endpoints.

---

## 6. DATABASE

### Conventions

- UUID primary keys everywhere (`id` = internal, `public_id` = API-safe where exposed)
- `created_at`, `updated_at` on every table
- Indexes on lookup-heavy columns (org_id, user_id, email, status)
- All schema changes via Goose migration files — never manually
- Transactions for multi-step operations (org creation, membership changes, lead conversion, approval decisions)
- Audit logs and webhook logs are append-only

### Migration Count: 64

Files live in `backend/internal/migrations/`. Run via `goose` or `make migrate`.
r10 ended at 52. Post-r10: `00053` dynamic employee statuses · `00054/00055` CRM templates + permissions · `00056` CRM settings · `00057`–`00064` capture (api keys, lead capture fields, inbound emails, social integrations, website visitors + pageviews, capture permissions, inbound email logs, social lead logs). Next: `00065` (created_by nullable — Fix Pass A).

### Key Tables (74 total)

**Core / auth / org (14):**
`users` · `organizations` · `organization_members` · `organization_invitations` · `permissions` · `roles` · `auth_accounts` · `sessions` · `login_events` · `verification_tokens` · `subscriptions` · `organization_usage` · `audit_logs` · `tasks`

**Platform (6):**
`platform_contacts` · `platform_companies` · `platform_notes` · `platform_tasks` · `platform_activities` · `platform_email_logs`

**CRM (6):**
`crm_leads` (+ `custom_fields`, `capture_source`, `capture_metadata` since 00058) · `crm_pipelines` · `crm_pipeline_stages` · `crm_deals` · `crm_templates` · `crm_settings`

**Capture (7):**
`org_api_keys` · `org_inbound_emails` · `social_integrations` · `website_visitors` · `visitor_pageviews` · `inbound_email_logs` · `social_lead_logs`

**HRM (41):**
Group A (19): `hrm_departments` · `hrm_positions` · `hrm_salary_components` · `hrm_salary_structures` · `hrm_salary_structure_components` · `hrm_approval_templates` · `hrm_approval_template_levels` · `hrm_approval_instances` · `hrm_approval_decisions` · `hrm_warning_types` · `hrm_warning_escalation_rules` · `hrm_document_templates` · `hrm_document_bulk_sends` · `hrm_shifts` · `hrm_work_schedule_assignments` · `hrm_holiday_calendars` · `hrm_holidays` · `hrm_calendar_assignments` · `hrm_employee_contracts`
Group B (6): `hrm_employees` · `hrm_promotions` · `hrm_transfers` · `hrm_resignations` · `hrm_terminations` · `hrm_employee_statuses`
Group C (4): `hrm_employee_warnings` · `hrm_complaints` · `hrm_employee_documents` · `hrm_acknowledgements`
Group D (6): `hrm_attendance_periods` · `hrm_attendance_records` · `hrm_employee_salary_records` · `hrm_payslip_runs` · `hrm_payslips` · `hrm_payslip_lines`
Group E (4): `hrm_awards` · `hrm_announcements` · `hrm_calendar_events` · `hrm_employee_milestones`
Leave (2): `hrm_leave_types` · `hrm_leave_requests`

---

## 7. FRONTEND ARCHITECTURE

### Folder Structure

```
frontend/
  src/
    app/
      (auth)/                 ← login, signup
      (onboarding)/           ← create-organization, select-organization
      (dashboard)/
        [orgId]/
          crm/
            leads/  pipeline/  reports/  agenda/
            setup/routing/  setup/templates/
          contacts/  companies/  companies/[companyId]/
          tasks/
          hrm/                ← all HRM pages incl. setup/statuses
          settings/ (members, roles, profile)
          security/sessions/
    components/
      ui/  layout/  crm/  tasks/  members/  roles/  settings/  hrm/  providers/
    contexts/DrawerContext.tsx
    lib/
      api.ts  auth.ts  constants.ts  token.ts  jwt.ts  queryKeys.ts
      crm/ (leads, contacts, companies, deals, pipelines, reports, settings, templates)
      hrm/ (…)
      members.ts  roles.ts  org.ts  profile.ts  security.ts  tasks.ts  permissionGroups.ts
    stores/ (authStore, permissionStore, uiStore)
    hooks/ (useIsMobile, …)
    types/ (api, auth, crm, hrm, org, rbac, task)
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
  permissions: string[];
  setPermissions: (perms: string[]) => void;
  can: (perm: string) => boolean;
  canAny: (perms: string[]) => boolean;
  reset: () => void;
}
```

**`uiStore`** — sidebar and theme, nothing else.

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

Components use `dark:` Tailwind classes — never inline theme checks in JS. `uiStore.theme` drives `setTheme()` and persists to localStorage (safe to persist).

### Organization Context Flow

1. After login, user lands on org selection screen if no org is active
2. User selects org → POST `/api/v1/organizations/:id/switch` → receive new `access_token` with `business_id` claim
3. Store new token, store `orgId` in context and URL (`/[orgId]/...`)
4. All subsequent API calls use the org-scoped URL pattern

### Permission Pattern (frontend)

```tsx
const { can } = usePermission(); // reads from permissionStore — no API call

{
  can("tasks.create") && <Button onClick={openCreateModal}>New Task</Button>;
}
```

`permissionStore` is populated after org switch by `GET /api/v1/members/me`. Backend enforces on every request — frontend gates are UX only.

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
// Request interceptor → attach Authorization: Bearer <token>
// Response interceptor → on 401: /auth/refresh → setToken(new) → retry
```

**Why not Zustand for the token:** Zustand state can be accidentally persisted via the `persist` middleware, putting the token in `localStorage`. A plain module variable lives only in memory and is cleared on page refresh (triggering the silent refresh flow).

### Component Quality Standard

- Fully themed — light and dark both look intentional, not just inverted
- Indigo accent on interactive elements in both modes — actions and active states, not decoration
- One typeface family with a strict scale; no separate display font
- Clear action hierarchy: primary filled, secondary outlined, tertiary text links — never more than one primary action visible at once
- Skeleton loading states on every data-fetching component
- Keyboard accessible: visible focus rings, tab order matches visual order, escape closes modals
- WCAG AA contrast, explicit ARIA landmarks on layout regions
- Responsive (works on 1280px minimum, designed for 1440px)
- No placeholder "coming soon" states — if a page is in scope, it is built properly
- Error states and empty states are designed, not default browser behavior

### Dashboard Page Pattern

Default anatomy for data-overview pages (CRM Reports, HRM Reports, org dashboard home):

1. **KPI strip** — 3–4 metric cards: absolute value, small sparkline, percentage change with directional arrow
2. **Data visualization** — split-pane: primary time-series chart + secondary category bar chart
3. **Data table** — full-width: search, multi-select filters, export; hover states, status badges, `⋯` context menu

List views that aren't overview pages (Leads, Tasks, Members) skip the KPI/chart layers and go straight to the table.

### Collection View Pattern (Notion-style)

Default anatomy for single-entity collection pages where each record is a lightweight,
frequently-edited item — first example is Tasks. Distinct from the Dashboard Page Pattern:
no KPI strip, no charts, this is a working list, not a report.

1. **Toolbar** — page title + record count top-left; view switcher (e.g. Grouped/List) +
   primary "New" action top-right.
2. **Grouped view (default)** — records grouped by their primary status/category field into
   collapsible sections; each header shows a status dot, label, count, and a hairline divider —
   no bordered card around the section.
3. **List view (alternate)** — flat list with filter tabs above it (same status field), for
   scanning everything in one scroll.
4. **Row = borderless, chrome-less** — no grid lines, no card border; hover background is the
   only structural cue between rows. Primary fields (title, status, date, done-toggle) are
   inline-editable directly on the row — no separate edit mode. A drawer is reserved for
   secondary fields (e.g. description) via an explicit "Edit" action.
5. **Quick-add** — records are created inline at the bottom of a list/group ("+ New …"), not via
   modal or drawer. Enter commits and keeps the input open for consecutive entries; Escape/blur
   cancels.
6. **Status representation** — solid-tint pill (background tint + matching text color, no
   border), not an outlined badge.
7. **Loading state** — skeleton rows matching the row's real layout, never a spinner+text.

Applies to: Tasks (shipped). Candidate for CRM Leads, Members, and other simple single-entity
lists when they're next revisited — this does not retroactively obligate rewriting those pages.

---

## 8. FRONTEND MODULE REGISTRY

### AUTH PAGES [✅ DONE]

`/login` · `/signup` — own layout, no sidebar. (Forgot/reset-password pages: backend routes exist; page files not present in the current tree — build when email sending lands, see Section 9.)

### ONBOARDING [✅ DONE]

`/create-organization` · `/select-organization`

### DASHBOARD SHELL [✅ DONE]

Sidebar, Topbar, org switcher, drawer system (`DrawerContext` + `ui/Drawer`), org context in URL.

### TASKS [✅ DONE — redesigned to the Collection View Pattern]

`/[orgId]/tasks` — Grouped (collapsible, by status) and List (flat, filter tabs) view modes,
inline quick-add, full inline editing (title, status, due date, complete-toggle); drawer
reserved for secondary-field edits, permission-gated actions.

### SETTINGS — MEMBERS [✅ DONE]

`/[orgId]/settings/members` — members list, invite, role assignment, per-member action menu (reset password, manage/view permissions, remove), invitation resend/revoke. Menu hidden for own row and owner's row, matching backend guards.

### SETTINGS — ROLES & PERMISSIONS [✅ DONE]

`/[orgId]/settings/roles` — role list with System/Custom badges, create (via `PermissionForm` create mode), edit, clone, delete. Shared `lib/permissionGroups.ts` categorization (includes all HRM resources). ⚠️ `capture.*` permission group not yet added to `permissionGroups.ts` — do this with the capture frontend, or the six capture permissions will be invisible in the picker (same class of bug as the r10 HRM fix).

### PROFILE & SECURITY [✅ DONE]

`/[orgId]/settings/profile` (avatar crop modal included) · `/[orgId]/security/sessions`

### CRM — LEADS [✅ DONE]

`/[orgId]/crm/leads` — list, filters, `LeadForm`, `ConvertForm` (contact + deal creation).

### CRM — CONTACTS & COMPANIES [✅ DONE]

`/[orgId]/contacts` · `/[orgId]/companies` · `/[orgId]/companies/:companyId` — note the frontend URLs are NOT nested under `/crm/`; the backend API routes are (`/organizations/:orgId/crm/...`). Known and fine.

### CRM — PIPELINE & DEALS [✅ DONE]

`/[orgId]/crm/pipeline` — kanban, drag-to-move, `DealForm`, won/lost.

### CRM — REPORTS [✅ DONE]

`/[orgId]/crm/reports` — overview dashboard per the Dashboard Page Pattern.

### CRM — AGENDA [✅ DONE — post-r10, folded into doc r11]

`/[orgId]/crm/agenda` — "today view" over `GET /crm/reports/agenda` (TanStack Query, permission-gated).

### CRM — SETUP [✅ DONE — post-r10, folded into doc r11]

`/[orgId]/crm/setup/routing` — lead round-robin settings (crm_settings)
`/[orgId]/crm/setup/templates` — template CRUD (`TemplateForm`)

### CRM — CAPTURE [⚪ NOT STARTED — backend exists, zero frontend]

Planned (Task.md Steps 8–10): `/[orgId]/crm/setup/capture` (API key list + create drawer with show-once raw key + embed code panel; email + social tabs) and `/[orgId]/crm/capture/visitors` (visitor list). New `lib/crm/capture.ts`, `types/capture.ts`. Must also add the `capture.*` group to `lib/permissionGroups.ts`.

### HRM [✅ DONE — statuses setup added post-r10]

All under `/[orgId]/hrm/...`: departments, positions, employees, leave, attendance, payroll, lifecycle, warnings, compliance, recognition, reports, and setup/{approvals, document-templates, holidays, salary, shifts, warning-types, **statuses**}. `ApprovalInstanceView` drawer wired into Warnings, Recognition, and Lifecycle pages.

---

## 9. UNBUILT MODULE REGISTRY

### MOBILE APP [🔵 ACTIVE — architecture restored, backend contract defined, zero code written]

Expo + React Native. Full architecture restored from the r10 archive: Section 14 (Mobile
Architecture) and Section 15 (Mobile Module Registry) — decided, not redesigned. Backend contract
is a real entry now: Section 5 → AUTH → MOBILE. Suggested starting point: AUTH SCREENS
(Section 15) paired with the AUTH — MOBILE backend routes, mirroring how every other module here
starts with auth before anything else.

---

### CRM — ADVANCED FUNCTIONALITY PASS [🔵 ACTIVE — capture in flight]

Mridha's wish list, triaged by buildability. **In flight now:** lead auto-capture (Section 5 → CAPTURE: fix pass + frontend). **Shipped from this list:** lead routing round-robin, templates & snippets, agenda/smart-task view.

**Contained — no new infrastructure or one well-understood integration:**

- Activity metrics reporting (calls/meetings/deals-closed per rep)
- Sales forecasting (weighted pipeline by historical win rate per stage)
- Data enrichment on company domain (Clearbit or Apollo — real per-lookup cost)
- Trigger-based actions — small hardcoded set first ("on Closed Won, do X"), not a rule engine

**Needs new infrastructure first, or a real ongoing vendor cost:**

- Reminders / SLA alerts — needs a background job scheduler and a notification delivery path; neither exists
- Real Facebook/LinkedIn lead-ad integration (Graph API fetch, OAuth connect, field-mapping UI) — upgrades the current webhook skeleton
- Visitor IP→company enrichment (IPinfo/Clearbit + Redis queue + worker) — upgrades current manual identify
- Meeting scheduling via Calendly — mostly a webhook integration
- Sales velocity (time-in-stage) — needs a stage-transition history table first (`crm_deals` only stores current `stage_id`)
- SMS via Twilio — contained but new vendor + per-message cost

**Major, multi-week-plus:**

- Two-way email sync (Gmail/Outlook) — reconcile with Email Sending below, don't build twice
- Telephony / call recording / transcription
- Automated sequences/cadences — mini workflow engine; depends on email sync for reply detection
- CPQ + e-signature
- Invoicing/accounting integration — decide external-integration vs future in-house Accounting module
- Ticketing/Helpdesk — its own product, wasn't on the original module list

**Likely not worth pursuing:** LinkedIn message sync (no official API; scraping is fragile and ToS-risky).

---

### MFA / 2FA [⚪ NOT STARTED]

TOTP enrollment + verification, backup codes, per-org enforcement policy. Lives in `internal/auth/`; needs a secrets table and a frontend enroll/verify flow.

---

### SOCIAL LOGIN / SSO [⚪ NOT STARTED]

Google / Microsoft / GitHub OAuth flows. `POST /api/v1/auth/oauth/sync` already exists as the backend hook; this item is the provider-specific flow wiring.

---

### EMAIL SENDING [⚪ NOT STARTED]

Transactional provider (SES / Postmark / Resend) for verification, invites, password reset — currently token-only. Also unblocks the forgot/reset-password frontend pages. Distinct from capture's inbound email parsing, but if Postmark is chosen, one account can serve both inbound and outbound — decide together.

---

### RESOURCE-LEVEL PERMISSIONS [⚪ NOT STARTED]

Per-record access control ("only deals they own") beyond module/action RBAC. Touches every repository's query layer — needs its own ADR when it starts.

---

### BILLING & SUBSCRIPTION MANAGEMENT [⚪ NOT STARTED]

Payment provider, plan tiers, usage limits, invoices, billing UI. `organization_usage` and `organizations.max_seats` already exist as head starts.

---

### HRM FUNCTIONALITY PASS [⚪ NOT STARTED]

Post-completion polish pass over the shipped HRM module, same spirit as the CRM pass. Includes the known open item: approval-instance list endpoint.

---

### PROJECT MANAGEMENT MODULE [⚪ NOT STARTED]

Full projects module (milestones, dependencies) — broader than the generic `task` module, which stays a simple RBAC-testing CRUD.

---

### E-COMMERCE ADMIN MODULE [⚪ NOT STARTED]

Per the original long-term vision.

---

### ACCOUNTING MODULE [⚪ NOT STARTED]

Per the original long-term vision.

---

### ERP MODULE [⚪ NOT STARTED — scope undefined]

Umbrella term; may end up being HRM + Accounting + Projects + Inventory combined rather than a standalone build. Real scoping needs those three to exist first, since that's what "ERP" would be built out of.

---

### FULL PRODUCTION DEPLOYMENT [⚪ NOT STARTED]

Confirmed not started (r9 audit; unchanged): no Sentry SDK, no Caddyfile, `deploy.yml` is a stub. Scope when picked up: `docker-compose.prod.yml`, TLS (Caddy), production env/secrets, Sentry, backups, VPS + DNS. **Hard prerequisite:** Capture Fix Pass B must be complete before any deployment — two webhook endpoints are currently unauthenticated.

---

### MOBILE APP [⚪ NOT STARTED]

Expo + React Native. Architecture was fully designed (folder structure, SecureStore token strategy, Expo Router guards, theming, EAS pipeline) and is archived in this doc's r10 revision (git history) — restore those sections when this item starts. Zero code exists. Backend contract sketch lives in Section 5 → AUTH → Mobile variants.

---

## 10. CORE SECURITY STANDARDS

These apply to every line of code in this project. Never compromise on these.

**Backend:**

- Never store plaintext passwords — bcrypt always
- Never store raw refresh/reset/email tokens — hash first, store hash
- Never store raw API keys — SHA-256 hash + display prefix only; raw value returned exactly once at creation
- Never log passwords, tokens, API keys, secrets, or any sensitive data
- Parameterized SQL only — never string-concatenate SQL
- Validate all request bodies at handler layer
- Sanitize and normalize email (lowercase, trim)
- Return generic errors for login and auth operations — never reveal why login failed
- Log detailed errors server-side only
- Secure CORS (explicit origins, no wildcard in production)
- `HttpOnly: true` on refresh cookie — always, not configurable
- Transactions for multi-step write operations
- Audit log for: login attempts, logout, password reset, role changes, permission changes
- **Webhook endpoints must verify provider signatures (HMAC / X-Hub-Signature-256) before processing — an unverified webhook is an unauthenticated write endpoint** (Fix Pass B closes the current gap)
- Every public (`/pub/*`) endpoint gets Redis-backed rate limiting — per API key where one exists, per IP otherwise
- Public endpoints never leak internal state: no validation detail in error responses, no internal IDs beyond `public_id`
- Third-party access tokens (social integrations) are encrypted at rest once real OAuth lands — never serialized in JSON responses

**Frontend:**

- Access token in `lib/token.ts` module variable only — never Zustand, never localStorage, never sessionStorage
- Refresh token is httpOnly cookie — frontend never reads it, never stores it
- Raw API keys shown once, never stored client-side beyond the creation modal
- Never add `persist` middleware to `authStore` or `permissionStore`
- `withCredentials: true` on all API requests
- Never show stack traces or backend error internals in UI
- Permission checks are UX only — backend enforces on every request
- No sensitive data in URL query params

---

## 11. DEVELOPMENT WORKFLOW

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
- **Review AI-assisted output before committing** — no stream-of-consciousness comments, no "task says" references, no unresolved "Wait, let me check…" left in source. If a comment asks a question, answer it or delete it. (Added r11 after the capture audit found exactly this, hiding a feature-breaking bug.)

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

## 12. RESPONSE FORMAT

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

## 13. VERSION AWARENESS

Before writing code that depends on any of these, check for version-specific API differences:

| Package / Framework | Version in use                                                 |
| ------------------- | -------------------------------------------------------------- |
| Go                  | 1.25                                                           |
| Fiber               | v3.2.0                                                         |
| pgx                 | v5.6.0                                                         |
| go-redis            | v9.19.0                                                        |
| golang-jwt/jwt      | v5.3.1                                                         |
| expr-lang/expr      | v1.17.8                                                        |
| Next.js             | 16.2.9 (check if unsure)                                       |
| Tailwind CSS        | v4                                                             |
| React               | latest stable                                                  |
| Zustand             | v5                                                             |
| next-themes         | latest stable                                                  |
| React Hook Form     | v7                                                             |
| TanStack Query      | v5                                                             |
| Axios               | v1                                                             |
| Expo SDK            | 57 (57.0.7 — confirm exact patch at scaffold time, moves fast) |
| Expo Router         | bundled with Expo SDK 57                                       |
| React Native        | 0.86 (bundled with SDK 57) — check package.json after scaffold |
| expo-secure-store   | latest compatible with SDK 57                                  |

`create-expo-app@latest` without `--template default@sdk-57` currently defaults to SDK 54 during
the transition window — pass the flag explicitly.

Do not use Fiber v2 API patterns with Fiber v3.

Tailwind v4 syntax — always use these, never the v3 equivalents:

- CSS entry point is `@import "tailwindcss";` — never the three `@tailwind base; @tailwind components; @tailwind utilities;` directives
- Theme config lives in CSS via `@theme { --color-x: ...; }` (this is already how `globals.css` is set up, Section 7) — never add a `tailwind.config.js` expecting `theme.extend` to work by default; v4 doesn't require one
- Gradients are `bg-linear-to-r`, `bg-linear-45`, etc. — not `bg-gradient-to-r`
- Referencing a CSS variable in an arbitrary value uses parentheses — `bg-(--brand-color)`, not `bg-[--brand-color]`. Square brackets stay for literal arbitrary values like `bg-[#4f46e5]`.
- Default border color is `gray-200`, not `currentColor` — don't assume a bare `border` inherits text color like it did in v3
- Custom utility classes are defined with `@utility` directly in CSS, not a JS plugin's `addUtilities()` callback

---

## 14. MOBILE ARCHITECTURE

Restored from the r10 archive, r14 — decided, not redesigned. Expo + React Native client. Ports
the web frontend's architecture (Section 7) wherever the platform allows; diverges only where
native constraints force it — token storage is the main one.

### Folder Structure

mobile/
app/ ← Expo Router file-based routes
(auth)/
login.tsx
signup.tsx
forgot-password.tsx
reset-password.tsx
(dashboard)/
[orgId]/
index.tsx ← dashboard home
tasks/
crm/
leads/ contacts/ companies/ pipeline/ deals/ reports/
settings/
members/ roles/
security/
create-organization.tsx
select-organization.tsx
\_layout.tsx ← root layout: theme provider + auth gate
components/
ui/ layout/ crm/ tasks/ rbac/
lib/
api.ts auth.ts secureToken.ts constants.ts
stores/
authStore.ts permissionStore.ts uiStore.ts
hooks/
useAuth.ts usePermission.ts useOrg.ts
theme/
tokens.ts ThemeProvider.tsx
types/ ← mirrors frontend/src/types/\*

### Auth Flow (mobile)

1. Login → `POST /api/v1/auth/mobile/login` → `{ access_token, refresh_token, expires_in }` in
   the body, no cookie involved
2. Store `refresh_token` via `expo-secure-store`; keep `access_token` in an in-memory module
   variable (`lib/secureToken.ts`) — same separation principle as web, different storage mechanism
3. Set user + org in `authStore`, permissions in `permissionStore` — identical shape to web
4. Axios request interceptor attaches `Authorization: Bearer <token>`
5. On 401 → read the refresh token from SecureStore → `POST mobile/refresh` → store the new
   (possibly rotated) tokens → retry the original request
6. Logout → read refresh token from SecureStore → `POST mobile/logout` with `{ refresh_token }` →
   clear SecureStore + in-memory token → reset all stores
7. Cold start → read refresh token from SecureStore → if present, call `mobile/refresh` before
   rendering any protected route; if it fails or is absent, route into `(auth)`

### Navigation & Route Protection

- Expo Router — file-based, same mental model as the Next.js App Router already in use
- `(auth)` / `(dashboard)` groups mirror the web's layout groups; `[orgId]` mirrors the web's
  URL-based org context
- Gate `(dashboard)` with Expo Router's Protected Routes, keyed off
  `authStore.status === 'authenticated'`

### State Management

Same three Zustand stores as web, ported with identical interfaces:

- `authStore`, `permissionStore` — never add `persist` middleware, ever
- `uiStore` may persist (theme, nav state) via `@react-native-async-storage/async-storage`, since
  `localStorage` doesn't exist in React Native

### Theming

- No CSS variables on native — port `globals.css` values into a plain `theme/tokens.ts` object
  with `dark`/`light` variants
- RN's `useColorScheme()` supplies the OS-level default; `uiStore.theme` overrides it once picked
  — same behavior as `next-themes`, different mechanism
- Load Inter via `expo-font` (`useFonts`) — one typeface family, matching web
- Plain `StyleSheet.create` for a first pass rather than a Tailwind-for-RN library — revisit only
  if styling velocity becomes a real problem

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
- Indigo accent on interactive elements, one typeface (Inter), same scale hierarchy as web
- Native feel over pixel-parity — iOS vs Android navigation conventions, safe areas, haptics
  (`expo-haptics`) on key actions
- Designed loading/empty/error states, not default RN placeholders
- A screen isn't done until checked on both iOS and Android

### Deployment

- EAS Build for iOS/Android binaries, EAS Submit for store submission, EAS Update for OTA JS
  updates between store releases
- Expo Go for early development only — move to development builds before anything
  production-like; Expo Go tracks only the latest SDK

### Version note (added r14)

Current stable is **Expo SDK 57** (57.0.7 as of this week), riding **React Native 0.86** /
**React 19.2** — confirms the number this doc guessed pre-r10. One scaffolding gotcha:
`create-expo-app@latest` without a template flag is still defaulting to **SDK 54** during the
transition window — use `--template default@sdk-57` explicitly. Re-confirm both numbers the day
you actually scaffold; Expo SDKs move fast enough that this note itself may be stale by then.

## 15. MOBILE MODULE REGISTRY

Same status convention as Section 5/8/9. Nothing built yet — flips to `🔵 ACTIVE` per screen
group as work starts on it, same promotion pattern as Section 9.

Scope note (added r14): this v1 list predates several web features shipped since r10 — Agenda,
Setup/Routing, Templates, Capture. They're intentionally not represented here; decide inclusion
explicitly when this registry is next revisited, don't assume either way.

---

### AUTH SCREENS [⚪ NOT STARTED — natural starting point]

Login, signup, forgot password, reset password — same fields as web (Section 8, AUTH PAGES).
Pairs with Section 5 → AUTH → MOBILE on the backend.

---

### ONBOARDING [⚪ NOT STARTED]

Create organization, select organization — shown after login when no org context exists.

---

### DASHBOARD SHELL [⚪ NOT STARTED]

Tab bar or drawer navigation (mobile equivalent of the web sidebar), org switcher, profile menu.

---

### TASKS [⚪ NOT STARTED]

List with filters, create/edit, permission-gated actions — same permission set as web (`tasks.*`).

---

### CRM — LEADS & PIPELINE [⚪ NOT STARTED]

Lead list + detail, convert flow, pipeline board (list view first — full drag-to-reorder Kanban is
a lot of native gesture work for v1; move-via-menu instead of drag, revisit later).

---

### CRM — CONTACTS [⚪ NOT STARTED]

Contacts and companies list + detail.

---

### CRM — REPORTS [⚪ NOT STARTED — v1: summary cards only]

Full charts (Section 8's bar/pie breakdowns) are a lot of screen real estate for mobile — start
with summary numbers, add charts later if they earn their place.

---

### SETTINGS — MEMBERS, ROLES & PERMISSIONS [⚪ NOT STARTED — lower priority]

Admin-heavy screens, arguably fine to stay web-only for v1. Revisit after the above ships.

---

### SECURITY [⚪ NOT STARTED — lower priority]

Session list, revoke, login events. Likely fine as web-only initially too.

## HOW TO UPDATE THIS DOCUMENT

When phase status changes:

- Update Section 2 STATUS blocks
- Update MODULE REGISTRY entries (change `🔵 ACTIVE` to `✅ DONE`)

When a new backend module is added:

- Add a new entry to Section 5 with its routes and permissions

When a new frontend page is built:

- Update Section 8

When a new mobile screen is built:

- Update Section 15 (Mobile Module Registry)
- Note any deviation from Section 14 (Mobile Architecture) in Section 14 itself if the plan
  changes mid-build — don't let the decided architecture silently drift from what's implemented

When a Zustand store gains new fields:

- Update the store interface in Section 7
- Never add token or sensitive credential fields — see hard rules

When a major architectural decision is made:

- Add a note to the relevant section (3, 4, or 7)

When design tokens change:

- Update Section 3 and `globals.css` together — keep them in sync

When something in the unbuilt module registry starts real work:

- Promote it in Section 9 (`⚪ NOT STARTED` → `🔵 ACTIVE`) and add a proper Section 5/8 entry
- Mark it `✅ DONE` in Section 9 (or remove the entry) once it ships
- Update Section 2's phase status if it changes the active phase

When something existing is modified, not added:

- Route shape changes — update the route block in Section 5/8 in place
- A permission key is renamed, split, or merged — update every place it's listed
- A status flag becomes inconsistent across sections — fix it on sight, don't wait for an audit pass

When a documented assumption gets resolved:

- Delete once confirmed true, or correct with the real behavior once confirmed false — an unresolved assumption sitting for multiple revisions means it was never checked

When the DB schema changes:

- New table → Section 6 Key Tables under the right group
- Migration count changes → update Section 6 same-day — this number goes stale fastest of anything in the doc
- A CHECK constraint, enum, or column change that affects API behavior → note it in the relevant Section 5 module entry

When a new dependency is added:

- Backend: Section 13 with version pinned
- Frontend: same, plus Section 3 if it's a new category of tool

When a module ships with known defects (r11):

- Ship status stays `🔵 ACTIVE`, never `✅ DONE`, until the known-open-items list is empty — a module that compiles but can't perform its core function is not done. Embed the defect list in the module's Section 5 entry so it can't be forgotten (see CAPTURE for the pattern).

Periodic structural drift audit:

- Before any major "update the docs" pass, don't trust the document's own status flags as ground truth — grep the actual source for route counts, migration counts, and file existence, then reconcile. This document has been wrong about counts and statuses in every single revision audit so far (r9: HRM/deployment; r11: four whole shipped features missing). Assume drift by default.
- Cross-check Section 5 (backend) against Section 8 (frontend) against Section 9 (unbuilt module registry) for the same module.
- A doc pasted into a chat conversation is not guaranteed to be the committed `docs/Project_Instruction.md` — whichever copy is more recent should overwrite the other after an update.

Keep this document current. A stale instruction is worse than no instruction.
