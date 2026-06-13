# Table: organization_members

## Migration

`00005_create_organization_members.sql`

## Domain

Tenant / Workspace Membership

## Purpose

Connects users to organizations and stores organization-specific access, role, title, department, and invitation state.

## Why this table exists

A user can belong to multiple organizations with different roles. Therefore organization access must live in a bridge table, not directly in users.

## Data owner

Organization/Membership module

## Main user stories supported

- As an owner, I can invite users to my workspace.
- As a user, I can belong to multiple companies.
- As authorization logic, I can calculate effective permissions per organization.

## Business rules

- One user cannot be duplicated in the same organization.
- Membership status controls access.
- Invitation status tracks onboarding flow.
- role_key is a snapshot/fallback even when role_id is null.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal membership primary key. | Internal |
| public_id | TEXT | Yes | 'mem_' + generated UUID | API-facing membership identifier. | Public identifier |
| org_id | UUID | Yes | none | Organization this member belongs to. | Internal |
| user_id | UUID | Yes | none | User who is a member. | Internal |
| role_id | UUID | No | NULL | Optional FK to roles table. | Internal/authorization |
| role_key | TEXT | Yes | 'member' | Role snapshot used by API/session. | Authorization |
| title | TEXT | No | NULL | Job title inside organization. | PII/business |
| department | TEXT | No | NULL | Department/team inside organization. | Business data |
| status | TEXT | Yes | 'active' | Membership status: active, inactive, suspended. | Authorization |
| custom_permissions | TEXT[] | Yes | empty array | Extra permission keys directly granted to member. | Authorization/security |
| invitation_status | TEXT | Yes | 'accepted' | Invitation state: pending, accepted, rejected, expired. | Operational |
| invited_by | UUID | No | NULL | User who invited this member. | Audit/PII |
| invitation_sent_at | TIMESTAMPTZ | No | NULL | When invitation was sent. | Operational |
| invitation_accepted_at | TIMESTAMPTZ | No | NULL | When invitation was accepted. | Operational |
| joined_at | TIMESTAMPTZ | Yes | now() | When user joined organization. | Operational |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## Relationships

- Belongs to organizations.
- Belongs to users.
- Optionally belongs to roles.
- invited_by references users.

## Constraints and indexes

- Unique (org_id, user_id).
- idx_org_members_org_id, user_id, role_id, role_key, status, invitation_status.

## Deletion behavior

Physical delete of organization/user cascades membership. Physical delete of role sets role_id null.

## Related API endpoints

- `/organizations/:orgId/members`
- `/organizations/:orgId/invitations`
- `/organizations/:orgId/members/:memberId/role`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Define effective permission calculation clearly: role.permissions + custom_permissions - future denied_permissions if added.
