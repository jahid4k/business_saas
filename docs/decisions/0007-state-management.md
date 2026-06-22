# ADR-0007: State management — TanStack Query v5 + Zustand

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

A SaaS dashboard has two fundamentally different kinds of state that must be managed correctly:

1. **Server state** — data that lives on the backend: leads, deals, tasks, members, roles.
   It is fetched asynchronously, can be stale, needs to be refetched, and can be mutated.
   Multiple components may display the same server data simultaneously.

2. **Client state** — data that lives only in the browser: whether the sidebar is collapsed,
   the current theme preference, which modal is open, temporary form values.
   It never needs to be persisted to the server.

Mixing these two concerns into a single state management solution (like Redux) causes problems:
async data needs a loading state, an error state, a cache, and a refetch strategy — none of
which apply to a simple boolean like `isSidebarOpen`.

---

## Decision

Use **TanStack Query v5** for all server state.
Use **Zustand** (minimally) for client state that needs to be shared across components.
Use plain `useState` for local component state.

Do **not** use Redux, MobX, Recoil, Jotai, or any other global state library.

---

## Reasoning

### Why TanStack Query for server state

TanStack Query (formerly React Query) treats server data as a first-class concern:

**Automatic caching:** A query result for `['leads', orgId]` is cached. If two components on
the same page both need the leads list, only one network request is made.

**Background refetching:** When the user focuses the window after switching tabs, stale data
is automatically refreshed in the background. No stale dashboards.

**Optimistic updates:** When a user marks a task done, the UI updates immediately. If the server
call fails, it rolls back automatically. This makes the UI feel instant.

**Pagination and infinite scroll:** Built-in `useInfiniteQuery` for lists that load on scroll.

**Deduplication:** Simultaneous calls for the same query are deduplicated into one request.

**Invalidation:** After creating a lead, `queryClient.invalidateQueries(['leads'])` triggers
an automatic refetch of the list. No manual state updates needed.

**TanStack Query v5 specific:** The v5 API uses a single `useQuery` options object (no positional
args), making query definitions cleaner and more tree-shakeable.

### Why Zustand for client state

Zustand has the smallest API surface of any state library:

```typescript
const useUIStore = create<UIState>((set) => ({
  sidebarCollapsed: false,
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),

  activeOrgSlug: null,
  setActiveOrgSlug: (slug: string) => set({ activeOrgSlug: slug }),
}))
```

That is the entire store. No actions, no reducers, no boilerplate. Any component calls
`useUIStore((s) => s.sidebarCollapsed)` and re-renders only when that value changes.

Zustand is used **only** for UI state that:
- Must be shared between components that are not in a parent-child relationship
- Does not need to be persisted to the server

Examples of Zustand state:
- Sidebar collapsed/expanded
- Active organization slug (for URL construction)
- Current theme (though next-themes handles this — see ADR-0013)

Examples of state that stays local (`useState`):
- Whether a modal is open (unless triggered from a non-parent)
- Input values inside a form (React Hook Form handles this — see ADR-0009)

---

## Rules

1. **Never put server data in Zustand.** If it comes from an API, it belongs in TanStack Query.
2. **Never put UI toggles in TanStack Query.** If no network call is involved, use `useState` or Zustand.
3. **Prefer `useState` over Zustand** unless the state genuinely needs to cross component boundaries.
4. **Query keys are arrays.** Always structure them as `[domain, orgId, resource, params]`:
   ```typescript
   ['crm', orgId, 'leads']
   ['crm', orgId, 'leads', { status: 'new', page: 1 }]
   ['tasks', orgId, taskId]
   ```
   This enables fine-grained invalidation after mutations.

---

## Query key convention

```typescript
// keys.ts — centralised query key factory
export const queryKeys = {
  crm: {
    leads: (orgId: string) => ['crm', orgId, 'leads'] as const,
    lead: (orgId: string, id: string) => ['crm', orgId, 'leads', id] as const,
    deals: (orgId: string) => ['crm', orgId, 'deals'] as const,
  },
  tasks: {
    list: (orgId: string) => ['tasks', orgId] as const,
    one: (orgId: string, id: string) => ['tasks', orgId, id] as const,
  },
  members: {
    list: (orgId: string) => ['members', orgId] as const,
  },
}
```

Centralising keys prevents typos and makes invalidation predictable.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Redux Toolkit | Massive boilerplate for async data; requires RTK Query on top (which is TanStack Query but worse) |
| Redux Toolkit + RTK Query | Two libraries doing the job of TanStack Query alone; adds 30KB |
| SWR | Good but TanStack Query v5 has better mutation handling, optimistic updates, and devtools |
| Recoil / Jotai | Atom-based — fine for client state but less suited to server state with cache invalidation |
| Context API only | Good for small apps; at SaaS scale, context re-renders become a performance problem |
| MobX | Observable model is powerful but heavy; reactivity model is hard to trace in code review |

---

## Consequences

**Positive:**
- Server data is always fresh — automatic background refetch
- Optimistic updates make the UI feel instant
- Cache deduplication reduces API calls significantly
- Devtools (TanStack Query Devtools) make cache state visible during development
- Zustand's minimal API means zero boilerplate overhead

**Negative:**
- TanStack Query v5 has a slightly different API than v4 — some tutorials reference v4 syntax
- Developers must understand the cache invalidation model to avoid stale data bugs
- Two state management tools to learn (though they serve completely different purposes)

---

## Related decisions

- [ADR-0006](0006-token-storage-strategy.md) — access token in memory; TanStack Query respects this by not storing tokens
- [ADR-0009](0009-form-validation.md) — React Hook Form for form state (not Zustand)
