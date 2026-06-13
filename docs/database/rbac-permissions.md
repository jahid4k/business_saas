# RBAC and Permission Documentation

## RBAC model

The current model uses permission keys as the real authorization unit. Roles are collections of permission keys. Members receive access through `organization_members.role_id`, `organization_members.role_key`, and optional `organization_members.custom_permissions`.

## Permission key format

```text
resource.action
```

Example:

```text
billing.manage
members.invite
projects.update
```

## Seeded permissions

| Permission key | Resource | Action |
| --- | --- | --- |
| dashboard.view | dashboard | view |
| organization.view | organization | view |
| organization.update | organization | update |
| members.view | members | view |
| members.invite | members | invite |
| members.update | members | update |
| members.remove | members | remove |
| roles.view | roles | view |
| roles.assign | roles | assign |
| billing.view | billing | view |
| billing.manage | billing | manage |
| subscription.view | subscription | view |
| subscription.update | subscription | update |
| projects.view | projects | view |
| projects.create | projects | create |
| projects.update | projects | update |
| projects.delete | projects | delete |
| settings.view | settings | view |
| settings.update | settings | update |
| audit_logs.view | audit_logs | view |
| api_keys.view | api_keys | view |
| api_keys.create | api_keys | create |
| api_keys.revoke | api_keys | revoke |

## Seeded system roles

| Role | Business meaning |
| --- | --- |
| owner | Full organization owner with all permissions. Includes billing.manage, members.remove, roles.assign, audit_logs.view, and API key controls. |
| admin | Broad management access but slightly less powerful than owner. Current seed does not include members.remove or billing.manage. |
| manager | Project and member visibility with project create/update but no billing or role management. |
| member | Regular organization member with dashboard, organization view, project create/update, and settings view/update. |
| viewer | Read-only organization viewer with dashboard, organization, members, projects, and settings view. |

## Effective permission calculation

Recommended application rule:

```text
effective_permissions = role.permissions + organization_members.custom_permissions
```

Then check whether the requested permission exists in the effective permission set.

## Authorization examples

| Action | Required permission |
| --- | --- |
| View dashboard | dashboard.view |
| Invite a member | members.invite |
| Change member role | members.update and roles.assign |
| Remove a member | members.remove |
| View billing | billing.view |
| Change subscription | billing.manage or subscription.update |
| View audit logs | audit_logs.view |
| Create project | projects.create |
| Delete project | projects.delete |

## Important SaaS-grade notes

- Do not authorize only by `role_key`. Use permission keys.
- Do not trust frontend permission checks. Backend must enforce permission checks.
- Permission changes should create `audit_logs` records.
- Role deletion should not break sessions; `role_key` gives fallback context, but backend should refresh permissions from database.
- For larger systems, replace `roles.permissions TEXT[]` with a normalized `role_permissions` join table.
