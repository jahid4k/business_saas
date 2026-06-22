# ADR-0012: Error handling — three-layer strategy

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

A production SaaS must handle errors gracefully at every level. Errors come from three sources:
1. **API failures** — network errors, validation errors, permission errors, server errors
2. **React component crashes** — uncaught JavaScript exceptions in components
3. **Next.js routing failures** — 404s, 500s during server rendering

If any of these are unhandled, the user sees a blank white screen or a cryptic browser error
message. This destroys trust.

---

## Decision

Implement a **three-layer error handling strategy**:

1. **API Error Layer** — typed error normalization in `lib/api.ts`; displayed as toasts or inline form errors
2. **React Error Boundary Layer** — catches component-level crashes; shows a friendly fallback UI
3. **Next.js Error Pages** — `error.tsx`, `not-found.tsx` for routing and server errors

---

## Layer 1: API Error normalization

All API errors are normalized into a single type before reaching any component:

```typescript
// types/api.ts
export type ApiError = {
  code: string         // 'INVALID_CREDENTIALS', 'NOT_FOUND', 'FORBIDDEN', etc.
  message: string      // Human-readable message (safe to show users)
  fields?: Record<string, string>  // Field-level validation errors
  status: number       // HTTP status code
  requestId: string    // For support — correlates to backend logs
}
```

The ky client in `lib/api.ts` catches all non-2xx responses and transforms them:

```typescript
// lib/api.ts
hooks: {
  afterResponse: [
    async (_request, _options, response) => {
      if (!response.ok) {
        const body = await response.json().catch(() => ({}))
        throw new ApiError({
          code: body.error?.code ?? 'UNKNOWN_ERROR',
          message: body.error?.message ?? 'An unexpected error occurred',
          fields: body.error?.fields,
          status: response.status,
          requestId: body.request_id ?? response.headers.get('X-Request-ID') ?? '',
        })
      }
    }
  ]
}
```

Components never inspect raw `Response` or `Error` objects — they always receive a typed `ApiError`.

### Error display rules

| Error type | Display method |
|-----------|---------------|
| Network error | Toast: "Connection lost. Check your internet." |
| 400 with `fields` | Inline form errors via `form.setError()` |
| 400 without `fields` | Toast with the error message |
| 401 | Silent refresh attempt → if fails, redirect to login |
| 403 | Inline banner: "You don't have permission to do this" |
| 404 | Inline empty state or `not-found.tsx` page |
| 409 (conflict) | Toast with the conflict message |
| 429 (rate limit) | Toast: "Too many requests. Wait a moment." |
| 500+ | Toast: "Something went wrong. Try again." + log `requestId` to console |

The `requestId` is logged to the browser console for all server errors. Support staff can use
it to correlate with backend logs.

---

## Layer 2: React Error Boundaries

React Error Boundaries catch JavaScript exceptions thrown during rendering. Without them, a single
component crash takes down the entire page.

**Module-level boundaries:** Each major section of the dashboard has its own Error Boundary.
A crash in the CRM leads table doesn't crash the sidebar or the header.

```tsx
// app/(app)/[orgSlug]/crm/leads/page.tsx
export default function LeadsPage() {
  return (
    <ModuleErrorBoundary moduleName="CRM Leads">
      <LeadsTable />
    </ModuleErrorBoundary>
  )
}
```

**Fallback UI:**

```tsx
// components/shared/ModuleErrorBoundary.tsx
export function ModuleErrorBoundary({
  moduleName,
  children
}: {
  moduleName: string
  children: React.ReactNode
}) {
  return (
    <ErrorBoundary
      fallback={
        <div className="flex flex-col items-center justify-center p-8 text-center">
          <AlertCircle className="h-10 w-10 text-destructive mb-4" />
          <h2 className="font-medium text-lg mb-2">
            {moduleName} failed to load
          </h2>
          <p className="text-muted-foreground text-sm mb-4">
            Something went wrong in this section.
          </p>
          <Button variant="outline" onClick={() => window.location.reload()}>
            Reload page
          </Button>
        </div>
      }
    >
      {children}
    </ErrorBoundary>
  )
}
```

The `ErrorBoundary` component itself is from `react-error-boundary` (a thin wrapper around
React's native `componentDidCatch` mechanism that works with functional components).

---

## Layer 3: Next.js error pages

### `error.tsx` — unhandled server errors

Placed inside route segments to catch errors during server rendering:

```tsx
// app/(app)/[orgSlug]/error.tsx
'use client'
export default function OrgError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  return (
    <div className="flex flex-col items-center justify-center min-h-screen">
      <h1 className="text-2xl font-medium mb-2">Something went wrong</h1>
      <p className="text-muted-foreground mb-6">
        {error.digest && `Error ID: ${error.digest}`}
      </p>
      <Button onClick={reset}>Try again</Button>
    </div>
  )
}
```

### `not-found.tsx` — 404 pages

```tsx
// app/not-found.tsx
export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center min-h-screen">
      <h1 className="text-2xl font-medium mb-2">Page not found</h1>
      <p className="text-muted-foreground mb-6">
        The page you're looking for doesn't exist or you don't have access.
      </p>
      <Button asChild>
        <Link href="/app">Go to dashboard</Link>
      </Button>
    </div>
  )
}
```

---

## Toast notifications

All non-inline errors show a toast using **Sonner**. Sonner is mounted once in `app/layout.tsx`:

```tsx
// app/layout.tsx
import { Toaster } from 'sonner'

export default function RootLayout({ children }) {
  return (
    <html>
      <body>
        {children}
        <Toaster position="bottom-right" richColors />
      </body>
    </html>
  )
}
```

Usage anywhere in the app:

```typescript
import { toast } from 'sonner'

toast.error('Failed to create lead', { description: error.message })
toast.success('Lead created successfully')
```

---

## Consequences

**Positive:**
- Users never see a blank white screen from any category of error
- API errors are always a typed `ApiError` — no checking `error.status` vs `error.response.status` vs `error.data.error.code`
- Module crashes are isolated — sidebar always works even if one table crashes
- `requestId` on server errors enables support without digging through logs

**Negative:**
- Error boundaries must be added explicitly to each module — easy to forget
- `react-error-boundary` adds a small dependency
- Normalizing every API error requires discipline — contributors must not bypass `lib/api.ts`

---

## Related decisions

- [ADR-0006](0006-token-storage-strategy.md) — 401 handling triggers silent refresh in the API layer
- [ADR-0009](0009-form-validation.md) — `ApiError.fields` maps to React Hook Form `setError()`
