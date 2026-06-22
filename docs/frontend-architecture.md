# Frontend architecture

## Overview

The BusinessSAAS frontend is a Next.js 15 App Router application. It is a protected multi-tenant
dashboard. Every page inside the `(app)` route group requires authentication and an active
organization context.

## Complete technology stack

| Concern | Technology | ADR |
|---------|-----------|-----|
| Framework | Next.js 15 App Router | ADR-0004 |
| Language | TypeScript (strict mode) | ADR-0004 |
| UI components | shadcn/ui | ADR-0005 |
| Styling | Tailwind CSS v4 | ADR-0005 |
| Dark/light mode | next-themes | ADR-0013 |
| Authentication | next-auth v5 + Go backend | ADR-0006 |
| Token storage | Memory only (JS variable) | ADR-0006 |
| Server state | TanStack Query v5 | ADR-0007 |
| Client state | Zustand | ADR-0007 |
| HTTP client | ky v2 | ADR-0006 |
| Forms | React Hook Form + Zod | ADR-0009 |
| Tables | TanStack Table v8 | ADR-0011 |
| Charts | Recharts | — |
| Toasts | Sonner | ADR-0012 |
| Error boundaries | react-error-boundary | ADR-0012 |
| URL state | nuqs | — |
| Date utilities | date-fns | — |
| Package manager | Bun | ADR-0010 |
| Multi-tenancy | Path-based `/app/[orgSlug]/` | ADR-0008 |

## Folder structure

```
frontend/
├── app/
│   ├── layout.tsx               ← root layout: ThemeProvider, QueryProvider, Toaster
│   ├── globals.css              ← Tailwind v4 @theme tokens, base styles
│   ├── (auth)/                  ← public routes (no dashboard shell)
│   │   ├── login/page.tsx
│   │   ├── signup/page.tsx
│   │   └── reset-password/page.tsx
│   └── (app)/                   ← protected routes
│       ├── layout.tsx           ← auth check, dashboard shell
│       ├── page.tsx             ← redirect to /app/[first-org]/dashboard
│       └── [orgSlug]/
│           ├── layout.tsx       ← org context provider, permission loader
│           ├── dashboard/page.tsx
│           ├── crm/
│           │   ├── page.tsx
│           │   ├── contacts/page.tsx
│           │   ├── leads/page.tsx
│           │   └── deals/page.tsx
│           ├── tasks/page.tsx
│           ├── settings/
│           │   ├── members/page.tsx
│           │   └── roles/page.tsx
│           └── hrm/             ← future
│
├── components/
│   ├── ui/                      ← shadcn/ui primitives (copied in via CLI)
│   │   ├── button.tsx
│   │   ├── input.tsx
│   │   ├── dialog.tsx
│   │   ├── data-table.tsx
│   │   └── ... (added as needed)
│   ├── layout/                  ← shell components
│   │   ├── Sidebar.tsx
│   │   ├── Topbar.tsx
│   │   ├── OrgSwitcher.tsx
│   │   ├── ThemeToggle.tsx
│   │   └── Breadcrumb.tsx
│   └── shared/                  ← used across multiple features
│       ├── DataTable.tsx        ← TanStack Table wrapper
│       ├── PageHeader.tsx       ← title + breadcrumb + action slot
│       ├── EmptyState.tsx       ← empty list illustration + CTA
│       ├── LoadingSpinner.tsx
│       └── ModuleErrorBoundary.tsx
│
├── features/                    ← feature-specific components
│   ├── auth/
│   │   ├── LoginForm.tsx
│   │   └── SignupForm.tsx
│   ├── crm/
│   │   ├── leads/
│   │   │   ├── LeadTable.tsx
│   │   │   ├── LeadForm.tsx
│   │   │   └── LeadColumns.tsx  ← TanStack Table column defs
│   │   └── deals/
│   │       ├── PipelineBoard.tsx
│   │       └── DealCard.tsx
│   ├── tasks/
│   │   ├── TaskTable.tsx
│   │   └── TaskForm.tsx
│   └── settings/
│       ├── MemberTable.tsx
│       └── RolePermissionMatrix.tsx
│
├── hooks/
│   ├── useAuth.ts               ← current user from next-auth session
│   ├── useOrg.ts                ← active org from Zustand + URL
│   ├── usePermission.ts         ← permission check against membership
│   └── useDebouncedValue.ts
│
├── lib/
│   ├── api.ts                   ← ky client, token injection, refresh logic, error normalisation
│   ├── auth.ts                  ← next-auth config (providers, callbacks)
│   ├── query-client.ts          ← TanStack Query client singleton
│   ├── utils.ts                 ← cn() helper, formatters
│   └── validations/             ← Zod schemas
│       ├── auth.ts
│       ├── crm/
│       └── tasks.ts
│
├── store/
│   ├── auth.ts                  ← Zustand: current user
│   ├── org.ts                   ← Zustand: active org slug + id
│   └── ui.ts                    ← Zustand: sidebar state
│
├── types/
│   ├── api.ts                   ← ApiError, PaginatedResponse, ApiResponse
│   └── domain.ts                ← User, Org, Lead, Deal, Task, Member, Role, Permission
│
└── middleware.ts                 ← route protection, org slug validation
```

## Rendering strategy

| Page type | Strategy | Why |
|-----------|----------|-----|
| Login/signup | Client Component | Form interactions |
| Dashboard home | Server Component | No interaction, show stats |
| List pages (leads, tasks) | Server prefetch + Client | Initial data from server, mutations on client |
| Detail pages | Server Component + Client form | Pre-render content, client for edits |
| Settings | Client Component | Frequent mutations |

Rules:
- Default to Server Components — add `"use client"` only when needed
- Needs required: event handlers, `useState`, `useEffect`, browser APIs, hooks
- Server Components can await data — use this for initial page loads

## Data flow

```
Server Component (page.tsx)
  └── prefetches initial data via fetch() with Authorization header
  └── passes to Client Component as props or via React cache

Client Component
  └── receives initial data as props
  └── uses useQuery() for subsequent fetches, mutations
  └── uses TanStack Query cache — no re-fetch on first render if data provided
```

## Middleware protection

```typescript
// middleware.ts
export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  // Public routes
  if (pathname.startsWith('/login') || pathname.startsWith('/signup')) {
    return NextResponse.next()
  }

  // Protected routes
  const session = await auth()  // next-auth v5
  if (!session) {
    return NextResponse.redirect(new URL('/login', request.url))
  }

  // Org validation for /app/[orgSlug]/* routes
  const orgSlugMatch = pathname.match(/^\/app\/([^/]+)/)
  const orgSlug = orgSlugMatch?.[1]
  if (orgSlug && session.user.orgs && !session.user.orgs.includes(orgSlug)) {
    return NextResponse.redirect(new URL('/app', request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico|api/auth).*)'],
}
```

## Adding a new module

When HRM (or any new module) is added:

1. Create `app/(app)/[orgSlug]/hrm/` with page files
2. Add the module's API functions to `lib/api/hrm.ts`
3. Add TanStack Query keys to `lib/query-keys.ts`
4. Add Zod schemas to `lib/validations/hrm/`
5. Add feature components to `features/hrm/`
6. Add sidebar nav item in `components/layout/Sidebar.tsx`
7. Add permissions to the module doc under `docs/modules/hrm.md`

The module is isolated — it does not touch other modules' files.

## Environment variables

```bash
# .env.local
NEXT_PUBLIC_API_URL=http://localhost:8080       # browser-side API calls
BACKEND_INTERNAL_URL=http://backend:8080        # server-side calls inside Docker
NEXTAUTH_SECRET=...                             # next-auth encryption key
NEXTAUTH_URL=http://localhost:3000              # full URL for next-auth callbacks
NEXT_PUBLIC_APP_NAME=BusinessSAAS
```
