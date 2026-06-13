# Table: organizations

## Migration

`00002_create_organizations.sql`

## Domain

Tenant / Workspace

## Purpose

Stores SaaS tenant/workspace/company records.

## Why this table exists

In B2B SaaS, billing, members, roles, usage, and audit activity usually belong to a tenant organization rather than directly to a single user.

## Data owner

Organization/Tenant module

## Main user stories supported

- As a customer, I can create/manage a workspace.
- As the system, I can isolate data by organization.
- As billing, I can attach subscriptions and usage to a company.

## Business rules

- Slug is unique case-insensitively.
- Status controls tenant availability.
- deleted_at supports soft delete.
- metadata stores flexible tenant-level attributes for early-stage SaaS.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal organization primary key. | Internal |
| public_id | TEXT | Yes | 'org_' + generated UUID | API-facing organization identifier. | Public identifier |
| name | TEXT | Yes | none | Organization display name. | Business data |
| slug | TEXT | Yes | none | Unique workspace slug used in URLs. | Public/business |
| legal_name | TEXT | No | NULL | Official legal business name. | Business sensitive |
| type | TEXT | No | NULL | Organization type/category. | Business data |
| industry | TEXT | No | NULL | Industry classification. | Business data |
| website | TEXT | No | NULL | Company website. | Public/business |
| logo_url | TEXT | No | NULL | Organization logo URL. | Public/business |
| country | CHAR(2) | No | NULL | Organization country code. | Business/location |
| timezone | TEXT | Yes | 'UTC' | Default organization timezone. | Operational |
| currency | TEXT | Yes | 'USD' | Default billing/display currency. | Operational |
| status | TEXT | Yes | 'active' | Tenant lifecycle: active, suspended, deleted. | Operational |
| metadata | JSONB | Yes | {} | Flexible organization-level metadata. | Varies |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |
| deleted_at | TIMESTAMPTZ | No | NULL | Soft delete timestamp. | Operational |

## Relationships

- Parent of organization_members, roles, sessions, subscriptions, organization_usage, and audit_logs.

## Constraints and indexes

- idx_organizations_slug_lower_unique.
- idx_organizations_public_id, idx_organizations_status, idx_organizations_created_at.

## Deletion behavior

Soft delete is preferred. Physical delete cascades to organization-specific children because most FKs use ON DELETE CASCADE.

## Related API endpoints

- `/organizations`
- `/organizations/:orgId`
- `/organizations/:orgId/settings`
- `/organizations/:orgId/members`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Add tenant plan/feature snapshot only if needed; current design keeps that in subscriptions.
