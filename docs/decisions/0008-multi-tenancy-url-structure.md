# ADR-0008: Multi-tenancy URL structure — path-based routing

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS is a multi-tenant SaaS platform. Each organization (tenant) has its own data
and its own members with their own permissions. When a user navigates the dashboard, the URL
must clearly identify which organization's data is being shown.

Two common multi-tenancy URL approaches exist in production SaaS:
1. **Path-based:** `app.businesssaas.com/app/acme-corp/crm/leads`
2. **Subdomain-based:** `acme-corp.businesssaas.com/crm/leads`

The URL structure determines how Next.js routes are defined, how `middleware.ts` extracts the
org context, how API calls are constructed, and how the browser handles cookies.

---

## Decision

Use **path-based multi-tenancy** with the organization slug as the first dynamic segment
after the app prefix:

```
/app/[orgSlug]/dashboard
/app/[orgSlug]/crm/leads
/app/[orgSlug]/crm/deals
/app/[orgSlug]/tasks
/app/[orgSlug]/settings/members
/app/[orgSlug]/settings/roles
/app/[orgSlug]/hrm/employees        (future)
```

The `orgSlug` is the human-readable identifier for the organization (e.g. `acme-corp`).
The org UUID (`orgId`) is resolved from the slug during navigation and stored in Zustand.

---

## Reasoning

### Simplicity and reliability

Path-based routing works on any domain, any hosting, any environment. No DNS wildcard setup.
No wildcard SSL certificate. No nginx subdomain routing configuration. It works in development
on `localhost:3000` with zero configuration.

Subdomain routing requires:
- A wildcard DNS record (`*.businesssaas.com → load balancer`)
- A wildcard SSL certificate (`*.businesssaas.com`)
- Next.js middleware to parse the subdomain from `request.headers.get('host')`
- Cookie domains set to `.businesssaas.com` (not the default)
- CORS configured for every possible subdomain

All of this can be done, but it is significant infrastructure work that adds no product value
at the current scale.

### Next.js App Router native support

The `app/(app)/[orgSlug]/` directory maps perfectly to the URL structure. Next.js handles
parameter extraction automatically. No custom routing logic needed.

```
app/
└── (app)/
    └── [orgSlug]/
        ├── dashboard/
        │   └── page.tsx       → /app/acme-corp/dashboard
        ├── crm/
        │   └── leads/
        │       └── page.tsx   → /app/acme-corp/crm/leads
        └── tasks/
            └── page.tsx       → /app/acme-corp/tasks
```

### Middleware integration

`middleware.ts` extracts `orgSlug` from the URL and validates the session includes access
to that organization — all without subdomain parsing:

```typescript
// middleware.ts
export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  // Extract orgSlug from path
  const orgSlugMatch = pathname.match(/^\/app\/([^/]+)/)
  const orgSlug = orgSlugMatch?.[1]

  const session = await getSession(request)

  if (!session) {
    return NextResponse.redirect(new URL('/login', request.url))
  }

  // Validate session has access to this org
  if (orgSlug && !session.orgs.includes(orgSlug)) {
    return NextResponse.redirect(new URL('/app', request.url))
  }

  return NextResponse.next()
}
```

### Org switcher behaviour

When a user switches organizations, navigation changes the `[orgSlug]` segment:

```typescript
// User switches from acme-corp to techcorp
router.push(`/app/${newOrgSlug}/dashboard`)
```

TanStack Query automatically invalidates all queries scoped to the previous org (because query
keys include `orgId`). No manual cleanup required.

---

## URL → API mapping

The frontend URL uses the org slug (human-readable). The backend API uses the org UUID.
Resolution happens once per session switch:

```
Frontend: /app/acme-corp/crm/leads
  ↓ resolve slug to UUID (from session or one API call)
Backend:  GET /api/v1/organizations/019e7f92-.../crm/leads
```

The slug-to-UUID mapping is cached in Zustand for the session lifetime.

---

## Future subdomain migration path

If BusinessSAAS grows to where subdomains become a product requirement (e.g. for white-labelling),
the migration path is:

1. Configure wildcard DNS and SSL
2. Add `X-Forwarded-Host` parsing in `middleware.ts`
3. Keep all routes exactly the same — just change how `orgSlug` is extracted (from path or subdomain)
4. Update cookie domain

The internal routing logic and all API calls remain unchanged. Path-based first makes this
migration cheap.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Subdomain-based (`acme.businesssaas.com`) | Requires wildcard SSL, DNS config, complex middleware; no product benefit yet |
| Query param (`?org=acme-corp`) | Not bookmarkable as org-specific; URL doesn't communicate context clearly |
| User selects org at login, no URL segment | Single active org per session; doesn't support quick org switching; breaks deep links |
| Numeric org ID in URL (`/app/12345/`) | Ugly, exposes internal IDs, not human-readable |

---

## Consequences

**Positive:**
- Zero infrastructure requirement to implement
- Works on `localhost:3000` without any configuration
- Deep links work: `/app/acme-corp/crm/leads?leadId=xyz` is bookmarkable and shareable
- TanStack Query cache keys naturally scope to org (they include `orgId`)
- Clear, readable URLs improve user trust

**Negative:**
- URLs are slightly longer than subdomain approach
- Changing org slug (if the org renames) requires redirects from old slug
- No visual brand isolation per tenant (subdomain can show custom branding) — not a requirement now

---

## Related decisions

- [ADR-0004](0004-frontend-framework.md) — Next.js App Router; `[orgSlug]` is a native dynamic segment
- [ADR-0007](0007-state-management.md) — Zustand stores the resolved `orgId` from the slug
