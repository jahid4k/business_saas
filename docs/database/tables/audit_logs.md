# Table: audit_logs

## Migration

`00011_create_audit_logs.sql`

## Domain

Security / Compliance

## Purpose

Stores security and business audit events.

## Why this table exists

SaaS systems need a reliable trail of important actions: sign-ins, role changes, billing changes, settings updates, and failures.

## Data owner

Security/Platform module

## Main user stories supported

- As an owner, I can review important workspace activity.
- As security, I can investigate suspicious behavior.
- As support, I can troubleshoot changes without guessing.

## Business rules

- Application should treat this table as append-only.
- Do not store unnecessary secrets in changes/metadata.
- status should indicate success, failure, or warning when applicable.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal audit log primary key. | Internal |
| public_id | TEXT | Yes | 'audit_' + generated UUID | API-facing audit event id. | Public identifier |
| org_id | UUID | No | NULL | Organization context of event. | Internal/business |
| user_id | UUID | No | NULL | Actor user if known. | Internal/PII link |
| event_type | TEXT | Yes | none | Event key such as auth.sign_in or billing.subscription_changed. | Operational/security |
| description | TEXT | No | NULL | Human-readable event summary. | Operational |
| resource_type | TEXT | No | NULL | Type of resource affected. | Operational |
| resource_id | TEXT | No | NULL | Public or internal resource id affected. | Operational |
| changes | JSONB | No | NULL | Before/after change details. | Potentially sensitive |
| metadata | JSONB | Yes | {} | Additional structured context. | Potentially sensitive |
| ip_address | INET | No | NULL | IP address of actor/request. | PII/security |
| user_agent | TEXT | No | NULL | User-agent string. | Device/PII-like |
| status | TEXT | No | NULL | Event outcome: success/failure/warning. | Operational/security |
| error_message | TEXT | No | NULL | Error message when event failed. | Sensitive operational |
| created_at | TIMESTAMPTZ | Yes | now() | Event time. | Operational |

## Relationships

- Optionally belongs to organizations.
- Optionally belongs to users.

## Constraints and indexes

- idx_audit_logs_org_id.
- idx_audit_logs_user_id.
- idx_audit_logs_event_type.
- idx_audit_logs_resource.
- idx_audit_logs_created_at.

## Deletion behavior

Prefer retention policy instead of ordinary delete. If organization is physically deleted, related audit logs cascade by current schema.

## Related API endpoints

- `/organizations/:orgId/audit-logs`
- `/admin/audit-logs`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Consider preventing updates/deletes using DB permissions or triggers.
- Add retention/archive policy.
