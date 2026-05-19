# Table: roles

## Migration

`00004_create_roles.sql`

## Domain

Authorization / RBAC

## Purpose

Stores system role templates and organization-specific custom roles.

## Why this table exists

Roles group multiple permissions into business-friendly access levels such as owner, admin, manager, member, and viewer.

## Data owner

Authorization/RBAC module

## Main user stories supported

- As an owner, I can assign members an appropriate role.
- As a SaaS platform, I can provide default roles.
- As a tenant, I can later create organization-specific custom roles.

## Business rules

- org_id NULL means global system role template.
- org_id non-null means tenant-specific role.
- System role names are unique globally.
- Organization role names are unique within each organization.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal role primary key. | Internal |
| public_id | TEXT | Yes | 'role_' + generated UUID | API-facing role identifier. | Public identifier |
| org_id | UUID | No | NULL | Tenant owner; NULL for global templates. | Internal/business |
| name | TEXT | Yes | none | Role name such as owner/admin/member. | Operational |
| description | TEXT | No | NULL | Human-readable role explanation. | Operational |
| permissions | TEXT[] | Yes | empty array | Permission keys included in this role. | Security/authorization |
| is_system | BOOLEAN | Yes | false | Whether this is a platform-defined role. | Operational |
| is_custom | BOOLEAN | Yes | false | Whether tenant created/customized this role. | Operational |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## Relationships

- May belong to organizations through roles.org_id.
- Referenced by organization_members.role_id.

## Constraints and indexes

- idx_roles_org_name_unique for tenant roles.
- idx_roles_system_name_unique for global templates.
- idx_roles_org_id.
- idx_roles_is_system.

## Deletion behavior

If a role is deleted, organization_members.role_id becomes NULL because FK uses ON DELETE SET NULL. Application should handle fallback using role_key.

## Related API endpoints

- `/rbac/roles`
- `/organizations/:orgId/roles`
- `/organizations/:orgId/roles/:roleId`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Add immutable system-role protection at application layer.
- Consider normalized role_permissions table later.
