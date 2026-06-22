# Module: Organizations + RBAC

## What this module does

Organizations (also called businesses or workspaces) are the multi-tenancy unit of BusinessSAAS.
Every piece of data — leads, tasks, deals, members — belongs to one organization.

RBAC (Role-Based Access Control) defines what each member of an organization can do.

## Backend endpoints

### Organizations

| Method | Path | Auth | Description |
|--------|------|:----:|-------------|
| POST | `/api/v1/organizations` | JWT | Create new org |
| GET | `/api/v1/organizations` | JWT | List orgs for current user |
| GET | `/api/v1/organizations/:id` | JWT | Get one org |
| POST | `/api/v1/organizations/:id/switch` | JWT | Get a scoped access token for this org |

### Members

| Method | Path | Auth | Permission |
|--------|------|:----:|------------|
| GET | `/api/v1/organizations/:orgId/members` | JWT | `members.view` |
| GET | `/api/v1/organizations/:orgId/members/me` | JWT | (self) |
| POST | `/api/v1/organizations/:orgId/members/invite` | JWT | `members.invite` |
| GET | `/api/v1/organizations/:orgId/members/:memberId` | JWT | `members.view` |
| PATCH | `/api/v1/organizations/:orgId/members/:memberId/role` | JWT | `members.update` |

### Roles + Permissions

| Method | Path | Auth | Permission |
|--------|------|:----:|------------|
| GET | `/api/v1/roles` | JWT | (any) |
| GET | `/api/v1/permissions` | JWT | (any) |
| GET | `/api/v1/organizations/:orgId/rbac/roles` | JWT | `roles.view` |
| POST | `/api/v1/organizations/:orgId/rbac/roles` | JWT | `roles.create` |
| PATCH | `/api/v1/organizations/:orgId/rbac/roles/:roleId/permissions` | JWT | `roles.permissions.update` |
| GET | `/api/v1/organizations/:orgId/rbac/permissions/matrix` | JWT | `roles.view` |
| POST | `/api/v1/organizations/:orgId/rbac/check` | JWT | `roles.view` |

## Frontend pages

```
app/(app)/[orgSlug]/
├── settings/
│   ├── members/page.tsx    → list members, invite, change role
│   └── roles/page.tsx      → list roles, permission matrix
```

## The org switch flow

This is the most important concept in multi-tenancy:

```
1. User logs in → access token has no org context (bid: "")
2. User sees org list → selects an org
3. POST /organizations/:id/switch
4. Backend issues a NEW access token with bid + role embedded
5. Frontend stores new access token in memory (replaces old one)
6. All subsequent API calls carry the org-scoped token
7. URL changes to /app/[orgSlug]/dashboard
```

Without step 3-5, all org-scoped API calls return 400 "NO_BUSINESS_CONTEXT".

## Permission system

```
User → Membership → Role → RolePermissions → Permission keys
```

### System roles (seeded, cannot be deleted)

| Role | Key | Who it is |
|------|-----|-----------|
| Owner | `owner` | Created the org; has all permissions |
| Admin | `admin` | Can manage members and most settings |
| Member | `member` | Standard user; can create and edit |
| Viewer | `viewer` | Read-only across all modules |

### Permission key convention

```
<module>.<resource>.<action>

Examples:
  tasks.view
  tasks.create
  tasks.update
  tasks.delete
  crm.leads.view
  crm.leads.create
  crm.leads.convert
  members.view
  members.invite
  roles.update
  security.sessions.view
```

The backend uses a last-dot-split: `crm.leads.view` → resource=`crm.leads`, action=`view`.

### Frontend permission check

```typescript
// hooks/usePermission.ts
export function usePermission(permission: string): boolean {
  const { data: membership } = useMyMembership()
  return membership?.permissions.includes(permission) ?? false
}

// Usage in a component:
const canDelete = usePermission('tasks.delete')

return (
  <Button
    disabled={!canDelete}
    onClick={() => deleteTask(task.id)}
  >
    Delete
  </Button>
)
```

Permission checks in the frontend are for UX only — the backend enforces them on every request.

## Zustand state for org context

```typescript
// store/org.ts
{
  activeOrgSlug: string | null
  activeOrgId: string | null
  activeOrgName: string | null
  setActiveOrg: (org: { slug: string; id: string; name: string }) => void
}
```

The `orgId` is resolved once per session switch and stored here. TanStack Query keys use
`orgId` for cache scoping.

## Error codes

| Code | HTTP | When |
|------|------|------|
| `NO_BUSINESS_CONTEXT` | 400 | JWT has no org context — must call `/switch` first |
| `NOT_A_MEMBER` | 403 | User is not a member of the requested org |
| `BUSINESS_NOT_FOUND` | 404 | Org doesn't exist (or user has no access — same response to prevent enumeration) |
| `FORBIDDEN` | 403 | Permission check failed |

## Related ADRs

- [ADR-0008](../decisions/0008-multi-tenancy-url-structure.md) — URL routing strategy
- [ADR-0003](../decisions/0003-auth-token-strategy.md) — org context embedded in JWT
