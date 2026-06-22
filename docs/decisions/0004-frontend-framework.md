# ADR-0004: Frontend framework — Next.js 15 App Router

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS needs a frontend that:

- Serves a complex, permission-gated dashboard with many modules (CRM, HRM, Tasks, Settings)
- Performs well at millions of users — initial load must be fast, client-side transitions snappy
- Supports both server-side and client-side rendering patterns depending on the page
- Integrates cleanly with next-auth v5 for authentication
- Can be maintained by one developer now and scaled to a team later
- Works inside Docker Compose for local development, deploys as a Node.js server in production

---

## Decision

Use **Next.js 15** with the **App Router** architecture.

TypeScript is mandatory — `strict: true` in `tsconfig.json`. No JavaScript files in the frontend.

---

## Reasoning

### Why Next.js

Next.js is the most mature full-stack React framework with the largest ecosystem. It handles:
- Routing (file-based, with nested layouts)
- Server-side rendering and static generation
- API routes (used minimally — mainly as a proxy layer)
- Image optimization
- Font optimization
- Middleware (route protection)

All of these are needed for a production SaaS and would require separate packages or hand-rolled
solutions in a plain React app.

### Why App Router (not Pages Router)

The Pages Router is in maintenance mode. All new Next.js features are built on App Router.
Starting a new project on Pages Router creates a migration debt from day one.

App Router enables:

**React Server Components (RSC):** Components that render on the server, fetch data during render,
and ship zero JavaScript to the client. Perfect for the initial load of a dashboard page — the
HTML arrives pre-rendered with data, no loading spinner needed.

**Nested layouts:** `app/(app)/layout.tsx` defines the dashboard shell (sidebar, topbar) once.
Every module page inside `(app)/` inherits it automatically. No prop drilling, no context abuse.

**Colocation:** A CRM leads page and its data-fetching logic live in `app/(app)/crm/leads/`.
The page, loading state, and error boundary are all in the same directory.

**`middleware.ts`:** Runs at the edge before any page is served. Checks the session and redirects
unauthenticated users before they see a flash of the dashboard. This is cleaner and faster than
client-side route guards.

### Why TypeScript strict mode

`strict: true` enables:
- `noImplicitAny` — no untyped variables
- `strictNullChecks` — null/undefined must be explicitly handled
- `strictFunctionTypes` — function parameter types are checked contravariantly

At the scale of a SaaS codebase, these checks prevent entire categories of runtime bugs.
The Go backend already has compile-time safety — the frontend should too.

---

## Folder structure decided by this ADR

```
frontend/
├── app/
│   ├── (auth)/              ← public routes (login, signup, reset-password)
│   │   ├── login/page.tsx
│   │   ├── signup/page.tsx
│   │   └── reset-password/page.tsx
│   ├── (app)/               ← protected routes
│   │   ├── layout.tsx       ← dashboard shell (sidebar, topbar)
│   │   ├── [orgSlug]/       ← tenant-scoped pages
│   │   │   ├── dashboard/
│   │   │   ├── crm/
│   │   │   ├── tasks/
│   │   │   ├── hrm/
│   │   │   └── settings/
│   └── layout.tsx           ← root layout (providers, fonts, theme)
├── components/
│   ├── ui/                  ← shadcn/ui primitives
│   ├── layout/              ← Sidebar, Topbar, OrgSwitcher
│   └── shared/              ← DataTable, PageHeader, EmptyState
├── features/                ← feature-specific components
│   ├── crm/
│   ├── tasks/
│   └── settings/
├── hooks/
├── lib/
├── types/
└── middleware.ts
```

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Create React App | Unmaintained; no SSR; no file-based routing |
| Vite + React | Excellent for SPAs but no SSR, no middleware, manual setup for everything |
| Remix | Solid architecture but smaller ecosystem; Next.js has more shadcn/ui integration |
| SvelteKit | Great framework but team knowledge is in React; switching costs outweigh benefits |
| Next.js Pages Router | Maintenance mode; no Server Components; migration to App Router inevitable |
| Astro | Excellent for content sites; not optimised for highly interactive dashboard apps |

---

## Consequences

**Positive:**
- Server Components reduce client bundle size significantly
- Nested layouts eliminate layout duplication
- `middleware.ts` provides clean, fast route protection
- Excellent Vercel/Node.js deployment story
- Large community; all shadcn/ui docs target App Router

**Negative:**
- App Router has a learning curve — Server vs Client components must be understood
- RSC caching behaviour can be surprising — `cache: 'no-store'` must be set for dynamic API calls
- `"use client"` boundaries must be placed thoughtfully or RSC benefits are lost

---

## Related decisions

- [ADR-0005](0005-ui-component-library.md) — UI library choice
- [ADR-0006](0006-token-storage-strategy.md) — token storage using next-auth v5 + memory
- [ADR-0008](0008-multi-tenancy-url-structure.md) — `[orgSlug]` URL structure
