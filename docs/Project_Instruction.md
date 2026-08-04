# BUSINESSSAAS — PROJECT MASTER INSTRUCTION

> Last updated: 2026-08-04 (r18 — two HRM Extended phases shipped, neither previously folded into
> this document. **Phase 1 — Resource-Level Permissions**: `authz.Scope` type + `ResolveScope`
> method (`internal/authz`), plus a new `internal/hrm/scope` package (`Predicate()` for list-query
> WHERE-fragments, `Resolver.AuthorizeRecordAccess()` for GET-by-ID checks) — `view_own`/
> `view_team`/`view_all` layered on top of existing flat RBAC. `view_team` is a depth-parameterized
> recursive CTE over `hrm_employees.manager_id` (default depth 1, direct reports only) with an
> explicit path guard, since `manager_id` is a self-FK with no DB-level cycle prevention beyond
> blocking direct self-reference — proven safe against a real 3-node cycle via integration test.
> Migrations `00072`/`00073` seed the three tiers per resource per role and backfill custom
> roles/member overrides that held bare `.view` with no tier (a silent-regression risk otherwise:
> pass the route gate, then get zero rows with no error). Rolled out across all 12 employee-
> record-returning HRM modules plus salary's per-employee endpoints — every GET-by-ID now returns
> 403 `RECORD_ACCESS_DENIED` on an out-of-scope record instead of a silently wrong result. Also
> fixed while touching this area: `hrm.salary.employee` was missing from
> `frontend/src/lib/permissionGroups.ts`, and `PermissionForm.tsx` had its own hardcoded duplicate
> copy of the categorization instead of importing the shared one — same class of bug as the r10
> capture-permissions fix.
> **Phase 2 — Leave Engine Upgrade**: found during the Extended-plan audit, not on the original
> build list — shipped HRM had leave *requests* but no leave *balance*, and Phase 9 (Full & Final
> settlement) cannot compute encashment without one. Three new tables (migration `00074`):
> `hrm_leave_policies` (per-leave-type accrual method/rate, carry-forward cap, encashable flag —
> opt-in; a leave type with no policy behaves exactly as it did before this phase),
> `hrm_leave_transactions` (append-only signed ledger, the source of truth), `hrm_leave_balances`
> (immutable monthly snapshots, modeled on `hrm_employee_salary_records`'s single-`effective_date`
> pattern — the one actually-working "effective-dated" convention in this codebase, not the
> `effective_from`/`effective_to` one Section 4 describes). Two new scheduler jobs:
> `leave.accrue_and_snapshot` (daily) and `leave.year_end_carry_forward` (annual). Two new
> permissions, owner/admin only: `hrm.leave.adjust_balance`, `hrm.leave.encash` — balance/ledger
> reads reuse the existing `hrm.leave.view_own/team/all` tiers from Phase 1 with zero new seeding.
> `CreateRequest`/`ApproveRequest`/`CancelRequest`/`DeleteRequest` now post/reverse ledger
> transactions when a policy exists, via a real `pgx.Tx` bypassing `Repository` (mirroring
> `promotions.Apply`'s existing pattern) — approving past zero always succeeds, there is no
> balance-sufficiency gate anywhere in the write path, by design. One real bug caught by
> integration testing before it shipped: a snapshot boundary comparison used the wrong operator,
> silently dropping any transaction dated exactly on a snapshot's boundary date — fixed, and the
> test that caught it (comparing the checkpoint+delta read against a brute-force full-ledger sum)
> is now a permanent regression guard. Migration count 71 → 75, table count 79 → 82.)

> Last updated: 2026-08-03 (r17 — full-document drift audit, prompted by r16 having only touched
> Section 5. Read the whole doc against the whole source tree (three parallel audits: frontend,
> backend/database, mobile) rather than trusting any section's own status flags. Found drift in
> nearly every section:
> **Section 3** — frontend default theme is actually dark, not light (`app/layout.tsx`); GSAP is
> used across ~21 files, not "sparingly"; a 4th Zustand store (`commandStore`) was unlisted.
> **Section 4** — backend folder tree was missing `internal/hrm/` (the single largest module),
> `internal/platform/scheduler/` and `.../notifications/` (both shipped in r16), and all of
> `internal/dashboard/` — a real, wired, previously undocumented module.
> **Section 5** — added a DASHBOARD entry (flagged its `/orgs/:orgId/` route prefix breaking the
> app-wide `/organizations/:orgId/` convention); AUTH's route table was missing its alias routes
> and wrongly claimed mobile handlers weren't written (they are — see below); PLATFORM — CONTACTS
> was missing a `companies/enrich` route; CRM — DEALS documented the wrong param name for its board
> route (code and the code's own comment disagreed with each other); CRM — REPORTS was missing two
> routes (`rep-performance`, `forecast`).
> **Section 7** — folder tree was missing every file added in r16 (notifications) plus several
> that just predated this audit (`apikeys.ts`, `integrations.ts`, `visitors.ts`, `dashboard.ts`,
> a `crm/visitors` route, a `coming-soon` route). The Zustand store definitions had drifted hard:
> `permissionStore`'s real method is `hasPermission`, not the documented `can`/`canAny`; `authStore`
> has no `status` field and calls its membership field `currentMembership`, not `membership`; the
> "Permission Pattern" code sample called a `usePermission()` hook that doesn't exist anywhere in
> the codebase.
> **Section 8** — no entry existed for notifications (added one); CRM — CAPTURE said "zero
> frontend" but three of four planned pieces are actually built (`settings/apikeys`,
> `settings/integrations`, `crm/visitors`) — just under different paths than planned. The
> `capture.*` permission-group gap in `permissionGroups.ts` is real and still the one thing
> actually missing there.
> **Sections 9/14/15 — the big one.** MOBILE APP said "zero code written" / "nothing built yet."
> False: `mobile/` was committed 2026-07-23 (`a350092`, 93 files, +5799/-144) — one day after r14's
> own dated entry, never folded back in since. Real, working Expo app: auth screens, onboarding,
> dashboard shell, tasks, a single tabbed CRM screen (not split into per-entity routes as planned),
> flat settings. Real gaps that *are* still open: `forgotPassword`/`resetPassword` in `useAuth.ts`
> are stubs despite the pages and backend routes both being real; `components/{crm,tasks,rbac}/`
> are empty; no `security/` route; CRM — CONTACTS never got folded into the tabbed screen and is
> genuinely still unbuilt. Rewrote all three sections against source instead of the old plan.
> **Section 13** — `shopspring/decimal`, `robfig/cron/v3`, `resend/resend-go/v2` (all added in r16)
> were missing entirely; Expo/RN/expo-secure-store rows were still guesses (`57.0.7`, "latest
> compatible") — replaced with the confirmed versions from `mobile/package.json` now that it's
> known to exist.
> Nothing in Section 6, Section 10, or the CAPTURE/HRM entries already fixed in r16 needed further
> changes — re-verified, not re-assumed.)

> Last updated: 2026-08-03 (r16 — HRM Extended Phase 0 completed. Phase 0 had been reported
> substantially done, but a source audit found the branch didn't compile (three packages —
> `hrm/payslips`, `hrm/salary`, `platform/scheduler`, `platform/notifications` — imported
> `shopspring/decimal`, `robfig/cron/v3`, `resend-go/v2` that were never added to `go.mod`), and
> several stale test doubles no longer matched the interfaces they were meant to satisfy. Fixed
> both, then closed every remaining gap: **PLATFORM — SCHEDULER** and **PLATFORM —
> NOTIFICATIONS** promoted from scoping entries to real Section 5 modules (new migrations
> `00067`/`00068`); the two HRM crons already wired to the scheduler were both silently broken
> (milestones queried a column dropped in `00053`; attendance's `resolveShift` queried columns
> that don't exist, `wsa.scope`/`wsa.entity_id` instead of `assignee_type`/`assignee_id`) — both
> fixed, plus a real `attendance.absence_sweep` written to replace the previous stub; sentinel
> system user added (`00069`) so scheduler-triggered writes have a valid actor; PREP MIGRATIONS
> closed out (`00070` legal entity scaffold, `00071` currency columns) confirming
> `hrm_employees.manager_id` already existed under a different name than the doc assumed;
> notifications given a real API surface (list/mark-read/mark-all-read/preferences) and a
> frontend bell + drawer, replacing the placeholder icon. Guard tests extended: hygiene test now
> also catches AI-conversation comment artifacts (found and removed two real instances); the
> permissions test's bidirectional check — used-string-exists-in-a-seed-migration — is now
> codebase-wide instead of HRM-only, and immediately caught a real production bug on the first
> run: `capture/visitors` required `crm.view`, a permission that was never seeded, so the
> route 403'd for every user including owners. Migration count 64 → 71, table count 74 → 79.
> One thing checked and explicitly *not* changed: the original HRM Extended plan called for a
> `routing_test.go` check on `.Get("/")` under `StrictRouting: true`, reasoning it could 404
> requests without a trailing slash. Reading Fiber v3's router source suggested that risk was
> real; a throwaway reproduction against the pinned `v3.2.0` showed both `/x` and `/x/` match
> regardless of `StrictRouting` — not an actual bug in this version, so no test was added for it.)

> Last updated: 2026-07-29 (r15 — HRM extension surface scoped and recorded, plus two shared-
> infrastructure entries that came out of that scoping. New in Section 9: **PLATFORM PRIMITIVES**
> (five buildable shared pieces — notification, scheduler, resource-level permissions, checklist
> engine, form/question engine), **PREP MIGRATIONS** (three cheap-now/expensive-later schema
> hooks), and **HRM EXTENDED MODULES** (the full enterprise-HRM surface: recruitment, onboarding,
> performance, learning, compensation depth, benefits, assets, travel & expense, helpdesk, exit
> management, org chart, succession, analytics, multi-country). All three are scoping only — no
> build decision, no priority ordering, no effort estimates. Section 4 gained an **effective-dated
> (temporal) records** key pattern; Section 10 gained a **decimal money** rule. Section 5 → HRM
> gained a one-line pointer to the extension entry. Section 9 → CRM ADVANCED's
> notification/scheduler bullet was reduced to a pointer — those two now belong to PLATFORM
> PRIMITIVES (one fact, one owner).
> ⚠️ Merge note: the copy this revision was applied to predated the 2026-07-22 mobile r14 entry.
> That content has been restored — Section 5 → AUTH → MOBILE, Section 9 → MOBILE APP promoted to
> 🔵 ACTIVE, Section 13's Expo/RN/Expo Router/expo-secure-store rows, and Sections 14–15 (Mobile
> Architecture, Mobile Module Registry). If mobile was deliberately rolled back, delete those four
> places; nothing else in r15 depends on them.
> ⚠️ Open, unresolved: memory of recent sessions refers to this project as **Havelio**
> (domain `havelio.app`) rather than BusinessSAAS, and describes a mobile Expo scaffold with
> working auth/token handling — both go further than anything in this document. Neither was
> applied here, because a rename touches every section plus `go.mod`'s module path, and because
> this doc's own rule is that pasted docs and conversation memory are not authoritative against
> source. Verify against the repo, then either apply the rename in one deliberate pass or delete
> this note.

> Last updated: 2026-07-22 (r14 — Mobile App promoted ⚪ NOT STARTED → 🔵 ACTIVE. Section 9's
> entry slimmed to a pointer, same pattern as CAPTURE. Section 5 → AUTH gained a real MOBILE
> subsection with the actual route list, and resolved the r10 draft's one open question: checked
> against `frontend/src/app/(auth)/signup/page.tsx`, `Signup` does NOT auto-authenticate — the web
> client calls `login()` separately right after. `MobileSignup` mirrors that instead of inventing
> an auto-login path mobile alone would exercise. Mobile Architecture and Mobile Module Registry
> restored from the r10 archive as new Section 14 and Section 15 — appended at the end rather than
> reinstated at their old Section 9/10 slot, to avoid renumbering every cross-reference in Sections
> 1–13. Section 13's version table gained Expo SDK 57 (57.0.7)/React Native 0.86/Expo Router/
> expo-secure-store rows, plus a scaffolding gotcha: `create-expo-app` without
> `--template default@sdk-57` currently still lands on SDK 54 during the transition window.
> Zero mobile code written — this revision is the doc-level start only, implementation is next.)

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

### Phase 2 — Frontend + CRM + HRM buildout: 🔵 ACTIVE

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
| State         | Zustand (four stores — see Section 7: `authStore`, `permissionStore`, `uiStore`, `commandStore`)       |
| Theme         | `next-themes` (light/dark, default **dark** — `app/layout.tsx` sets `defaultTheme="dark"`)             |
| Forms         | React Hook Form + Zod validation                                                                       |
| Animation     | GSAP — used across most major pages (auth, onboarding, CRM, HRM, settings, drawers), not just the sparse skeleton/count-up usage this row previously described |
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
    dashboard/                ← org-home metrics widget (undocumented until 2026-08-03 audit — see
                                 Section 5 → DASHBOARD; route is /api/v1/orgs/:orgId/dashboard,
                                 breaking the app-wide /organizations/:orgId/ convention)
    capture/                  ← lead auto-capture (r11)
      apikeys/                ← org API keys (generate, validate, revoke)
      public/                 ← /pub/leads public web-form capture endpoint
      email/                  ← inbound email webhook → lead + per-org addresses
      social/                 ← social lead-ad webhooks + integrations
      visitors/               ← website visitor identify + pageview log
    platform/
      contacts/               ← shared contacts + companies (used by CRM and future modules)
      engagement/             ← shared notes, tasks, activities, emails, timeline
      scheduler/              ← named-job registry, Redis-locked, run history (built 2026-08-03)
      notifications/          ← unified in-app/email dispatch (built 2026-08-03)
    crm/
      leads/                  ← CRM lead management (now with capture fields + dedup + round-robin)
      pipeline/               ← pipeline and stages
      deals/                  ← deal CRUD + board view
      reports/                ← CRM analytics endpoints (incl. agenda)
      templates/              ← email/note snippet templates (post-r10)
      settings/               ← per-org CRM settings: lead routing round-robin (post-r10)
    hrm/                      ← 26 sub-packages (departments, positions, employees, salary,
                                 attendance, payslips, leave, approvals, ...) — see Section 5 →
                                 HRM MODULE for the full breakdown; omitted here to keep this tree
                                 scannable, not because it's small
    middleware/               ← auth, business context, logger, rate limit, permission, recover, apikey
    database/                 ← postgres pool + redis client
    config/                   ← env loading and validation
    audit/                    ← append-only audit log
    migrations/               ← Goose SQL migration files
    tests/
      unit/                   ← service-level unit tests (incl. architecture/ guard tests — see
                                 Section 5 note under HRM Extended Phase 0)
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

**Effective-dated (temporal) records (r15)** — for anything where "what was true on date X" is a real question, store `effective_from` / `effective_to` rows rather than overwriting a current value. Applies to: salary records, statutory slabs, compensation bands, reporting relationships, asset assignments, benefit enrollments, and rate tables. Overwriting makes historical recompute, retro/arrears, and any point-in-time analytics impossible after the fact. Corollary: transfers and reassignments are two rows (close one, open one), never an edit.

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
POST /api/v1/auth/signup           (alias: /sign-up)
POST /api/v1/auth/login            (alias: /sign-in)
POST /api/v1/auth/logout           (alias: /sign-out)
POST /api/v1/auth/logout-all       (alias: /sign-out-all)
POST /api/v1/auth/refresh          (alias: /refresh-token)
POST /api/v1/auth/password-reset/request
POST /api/v1/auth/password-reset/confirm
POST /api/v1/auth/oauth/sync
GET  /api/v1/auth/me
```

Every public endpoint above is rate-limited (`internal/auth/routes.go`'s
`RegisterRoutesWithRateLimit`, the variant actually wired in `main.go`); `logout`/`logout-all` are
not, matching `/mobile/logout` below — a holder of an expired access token must still be able to
revoke it.

Token contract:

- Access token → `Authorization: Bearer <token>` header (short TTL, default 15m)
- Refresh token → httpOnly cookie `bsaas_refresh`, path `/api/v1/auth` (long TTL, default 7d)
- Frontend never touches the refresh token directly
- `/refresh` sends cookie, receives new access token in body

**MOBILE [✅ DONE — confirmed built 2026-08-03, doc previously said "not yet written"]:**

Live in `backend/internal/auth/routes.go`, wired into `RegisterRoutesWithRateLimit`:

```
POST /api/v1/auth/mobile/signup   (rate-limited)
POST /api/v1/auth/mobile/login    (rate-limited)
POST /api/v1/auth/mobile/logout   (not rate-limited, matching web /logout)
POST /api/v1/auth/mobile/refresh  (rate-limited)
```

Same `Service`/`Repository` as web auth, zero new business logic — handler methods
`MobileSignup`/`MobileLogin`/`MobileLogout`/`MobileRefresh` (`internal/auth/handler.go`) return the
refresh token in the JSON body instead of setting the `bsaas_refresh` httpOnly cookie, and read it
back from the request body on `mobile/refresh` / `mobile/logout` instead of the cookie jar.
`logout-all`, `password-reset/*`, and `me` stay shared as-is — none of them touch tokens, no mobile
variant needed. This was never actually a gap — see Section 9 → MOBILE APP for the fuller
correction: the `mobile/` client itself is also far more built than this doc previously claimed.

Resolved (was an open assumption in the r10 draft): `Signup` does not auto-authenticate.
`frontend/src/app/(auth)/signup/page.tsx` creates the user then calls `login()` separately to
bootstrap the session. `MobileSignup` should do the same — create the user, return it with no
tokens, let the client call `mobile/login` right after. Keeps both clients on identical semantics
instead of giving mobile a second, divergent signup path.

---

### USER [✅ DONE]

Routes:

```
GET    /api/v1/me
PATCH  /api/v1/me
PATCH  /api/v1/me/settings
PATCH  /api/v1/me/preferences
POST   /api/v1/me/avatar
GET    /api/v1/me/avatars
POST   /api/v1/me/avatars/:avatarId/activate
DELETE /api/v1/me/avatars/:avatarId
```

`/users/me` (GET/PATCH) is a backward-compatible alias, same handlers as `/me`.

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
POST   /api/v1/organizations/:orgId/rbac/check-member   ← alias of /check, same handler

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

Permissions: `tasks.view` · `tasks.create` · `tasks.update` · `tasks.delete` · `tasks.view_all`
Statuses: `todo` · `in_progress` · `done` · `cancelled`

`tasks.view_all` (migration 00065) isn't route-gated via `permFn` — `handler.List` checks it
inline (`authzSvc.Can(ctx, userID, orgID, "tasks", "view_all")`) to decide between "see everyone's
tasks" and "see only your own." This is the one working precedent for the `view_own`/`view_team`/
`view_all` scoping pattern Phase 1 (resource-level permissions) is about to formalize across HRM —
worth reading before designing that ADR.

---

### DASHBOARD [✅ DONE — found undocumented in the 2026-08-03 audit]

Org-home metrics widget. One route, no permission gate beyond org membership:

```
GET /api/v1/orgs/:orgId/dashboard   ← requireAuth + requireOrgMatch only, no permFn
```

⚠️ Route prefix is `/orgs/:orgId/` — every other module in this registry uses
`/organizations/:orgId/`. Not fixed as part of this audit pass (would be a breaking API change for
whatever frontend already calls it — `frontend/src/lib/dashboard.ts` — needs to move in the same
commit as the backend route if this is ever corrected). Flagging so it doesn't get silently copied
as the pattern for the next new module.

---

### PLATFORM — CONTACTS [✅ DONE]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/contacts
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/contacts/:contactId

GET/POST         /api/v1/organizations/:orgId/crm/companies
GET              /api/v1/organizations/:orgId/crm/companies/enrich       ← crm.companies.view (registered before :companyId)
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

### PLATFORM — SCHEDULER [✅ DONE — built 2026-08-03]

Generic named-job registry in `internal/platform/scheduler/`: in-process ticker (30s), Redis
`SETNX` distributed lock per job (multi-instance safe), run history, manual trigger. Migration
`00067`. Supersedes the PLATFORM PRIMITIVES §2 scoping entry in Section 9 — built, not just scoped.

Routes:

```
GET  /api/v1/platform/scheduler/jobs              ← platform.scheduler.view
GET  /api/v1/platform/scheduler/jobs/:name/runs   ← platform.scheduler.view
POST /api/v1/platform/scheduler/jobs/:name/run    ← platform.scheduler.manage
```

Permissions: `platform.scheduler.view` / `.manage` — granted to owner/admin.

Two jobs registered in `main.go`. Both were already wired to the scheduler before this revision,
but neither actually worked — one was a stub, the other silently errored on every run:

- `milestones.generate_upcoming` (daily 01:00) — was querying `hrm_employees.status`, a column
  migration `00053` dropped in favor of `status_id`; the query errored every time, the error was
  swallowed, and the cron always no-op'd. Fixed to join `hrm_employee_statuses` on
  `category='active'`.
- `attendance.absence_sweep` (daily 02:00) — was a literal stub (`slog.Info(...stub executed...)`
  and nothing else). Now marks employees absent for the prior day when no attendance record
  exists, it's a working day per their resolved shift, it isn't a holiday (employee → department
  → org calendar cascade), and they have no approved leave. Fixing this required also fixing a
  pre-existing bug in `attendance.resolveShift`: it queried columns `wsa.scope`/`wsa.entity_id`,
  but `hrm_work_schedule_assignments` (migration `00027`) actually names them
  `assignee_type`/`assignee_id` — shift resolution always silently failed and every attendance
  record fell back to a flat 8h default instead of the employee's real shift.

Both jobs need a valid `created_by`/actor UUID for system-generated rows; migration `00069` seeds
a sentinel system user (`scheduler.SystemUserID`, never a real login) for exactly this.

---

### PLATFORM — NOTIFICATIONS [🔵 ACTIVE — infra + API built 2026-08-03, digest/push not built]

Unified dispatch in `internal/platform/notifications/`: per-user, per-event-type, per-channel
preference matrix; in-app (DB row) and email (Resend — real, not a stub) channels. Migration
`00068`. Supersedes the PLATFORM PRIMITIVES §1 scoping entry in Section 9.

Routes (self-scoped to the requesting user, like `/me` — `requireAuth` only, no RBAC permission
check, since a user always owns their own notifications):

```
GET   /api/v1/notifications                ← list (paginated) + unread_count
POST  /api/v1/notifications/:id/read
POST  /api/v1/notifications/read-all
GET   /api/v1/notifications/preferences
PATCH /api/v1/notifications/preferences
```

Frontend: Topbar bell wired to a drawer (`NotificationDrawer`) with unread badge, mark-read,
mark-all-read (previously a decorative placeholder icon with no handler). Only consumer today is
`auth` (password reset / invite emails) — no HRM module calls `Dispatch()` yet, correctly, since
none is queued (Section 9's build-order note applies: build the primitive when the first real
consumer arrives). Digest batching and push channel are not built.

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
GET              /api/v1/organizations/:orgId/crm/deals/:pipelineId/board
```

The board route's param is `:pipelineId` (board = all deals in one pipeline, not one deal) —
`internal/crm/deals/routes.go:37`. Its own doc-comment (same file, line 19) still says `:dealId`;
code and comment disagree with each other, not just with this doc. Corrected here to match the
actual registered route; the source comment itself is still wrong.

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
GET /api/v1/organizations/:orgId/crm/reports/rep-performance
GET /api/v1/organizations/:orgId/crm/reports/forecast
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
GET      /api/v1/organizations/:orgId/capture/visitors          ← capture.visitors.view (was nonexistent crm.view, 403'd for everyone; fixed 2026-08-03)
```

**Public routes:**

```
POST /api/v1/pub/leads                       ← RequireAPIKey, scope capture:leads — WORKS (web form / chat)
POST /api/v1/pub/visitors/identify           ← RequireAPIKey, scope capture:visitors
POST /api/v1/pub/email/webhook               ← ⚠️ NO verification (Fix B) — do not expose publicly yet
GET/POST /api/v1/pub/social/:platform/webhook ← ⚠️ NO signature check; GET accepts any verify_token (Fix B)
```

**API key contract:** raw key `bs_live_<64 hex>`, shown exactly once at creation; SHA-256 hash + 16-char prefix stored; scopes `capture:leads` (+ `capture:visitors`, constant pending); optional per-key `allowed_origins` and `expires_at`.

**Permissions seeded (00062):** `capture.apikeys.view/create/delete` · `capture.email.manage` · `capture.social.manage` · `capture.visitors.view` — granted to owner/admin. Apikeys and visitors are referenced correctly; email/social routes still use `settings.view` instead of their seeded keys (Fix A item 3, partially open — see below).

**Deliberate scope reductions (documented, not bugs):**

- Social is a **webhook skeleton**, not a real Facebook integration — no Graph API `leadgen_id` fetch, no OAuth connect, no field-mapping engine; it maps flat payload fields, which real FB webhooks don't send. Real integration is tracked separately (Section 9).
- Visitors is **manual identify** (Segment-style: `traits.email/name/company` → lead), not IP→company enrichment. No IPinfo, no Redis queue, no background worker.
- No hosted form / JS embed endpoint yet (was Step 4 second half) — capture frontend work.

**Known open items (r11 audit — this list IS the current work):**

_Fix Pass A — feature-breaking + correctness:_

1. `CreateLead(ctx, orgID, "", …)` in email/social/visitors → `created_by` gets empty string → invalid-UUID error on every system-generated lead. Fix: migration 00065 drops NOT NULL on `crm_leads.created_by`; model `CreatedBy *string`; pass nil for system captures; UI renders null as "System".
2. `social/repository.go` INSERTs into `access_token_enc`; column is `access_token` (00060) → connect always fails. Fix repo SQL.
3. ~~Visitors route used nonexistent `crm.view`, 403'd for everyone~~ — fixed 2026-08-03, now
   `capture.visitors.view` (caught by a new guard test, `TestPermissions_UsedStringsExistInMigrations`,
   that checks every `permFn(...)` string against seeded migration keys, codebase-wide). Still
   open: email/social routes use `settings.view` instead of their seeded `capture.email.manage` /
   `capture.social.manage` keys; `capture:visitors` scope constant + scope/name validation in
   `GenerateKey` not added.
4. `ValidateKey` never checks `expires_at` (`ErrKeyExpired` sentinel unused). Add the check.
5. Dedup: scope to `req.CaptureSource != nil` only; `LOWER(email)` match; note module `"crm"`; skip note when `userID == ""`; don't swallow the note error silently.
6. ~~Delete leftover AI-conversation comments in `public/handler.go`~~ — fixed 2026-08-03 (a
   five-line internal monologue about `created_by`/UUID handling). `leads/service.go` checked
   clean — no artifacts found there. A new hygiene test (`TestHygiene_NoAIConversationArtifacts`)
   now catches this class going forward.
7. Minor: `social` model `AccessToken` json tag → `"-"`; `org_api_keys.created_by` ON DELETE CASCADE → RESTRICT; UNIQUE `(org_id, session_id)` on `website_visitors`; UNIQUE on `org_api_keys.key_hash`.

_Fix Pass B — security, required before any public exposure:_ 8. Inbound email webhook HMAC verification (`WEBHOOK_EMAIL_SECRET`). 9. Facebook `X-Hub-Signature-256` verification (`FACEBOOK_APP_SECRET`) + real `hub.verify_token` comparison (`FACEBOOK_VERIFY_TOKEN`). 10. Redis rate limiting on every `/pub/*` route (new `NewPublicCaptureRateLimit` constructor; per-key where a key exists, per-IP on webhooks).

---

### HRM MODULE [✅ DONE — verified r9; dynamic statuses added post-r10; route count corrected 2026-08-03; leave balance engine added r18]

All routes live under `/api/v1/organizations/:orgId/hrm/...`, permission-gated (`hrm.<submodule>.<action>`), **26 sub-modules, 216 routes** (206 as of the 2026-08-03 recount + 10 new leave-balance routes, r18). This entry summarizes; per-route detail belongs in a dedicated `docs/modules/hrm.md`.

**Database:** 45 tables. 40 verified in r9 (migrations `00020`–`00050`, of which `00048` is unrelated CRM seed data) + `hrm_employee_statuses` (00053) + `hrm_legal_entities` (00070, previously undercounted here — Section 6 already had it) + `hrm_leave_policies`/`hrm_leave_transactions`/`hrm_leave_balances` (00074, r18).

**Resource-level permissions (r18):** `view_own`/`view_team`/`view_all` scoping (`internal/hrm/scope`) layered on top of every group below plus salary's per-employee endpoints — see Section 9 → PLATFORM PRIMITIVES #3 for the primitive itself; this entry just records that it's now live across the whole module, not a config-only subset.

**Group A — Setup/Config** (`departments, positions, salary, approvals, warningtypes, doctemplates, shifts, holidays, contracts`): 72 routes. Salary formula engine via `expr-lang/expr`. Approvals' instance-list endpoint (`GET /hrm/setup/approvals/instances`) exists — a prior "missing since r9" note here was stale; confirmed against `internal/hrm/approvals/routes.go` 2026-08-03.

**Group B — Lifecycle** (`employees, promotions, transfers, resignations, terminations`): 37 routes + 4 new:

```
GET/POST     /organizations/:orgId/hrm/employee-statuses        ← hrm.employees.view / hrm.employees.manage_setup
PATCH/DELETE /organizations/:orgId/hrm/employee-statuses/:id    ← hrm.employees.manage_setup
```

Dynamic statuses (00053): per-org status list with category (`active/inactive/on_leave/terminated`) + color token; `hrm_employees.status_id` FK; migration auto-seeded defaults and mapped existing employees.

**Group C — Disciplinary** (`warnings, complaints, employeedocs, acknowledgements`): 34 routes. Cross-module writes via `ON CONFLICT DO NOTHING` + direct `pgPool.Exec` to avoid import cycles.

**Group D — Time & Compensation** (`attendance, payslips`): 19 routes. Multi-punch attendance, `ComputeSlab` progressive tax, immutable finalized payslips. Attendance's `resolveShift` had a column-name bug (`wsa.scope`/`wsa.entity_id` vs the real `assignee_type`/`assignee_id`) that silently no-op'd shift resolution on every call — fixed 2026-08-03 (see Section 5 → PLATFORM — SCHEDULER for the `attendance.absence_sweep` job this also unblocked).

**Group E — Recognition & Communication** (`awards, announcements, calendar, milestones`): 25 routes. Nightly crons for milestones/absences now genuinely run (see Section 5 → PLATFORM — SCHEDULER) — `milestones.generate_upcoming` previously errored on every run against a column `00053` had already dropped, silently, so it never generated anything despite being "wired."

**Leave** (`leave` — types + requests + balances): 22 routes (12 original + 10 added r18), `hrm.leave.*` permissions. Balance tracking (accrual, carry-forward, encashment recording) is opt-in per leave type — a type with no `hrm_leave_policies` row behaves exactly as it always has, zero enforcement. Two new scheduler jobs (`leave.accrue_and_snapshot` daily, `leave.year_end_carry_forward` annually — see Section 5 → PLATFORM — SCHEDULER) and two new owner/admin-only permissions (`hrm.leave.adjust_balance`, `hrm.leave.encash`); balance/ledger reads reuse the existing `hrm.leave.view_own/team/all` tiers, no new scope permissions needed. Encashment records days only — it does not compute a currency amount, that's Section 9 → HRM EXTENDED MODULES → Compensation depth (F&F)'s job once built.

**Reports:** 3 routes.

**Approval chain wiring:** callback registry on the approvals service; promotions, transfers, terminations, warnings, awards each register `HandleApprovalDecision` in `main.go`.

**Extension surface (r15):** scoped but unbuilt — see Section 9 → HRM EXTENDED MODULES. This entry covers what is shipped only.

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

### Migration Count: 75

Files live in `backend/internal/migrations/`. Run via `goose` or `make migrate`.
r11 ended at 64. Post-r11: `00065` tasks `related_type`/`related_id` context + `tasks.view_all`
permission (not the `crm_leads.created_by` fix Fix Pass A originally expected — that's still
open) · `00066` HRM money `float64`→`decimal.Decimal` + `organizations.money_rounding_scale/mode`
+ payslip totals widened to `NUMERIC(18,4)` · `00067` scheduler tables
(`platform_scheduled_jobs`, `platform_job_runs`) · `00068` notification tables
(`platform_notifications`, `platform_notification_preferences`) · `00069` sentinel system user
for scheduler-triggered writes · `00070` `hrm_legal_entities` + `legal_entity_id` backfilled onto
36 HRM tables · `00071` `currency CHAR(3)` prep (new columns + standardized existing ones) ·
`00072` seeds `view_own`/`view_team`/`view_all` per resource per role (Phase 1) · `00073`
backfills that tier onto custom roles/member overrides holding bare `.view` · `00074`
`hrm_leave_policies`/`hrm_leave_transactions`/`hrm_leave_balances` (Phase 2) · `00075` seeds
`hrm.leave.adjust_balance`/`hrm.leave.encash`, owner/admin only.

### Key Tables (82 total)

**Core / auth / org (14):**
`users` · `organizations` · `organization_members` · `organization_invitations` · `permissions` · `roles` · `auth_accounts` · `sessions` · `login_events` · `verification_tokens` · `subscriptions` · `organization_usage` · `audit_logs` · `tasks`

**Platform (10):**
`platform_contacts` · `platform_companies` · `platform_notes` · `platform_tasks` · `platform_activities` · `platform_email_logs` · `platform_scheduled_jobs` · `platform_job_runs` · `platform_notifications` · `platform_notification_preferences`

**CRM (6):**
`crm_leads` (+ `custom_fields`, `capture_source`, `capture_metadata` since 00058) · `crm_pipelines` · `crm_pipeline_stages` · `crm_deals` · `crm_templates` · `crm_settings`

**Capture (7):**
`org_api_keys` · `org_inbound_emails` · `social_integrations` · `website_visitors` · `visitor_pageviews` · `inbound_email_logs` · `social_lead_logs`

**HRM (45):**
`hrm_legal_entities` (prep migration `00070`, zero business logic yet — see PREP MIGRATIONS in Section 9) ·
Group A (19): `hrm_departments` · `hrm_positions` · `hrm_salary_components` · `hrm_salary_structures` · `hrm_salary_structure_components` · `hrm_approval_templates` · `hrm_approval_template_levels` · `hrm_approval_instances` · `hrm_approval_decisions` · `hrm_warning_types` · `hrm_warning_escalation_rules` · `hrm_document_templates` · `hrm_document_bulk_sends` · `hrm_shifts` · `hrm_work_schedule_assignments` · `hrm_holiday_calendars` · `hrm_holidays` · `hrm_calendar_assignments` · `hrm_employee_contracts`
Group B (6): `hrm_employees` · `hrm_promotions` · `hrm_transfers` · `hrm_resignations` · `hrm_terminations` · `hrm_employee_statuses`
Group C (4): `hrm_employee_warnings` · `hrm_complaints` · `hrm_employee_documents` · `hrm_acknowledgements`
Group D (6): `hrm_attendance_periods` · `hrm_attendance_records` · `hrm_employee_salary_records` · `hrm_payslip_runs` · `hrm_payslips` · `hrm_payslip_lines`
Group E (4): `hrm_awards` · `hrm_announcements` · `hrm_calendar_events` · `hrm_employee_milestones`
Leave (5): `hrm_leave_types` · `hrm_leave_requests` · `hrm_leave_policies` · `hrm_leave_transactions` · `hrm_leave_balances`

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
            leads/  pipeline/  reports/  agenda/  visitors/    ← visitors added, undocumented until 2026-08-03
            setup/routing/  setup/templates/
          contacts/  companies/  companies/[companyId]/
          tasks/
          hrm/                ← all HRM pages incl. setup/statuses
          settings/ (members, roles, profile, apikeys, integrations)  ← apikeys/integrations added, undocumented until 2026-08-03
          security/sessions/
          coming-soon/        ← generic placeholder route, undocumented until 2026-08-03
      api/lead-capture/       ← Next.js route handler proxying to /pub/leads, undocumented until 2026-08-03
    components/
      ui/  layout/  crm/  tasks/  members/  roles/  settings/  hrm/  providers/
      notifications/NotificationDrawer.tsx   ← added 2026-08-03, see Section 8
    contexts/DrawerContext.tsx
    lib/
      api.ts  auth.ts  constants.ts  token.ts  jwt.ts  queryKeys.ts
      crm/ (leads, contacts, companies, deals, pipelines, reports, settings, templates)
      hrm/ (…)
      members.ts  roles.ts  org.ts  profile.ts  security.ts  tasks.ts  permissionGroups.ts
      apikeys.ts  integrations.ts  visitors.ts  dashboard.ts  notifications.ts   ← all added, undocumented until 2026-08-03
    stores/ (authStore, permissionStore, uiStore, commandStore)   ← commandStore was missing from this list
    hooks/ (useIsMobile, …)
    types/ (api, auth, crm, hrm, org, rbac, task, notification)   ← notification type added 2026-08-03
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

**Four stores, not three** — `commandStore` was missing from this section entirely until the
2026-08-03 audit. Interfaces below are copied from the real source
(`frontend/src/stores/*.ts`), not the previous doc's assumed shape, which had drifted on both
`authStore` and `permissionStore`.

**`authStore`** — who the user is and which org they're in:

```ts
interface AuthState {
  user: SafeUser | null;
  currentOrg: Business | null;
  currentMembership: MembershipWithRole | null; // was documented as `membership`; there is no `status` field at all
  setUser: (user: SafeUser | null) => void;
  setOrg: (org: Business | null, membership: any) => void;
  reset: () => void;
}
```

**`permissionStore`** — what the user can do in the current org:

```ts
interface PermissionState {
  permissions: string[];
  setPermissions: (permissions: string[]) => void;
  hasPermission: (permission: string) => boolean; // was documented as `can`; `canAny` doesn't exist
  reset: () => void;
}
```

**`uiStore`** — sidebar, mobile menu, and theme (persisted: `sidebarCollapsed` + `theme` only):

```ts
interface UIState {
  sidebarCollapsed: boolean;
  mobileMenuOpen: boolean; // not previously documented
  theme: "light" | "dark";
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;
  toggleMobileMenu: () => void;
  setMobileMenuOpen: (open: boolean) => void;
  setUiTheme: (theme: "light" | "dark") => void;
}
```

**`commandStore`** — the ⌘K command menu's open state, nothing else (missing from this doc
entirely until now):

```ts
interface CommandState {
  isOpen: boolean;
  setOpen: (open: boolean) => void;
  toggle: () => void;
}
```

**Hard rules for Zustand:**

- Never add `persist` middleware to `authStore` or `permissionStore` — persisted auth state is a security risk
- `persist` is allowed on `uiStore` (and `commandStore`, though nothing in it needs persisting today)
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
const { hasPermission } = usePermissionStore(); // no API call — reads the in-memory store directly

{
  hasPermission("tasks.create") && <Button onClick={openCreateModal}>New Task</Button>;
}
```

(This sample previously called a `usePermission()` hook with a `can()` method — neither exists.
Every real call site in the codebase uses `usePermissionStore()` + `hasPermission()` directly, e.g.
`frontend/src/app/(dashboard)/[orgId]/tasks/page.tsx`.)

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
Topbar bell opens `components/notifications/NotificationDrawer` (added 2026-08-03 — see
PLATFORM — NOTIFICATIONS below); org-home page also renders the metrics widget via `lib/dashboard.ts`
against the backend DASHBOARD route (Section 5).

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

### PLATFORM — NOTIFICATIONS [🔵 ACTIVE — built 2026-08-03]

`components/notifications/NotificationDrawer.tsx`, opened from the Topbar bell (unread-count
badge, mark-read on click, mark-all-read action) via `lib/notifications.ts` +
`types/notification.ts`. Backend: Section 5 → PLATFORM — NOTIFICATIONS. No settings/preferences
page yet — the backend `GET/PATCH /notifications/preferences` routes exist but nothing in the UI
calls them.

### CRM — CAPTURE [🔵 PARTIAL — most of the frontend exists, previously documented as "zero"]

This entry was wrong: three of the four planned pieces are built, just under different paths than
originally planned (`settings/` instead of `crm/setup/capture`):

- `/[orgId]/settings/apikeys` (`lib/apikeys.ts`) — API key list, create drawer with show-once raw key, revoke.
- `/[orgId]/settings/integrations` (`lib/integrations.ts`) — inbound email address + social integration management (list/create/delete).
- `/[orgId]/crm/visitors` (`lib/visitors.ts`) — visitor list + JS embed snippet generator.
- `app/api/lead-capture/route.ts` — a live Next.js route handler proxying to the backend's public `/pub/leads` endpoint.

**Still genuinely missing:** the `capture.*` permission group in `lib/permissionGroups.ts` — the
six capture permissions (`capture.apikeys.*`, `capture.email.manage`, `capture.social.manage`,
`capture.visitors.view`) are invisible in the Roles picker even though the pages that need them
exist and work. This is the one item worth prioritizing here — it's the same class of bug as the
r10 HRM permission-picker fix. No dedicated `types/capture.ts` — types are inlined in each lib file.

### HRM [✅ DONE — statuses setup added post-r10]

All under `/[orgId]/hrm/...`: departments, positions, employees, leave, attendance, payroll, lifecycle, warnings, compliance, recognition, reports, and setup/{approvals, document-templates, holidays, salary, shifts, warning-types, **statuses**}. `ApprovalInstanceView` drawer wired into Warnings, Recognition, and Lifecycle pages.

---

## 9. UNBUILT MODULE REGISTRY

`⚪ NOT STARTED` = no code exists. When an item starts, promote it to `🔵 ACTIVE` here and give it a real entry in Section 5 and/or Section 8 with routes and permissions. When it ships, mark `✅ DONE` or remove the entry. Nothing here carries priority ordering — pick up whatever's needed, including because it connects to what you're currently building (Section 1).

PLATFORM PRIMITIVES and PREP MIGRATIONS sit first because most entries below reference them, not because they rank first. Position in this list still carries no priority.

---

### PLATFORM PRIMITIVES [🔵 PARTIAL — #1/#2 built 2026-08-03, see Section 5; #3–#5 ⚪ NOT STARTED]

Five buildable pieces of shared infrastructure that multiple modules need. They live in `internal/platform/` for the same reason contacts and engagement do: building them per-module means schema duplication across CRM, HRM, and everything after.

Two of these (notification, scheduler) were previously named only inside the CRM Advanced Functionality Pass entry below, as a parenthetical dependency. That mention is now superseded by this entry — one fact, one owner. The CRM entry keeps the dependency note but not the description.

**1. Notification system — ✅ built, see Section 5 → PLATFORM — NOTIFICATIONS**
Delivery-path abstraction: in-app, email (Resend, real), (later) push. Per-user preference matrix, per-event-type opt-out, read state all built; digest batching not built. Consumer today: `auth` (password reset/invite) only — no CRM or HRM consumer wired yet. Consumers if HRM extends: every single module in the HRM Extended Modules entry below — onboarding reminders, appraisal cycle deadlines, certification expiry, benefit enrollment windows, helpdesk SLA breach, exit clearance nudges.
Hard dependency (EMAIL SENDING) resolved — Resend is wired and live, not stubbed.

**2. Scheduler registry — ✅ built, see Section 5 → PLATFORM — SCHEDULER**
Named recurring jobs with Redis distributed locking (multi-instance safe), run history, manual trigger built. Failure alerting (beyond `consecutive_failures` tracking) not built. The two ad-hoc HRM crons (milestones, absences) are migrated onto this — and both had latent bugs (one a stub, one silently erroring on a dropped column) fixed as part of the migration; see Section 5 entry for detail.
Consumers still to come: analytics snapshot runs, certification expiry sweeps, enrollment window open/close, ticket auto-close, system access revocation on last working date, cycle phase transitions.

**3. Resource-level permissions — ✅ built (r18), see Section 5 → HRM MODULE**
Beyond module/action RBAC — per-record filtering, and a `view_own / view_team / view_all` tier applied consistently. `view_team` resolves against a reporting-manager chain (a depth-parameterized recursive CTE over `hrm_employees.manager_id`, default depth 1 — see PREP MIGRATIONS). Shipped as `authz.Scope`/`ResolveScope` (`internal/authz`) + a new `internal/hrm/scope` package (`Predicate()` for list queries, `Resolver.AuthorizeRecordAccess()` for GET-by-ID), rolled out across all 12 employee-record HRM modules plus salary's per-employee endpoints.
This is the primitive with the highest severity of failure: appraisal draft leakage, salary visibility, anonymous 360 de-anonymization, and succession/flight-risk exposure are trust and legal issues, not UX polish. Every future module that returns employee-level records (performance, compensation depth, succession, etc.) should build on this from day one rather than retrofitting later — retrofitting is exactly what r18 just did for the 12 already-shipped modules, and it touched every one of their repository query layers.

**4. Checklist engine**
Template + typed items + `owner_type` → assignee resolution at instantiation + offset-based due dates + blocking vs non-blocking + instance completion tracking.
One engine with a `checklist_type` discriminator, not one per consumer.
Known consumers (all unbuilt): HRM onboarding, exit clearance, probation confirmation, transfer handover.

**5. Form / question engine**
Configurable sections → typed questions → typed responses → scoring → aggregate, with definition snapshotting so historical records render as they were authored.
Known consumers (all unbuilt): interview scorecards, appraisal forms, LMS assessments, exit interviews, potential-criteria assessment, employee surveys.

⚠️ Build order note: #1–#3 have real consumers in already-shipped or already-planned work and are genuinely blocking. #4 and #5 have **zero current consumers** — every module that would use them is unbuilt. They are documented here because the pattern was identified across five independent designs, not because they should be built speculatively. Build them when the first real consumer arrives, and only if the second one is already visible.

---

### PREP MIGRATIONS [✅ DONE — 2026-08-03]

Schema hooks that cost one migration today and a system-wide rewrite if deferred:

- ~~`hrm_employees.reporting_manager_id` — verify whether this exists at all~~ — resolved: it
  exists as `hrm_employees.manager_id` (self-FK, `idx_hrm_emp_manager_id`, migration `00021`).
  There was never a missing column here — the open question was only ever "does the doc's assumed
  name match the source," and it didn't; the underlying capability was already there. Appraisal
  routing, expense approval, clearance ownership, `view_team` scoping, and org chart all resolve
  against `manager_id`, not a new column.
- `legal_entity_id` — done, migration `00070`. Minimal `hrm_legal_entities` table (zero business
  logic, per the original scope), one default entity per org, nullable FK backfilled onto every
  HRM table that carries a direct `org_id` (36 tables).
- `currency` — done, migration `00071`. `CHAR(3)` added alongside money columns that didn't have
  one (`hrm_employee_salary_records`, `hrm_promotions`, `hrm_salary_components`,
  `hrm_salary_structure_components`, `hrm_payslip_lines`), backfilled from each row's org
  currency; existing ad-hoc `TEXT` currency columns (`organizations`, `hrm_awards`,
  `hrm_terminations.severance_currency`, `hrm_payslips`, `hrm_payslip_runs`) standardized to the
  same `CHAR(3)`.

---

### CRM — ADVANCED FUNCTIONALITY PASS [🔵 ACTIVE — capture in flight]

Mridha's wish list, triaged by buildability. **In flight now:** lead auto-capture (Section 5 → CAPTURE: fix pass + frontend). **Shipped from this list:** lead routing round-robin, templates & snippets, agenda/smart-task view.

**Contained — no new infrastructure or one well-understood integration:**

- Activity metrics reporting (calls/meetings/deals-closed per rep)
- Sales forecasting (weighted pipeline by historical win rate per stage)
- Data enrichment on company domain (Clearbit or Apollo — real per-lookup cost)
- Trigger-based actions — small hardcoded set first ("on Closed Won, do X"), not a rule engine

**Needs new infrastructure first, or a real ongoing vendor cost:**

- Reminders / SLA alerts — blocked on notification + scheduler; see PLATFORM PRIMITIVES above
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

Per-record access control ("only deals they own") beyond module/action RBAC. Touches every repository's query layer — needs its own ADR when it starts. Full rationale and consumer list: PLATFORM PRIMITIVES #3 above.

---

### BILLING & SUBSCRIPTION MANAGEMENT [⚪ NOT STARTED]

Payment provider, plan tiers, usage limits, invoices, billing UI. `organization_usage` and `organizations.max_seats` already exist as head starts.

---

### HRM FUNCTIONALITY PASS [⚪ NOT STARTED]

Post-completion polish pass over the shipped HRM module, same spirit as the CRM pass. Includes the known open item: approval-instance list endpoint.

---

### HRM EXTENDED MODULES [⚪ NOT STARTED — scoping only, no build decision implied]

The shipped HRM module (Section 5) covers setup, lifecycle, disciplinary, time & compensation, recognition, and leave. That is a complete core. This entry records the extension surface an enterprise-grade HRM would additionally cover — scoped in a five-part design discussion, not committed to. Nothing here is a plan; it is a map, so that a future decision to build any part of it starts from a design rather than a blank page.

Everything below depends on PLATFORM PRIMITIVES above. Those dependencies are stated once per module and not restated — see that entry for what each primitive is.

**Talent acquisition & entry**

- **Recruitment / ATS** — requisitions (approval-gated), postings, candidates, applications, configurable pipeline stages, interviews + scorecards, offers (approval-gated), referrals. Two hard structural rules: candidate ≠ application (stage lives on the application), and stage history from day one — `crm_deals` skipped this and that is exactly why sales velocity is blocked above. Reuses the approval engine and `hrm_document_templates`. Resume parsing is a vendor bolt-on, not in scope.
  Depends on: EMAIL SENDING (not optional — an ATS without candidate email is half a product), resource-level permissions (scorecard bias-blocking), scheduler (candidate data purge / GDPR).

- **Onboarding** — checklist-driven, multi-owner, offset-based due dates relative to joining date. Not a table of its own: this is checklist engine consumer #1.
  Depends on: checklist engine, notification.

**Growth & development**

- **Performance Management** — four sub-systems: goals/OKR with cascading parent-child and check-in history; appraisal cycles (configurable rating scales, template + scale snapshotted onto each instance, publish-immutable, phase state machine, `manager_id_snapshot` frozen at instantiation); 360 feedback (formal cycle-bound + continuous) with anonymity enforced at the repository layer plus a minimum-response threshold; PIP feeding into existing terminations. `final_rating` must be structured and queryable — compensation and succession both read it.
  Depends on: form engine, resource-level permissions (draft leakage is the failure mode), notification, scheduler, `hrm_employees.manager_id` (exists — see PREP MIGRATIONS).

- **Learning & Development** — courses with mandatory version pinning on enrollment, modules/lessons, assessments, instructor-led sessions, certifications with expiry, skills taxonomy, training requests + budgets. No SCORM player, no video hosting — external links, PDFs via the Files module, mark-complete, quiz. Certification expiry sweep is the highest-value feature and is entirely scheduler-dependent. `hrm_skills` is consumed by recruitment, performance, and succession — treat it as shared taxonomy, not an LMS-internal table.
  Depends on: scheduler, form engine, reuses `hrm_acknowledgements` for compliance evidence.

**Compensation & benefits**

- **Compensation depth** — salary revision cycles (effective-dated, batch-approved, merit matrix from rating × compa-ratio), bonus/incentive engine (reuses `expr-lang/expr` with a widened shared evaluation context, `calculation_snapshot` JSONB mandatory), loans/advances with amortization schedules generated once at approval, reimbursement payout, statutory compliance (country-pluggable schema, per-country Go implementation behind an interface — no universal statutory formula engine).
  Payroll engine additions this requires: `run_type` on payslip runs (off-cycle, F&F, arrears), `line_type` enum, `is_employer_contribution`, deterministic calculation order, negative-net guard, mandatory dry-run preview.
  ⚠️ This cluster is internally coupled — loans → statutory (perquisite), benefits → statutory, revisions → payroll (arrears), bonus → performance. Design it whole, not in pieces.
  Depends on: decimal money discipline (Section 10), temporal modeling (Section 4), resource-level permissions (salary visibility is the sharpest case in the system).

- **Benefits Administration** — plans, coverage tiers, cost splits, enrollment windows (open / new-hire / qualifying-event), dependents with manual verification, enrollment → payroll deduction. Claims tracking deliberately out of scope; it lives with the insurance provider.
  Depends on: scheduler (window open/close, dependent aging), notification, effective-dated enrollments.

**Operations**

- **Asset Management** — categories with a `requires_return` flag, asset instances, assignment history (current holder is a derived query, never a stored column), maintenance log, requests, software license seats as a separate shape. Depreciation stays a stub here — book value only; real fixed-asset accounting belongs to the ACCOUNTING MODULE.
  Reuses `hrm_acknowledgements` for handover sign-off. Feeds exit clearance and payroll recovery.

- **Travel & Expense** — travel requests, itineraries, advances, expense claims with **line-level** approval (`amount` vs `approved_amount` per line), policy violations recorded as warnings not hard blocks, per-diem and effective-dated mileage rates. Multi-currency is unavoidable here regardless of when multi-country lands. OCR is a vendor bolt-on; leave a nullable column and do manual entry first.
  Boundary: claim lifecycle here, payout via payroll in compensation.

- **HR Helpdesk** — employee-facing tickets with SLA (clock pausable), internal-only comments, sensitive categories with a restricted assignee pool, knowledge base.
  ⚠️ Two boundaries: distinct from `hrm_complaints` (formal disciplinary complaints, legal weight) with a one-way convert path; and distinct from the customer-facing Ticketing/Helpdesk noted in the CRM entry above. Same data shape as that one, which is an argument for building it generic in `internal/platform/` rather than inside HRM — an open architectural fork, not a decision.
  Reuses the capture inbound-email pipeline for email-to-ticket.

- **Exit Management** — upgrade over the existing resignations/terminations decision records: an umbrella exit record, notice period tracking, exit interviews (confidential, aggregate-only, often sent post-departure), clearance checklists, F&F settlement, document issuance, rehire eligibility. Access revocation on last working date is scheduler-driven, not manual.
  F&F is an off-cycle payroll run (`run_type = 'fnf'`), not a separate calculator — same line types, same statutory engine, same immutability. It pulls from seven modules; negative net is a valid outcome and must be handled. Clearance blocking items gate finalization.
  Depends on: checklist engine (consumer #2), form engine, scheduler.

**Insight**

- **Org Chart & reporting structure** — effective-dated reporting relationships in their own table (not a column on employees), supporting matrix reporting via `relationship_type`. Cycle detection required. Recursive CTE first; optimize only on proven need. Position/seat modeling left nullable so vacant-seat → requisition can be retrofitted.
  Frontend is a genuine visualization (d3-hierarchy / react-flow) with lazy expand — the Collection View Pattern does not apply.

- **Succession / Talent Management** — 9-box (potential assessed separately from performance, never derived from it), critical positions, succession plans with readiness levels, signal-based flight risk (explainable indicators, not an ML score), individual development plans.
  ⚠️ Confidentiality here exceeds salary: 9-box position, flight risk, and successor nomination are never visible to the subject, while their development plan is — field-level filtering within a single record, not module-level RBAC.

- **People Analytics** — not a module, a consumer. Its ceiling is set entirely by whether the twelve above store structured, temporal, enum-typed data. Nightly snapshot + fact tables in the same Postgres, never live aggregation over OLTP. `hrm_metric_definitions` is the entry that looks skippable and is not: two dashboards disagreeing on "attrition rate" ends trust in the whole module permanently. Compensation analytics, DEI (aggregate-only, threshold-gated, country-configurable), and export are separately permissioned. Predictive attrition scoring is deliberately excluded.
  Depends on: scheduler (this is where it becomes non-negotiable), EMAIL SENDING (scheduled report delivery is what makes analytics actually get read).

**Cross-cutting**

- **Multi-country / Multi-currency** — not a module: a legal-entity layer between organization and employee, which re-scopes payroll runs, statutory resolution, approval chains, and every analytics view. Plus country-configurable working week, leave minimums, notice/termination law, name format, address schema, and ID types. Timezone-correct attendance attribution is a policy decision, not a storage one.
  Data residency (separate DB instance or region per jurisdiction) is explicitly out of scope and conflicts with the current single-Postgres deployment model — that, not schema, is where "multi-country SaaS" actually hits a wall.
  See PREP MIGRATIONS above: `legal_entity_id` and per-column currency are cheap today.

**Answers the original question this scoping started from:** the shipped HRM is entirely internal-facing (authenticated JWT entry only), which is why it needs no CRM-Capture equivalent. Recruitment/ATS is the one entry above that changes that — a public career page and application endpoint (`/pub/careers/*`) is the same shape as CRM capture: public routes, rate limiting, file validation, `LOWER(email)` dedup, and job-board webhooks if those are ever added. Whatever Capture Fix Pass A/B teaches transfers directly. Nothing else in HRM has an anonymous entry point. Attendance hardware push and payroll bank disbursement are integrations, but neither is inbound-webhook shaped.

**Vendor boundaries decided during scoping (consistent across all of the above):** resume parsing, receipt OCR, SCORM/video hosting, external certification verification, statutory filing submission, and predictive scoring are all either vendor bolt-ons or out of scope. The pattern: store the raw artifact, leave a nullable column for extracted data, add the vendor later. Do not build the extraction engine.

**If any part of this is picked up:** promote it here (⚪ → 🔵), add a real Section 5 entry with routes and permissions, and a Section 8 entry when frontend starts. Do not let this entry grow into the module documentation — it is a map, and a map that tries to be the territory goes stale first.

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

### MOBILE APP [🔵 ACTIVE — real code exists; doc previously said "zero code written," which was false as of 2026-08-03]

**This entry was wrong.** `mobile/` was committed 2026-07-23 (`a350092`, "Mobile added," 93 files,
+5799/-144) — one day after r14 (2026-07-22) dated the "zero code" claim, so the commit simply
never got folded back into the doc. Verified directly against source, not against the r10/r14
plan. See Section 14 for the architecture-vs-actual diff and Section 15 for per-screen status.

**What's real:** full Expo Router app under `mobile/src/`, real dependencies (`expo ~57.0.8`,
`react-native 0.86.0`, `expo-router ~57.0.8`, `expo-secure-store ~57.0.1`, `zustand ^5.0.14`,
TanStack Query, axios, react-hook-form/zod), `node_modules` installed. Auth screens (login,
signup, forgot-password, reset-password), onboarding (create-organization, select-organization),
dashboard shell, tasks, a single-screen tabbed CRM view, a flat settings screen, real Zustand
stores (`authStore`, `permissionStore`, `uiStore`), real `expo-secure-store` token handling, real
API client hitting the actual backend (`api.post('/auth/mobile/login', ...)`,
`api.get('/organizations/:orgId/crm/leads')`).

**What's stub or missing:** `components/{crm,tasks,rbac}/` are empty directories, tracked as
nothing in git. `useAuth.ts`'s `forgotPassword`/`resetPassword` are `console.log` + fake
`setTimeout` — the *pages* exist and the backend routes exist, but the mobile hook doesn't call
them yet. No `security/` route at all. CRM is one screen with an internal tab switcher (Leads /
Pipeline / Reports / Agenda / Setup), not split into per-entity routes the way web is. Settings is
a single flat screen — no members/roles subroutes.

**Structural deviations from the original plan** (not wrong, just different — update Section 14
to match rather than "fix" the code to match a plan written before this was built): code lives
under `mobile/src/...`, not bare `mobile/...`; there's an extra `apps/` wrapper route
(`(dashboard)/[orgId]/apps/{index,crm}`) and an `alerts/` route neither one was planned for.

Suggested next step, given the above: wire the two stub password-recovery calls in `useAuth.ts` to
the real backend routes (`AUTH — MOBILE` already supports `password-reset/*` — no backend work
needed), then decide whether to keep CRM as one tabbed screen or split it, before building
anything further on top of it.

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
- **Money is `NUMERIC(18,4)` in Postgres and `shopspring/decimal` in Go — never `float64`** (r15). Every money column carries a currency alongside it from day one. Rounding policy (level and mode) is an explicit org setting, not an implicit language default. Verify the existing salary formula engine's evaluation type before extending it — `expr` defaults to float.

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
| shopspring/decimal  | v1.4.0 (added 00066, HRM money fields — Section 10)            |
| robfig/cron/v3      | v3.0.1 (added 00067, `platform/scheduler`)                     |
| resend/resend-go/v2 | v2.28.0 (added 00068, `platform/notifications` email channel)  |
| Next.js             | 16.2.9 (check if unsure)                                       |
| Tailwind CSS        | v4                                                             |
| React               | latest stable                                                  |
| Zustand             | v5.0.14                                                        |
| next-themes         | latest stable                                                  |
| React Hook Form     | v7                                                             |
| TanStack Query      | v5                                                             |
| Axios               | v1                                                             |
| Expo SDK            | 57.0.8 confirmed (`mobile/package.json`, real app exists — see Section 9 → MOBILE APP) |
| Expo Router         | ~57.0.8, bundled with Expo SDK 57                               |
| React Native        | 0.86.0 confirmed (`mobile/package.json`)                        |
| expo-secure-store   | ~57.0.1 confirmed (`mobile/package.json`)                       |

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

**Corrected 2026-08-03 against the real `mobile/` tree** (see Section 9 → MOBILE APP) — the
structure below is what was actually built, not the original r10/r14 plan. Differences from that
plan: everything lives under a `src/` root (not bare `mobile/app/...`); there's an `apps/` wrapper
route and an `alerts/` route neither one was planned for; CRM is one screen with an internal tab
switcher, not split into per-entity routes; settings is flat, no members/roles subroutes; there is
no `security/` route at all.

### Folder Structure

```
mobile/
  src/
    app/                        ← Expo Router file-based routes
      (auth)/
        login.tsx               ← real
        signup.tsx               ← real
        forgot-password.tsx      ← real page; hook behind it is a stub (see below)
        reset-password.tsx       ← real page; hook behind it is a stub (see below)
        _layout.tsx
      (dashboard)/
        _layout.tsx
        [orgId]/
          _layout.tsx
          index.tsx              ← dashboard home, real
          tasks/index.tsx        ← real
          apps/                  ← not in the original plan
            index.tsx
            crm/index.tsx        ← single screen, internal tabs: Leads/Pipeline/Reports/Agenda/Setup
          alerts/index.tsx       ← not in the original plan
          settings/              ← flat, no members/ or roles/ subroutes
            _layout.tsx
            index.tsx
      create-organization.tsx    ← real
      select-organization.tsx    ← real
      index.tsx
      _layout.tsx                ← root layout: theme provider + auth gate
    components/
      ui/  layout/                ← real
      crm/  tasks/  rbac/          ← empty directories, nothing tracked in git
    lib/
      api.ts  auth.ts  secureToken.ts  constants.ts   ← all real
      crmApi.ts  tasksApi.ts  orgApi.ts  dashboardApi.ts   ← not in the original plan, all real
    stores/
      authStore.ts  permissionStore.ts  uiStore.ts    ← real, same shape discipline as web
    hooks/
      useAuth.ts                 ← real for login/signup/logout; forgotPassword/resetPassword are
                                    console.log + fake setTimeout, no real API call
      usePermission.ts  useOrganization.ts             ← real (doc previously said `useOrg.ts`)
    theme/
      tokens.ts  ThemeProvider.tsx                     ← real
    types/
      index.ts                    ← real, single file (doc previously implied per-domain files
                                     mirroring frontend/src/types/*; it's one file instead)
```

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

---

## 15. MOBILE MODULE REGISTRY

Same status convention as Section 5/8/9. **This section previously said "nothing built yet" —
false as of 2026-08-03; see Section 9 → MOBILE APP for the full correction.** Statuses below are
verified directly against `mobile/src/`, not against the v1 plan this registry originally
described.

Scope note (added r14): this v1 list predates several web features shipped since r10 — Agenda,
Setup/Routing, Templates, Capture. Capture/Agenda status per-screen is noted inline below where
mobile happens to already cover them (folded into the single CRM screen); the rest still isn't
represented here — decide inclusion explicitly when next revisited.

---

### AUTH SCREENS [🔵 ACTIVE — pages built, two hooks still stubbed]

Login and signup screens are real and call the real backend (`AUTH — MOBILE`, Section 5). Forgot
password and reset password *pages* exist (`(auth)/forgot-password.tsx`, `reset-password.tsx`),
but the hook behind them (`useAuth.ts`) is `console.log` + a fake `setTimeout` — no real API call
yet, even though the backend routes they'd need already exist. This is the one clear next step
tracked in the Section 9 entry.

---

### ONBOARDING [✅ DONE]

`create-organization.tsx` and `select-organization.tsx` are real, top-level routes — shown after
login when no org context exists, same as web.

---

### DASHBOARD SHELL [✅ DONE]

Root and `(dashboard)` layouts, org-scoped `[orgId]` layout, dashboard home (`index.tsx`) all real.
Org switcher / profile-menu mechanics not independently re-verified beyond confirming the files
are real and non-trivial (172 lines for the dashboard home alone).

---

### TASKS [✅ DONE]

`tasks/index.tsx` (195 lines) is real, hits the same `tasks.*`-permissioned backend routes as web
via `lib/tasksApi.ts`.

---

### CRM — LEADS & PIPELINE [🔵 PARTIAL — exists, but as one tabbed screen, not split routes]

`apps/crm/index.tsx` (263 lines) is a single screen with an internal tab switcher covering Leads,
Pipeline, Reports, Agenda, and Setup — not the per-entity route split (`leads/`, `pipeline/`, etc.)
this registry originally called for. Real data, real API calls (`lib/crmApi.ts`), just a different
information architecture than planned. Decide whether to keep this shape or split it before
building further on top.

---

### CRM — CONTACTS [⚪ NOT STARTED]

No Contacts or Companies tab was found in the single CRM screen above, and no separate route
exists — still genuinely not built, unlike Leads/Pipeline/Reports/Agenda which got folded in.

---

### CRM — REPORTS [🔵 PARTIAL — folded into the CRM tab screen, not a dedicated summary-cards screen]

A "Reports" tab exists inside `apps/crm/index.tsx` rather than the standalone summary-cards screen
originally planned. Real data via `lib/crmApi.ts`; not independently verified whether it's cards
or full charts — worth a direct look before assuming either way.

---

### SETTINGS — MEMBERS, ROLES & PERMISSIONS [⚪ NOT STARTED — lower priority]

`settings/index.tsx` is a flat profile-style screen; it lists "Members" as a menu item but there is
no `settings/members/` or `settings/roles/` route behind it — the label exists, the screen doesn't.
Admin-heavy screens, arguably fine to stay web-only for v1. Revisit after the above ships.

---

### SECURITY [⚪ NOT STARTED — lower priority]

Session list, revoke, login events. Likely fine as web-only initially too.

---

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

When a scoping entry is written for unbuilt work (r15):

- It goes in Section 9 as `⚪ NOT STARTED`, carries no priority ordering and no effort estimate, and stays a map — the moment work starts, the real detail moves to a Section 5/8 entry rather than growing inside Section 9.
- If the same shared pattern is identified across three or more independent scoping passes, record it in PLATFORM PRIMITIVES instead of repeating it per module — but do not build it until a real consumer exists.

Periodic structural drift audit:

- Before any major "update the docs" pass, don't trust the document's own status flags as ground truth — grep the actual source for route counts, migration counts, and file existence, then reconcile. This document has been wrong about counts and statuses in every single revision audit so far (r9: HRM/deployment; r11: four whole shipped features missing; r15: the copy being edited was a full revision behind on mobile). Assume drift by default.
- Cross-check Section 5 (backend) against Section 8 (frontend) against Section 9 (unbuilt module registry) for the same module.
- A doc pasted into a chat conversation is not guaranteed to be the committed `docs/Project_Instruction.md` — whichever copy is more recent should overwrite the other after an update.

Keep this document current. A stale instruction is worse than no instruction.
