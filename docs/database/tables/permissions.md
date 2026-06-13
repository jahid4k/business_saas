# Table: permissions

## Migration

`00003_create_permissions.sql`

## Domain

Authorization / RBAC

## Purpose

Stores canonical permission keys used by roles and authorization checks.

## Why this table exists

Permission keys give the application a stable authorization vocabulary. Roles can change, but the permission meaning should remain predictable.

## Data owner

Authorization/RBAC module

## Main user stories supported

- As a developer, I can protect API routes with permission keys.
- As an admin, I can understand what a role can do.
- As the system, I can validate role permission arrays.

## Business rules

- Permission key must be unique.
- Permission style is resource.action, for example billing.manage.
- System permissions should be seeded and rarely deleted.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal permission primary key. | Internal |
| public_id | TEXT | Yes | 'perm_' + generated UUID | API-facing permission identifier. | Public identifier |
| key | TEXT | Yes | none | Dot-format permission key. | Operational |
| resource | TEXT | Yes | none | Resource/domain controlled by permission. | Operational |
| action | TEXT | Yes | none | Action allowed on the resource. | Operational |
| description | TEXT | No | NULL | Human-readable explanation. | Operational |
| is_system | BOOLEAN | Yes | true | Marks platform-defined permission. | Operational |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## Relationships

- No FK from roles.permissions because roles stores permission keys as TEXT[]. Application must validate these keys against permissions.key.

## Constraints and indexes

- Unique key.
- idx_permissions_resource.
- idx_permissions_action.

## Deletion behavior

Avoid deleting permissions used in roles. Prefer deprecation flags in future if a permission must be retired.

## Related API endpoints

- `/rbac/permissions`
- `/rbac/permissions/:key`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Consider role_permissions join table if analytics/querying over permissions becomes complex.
