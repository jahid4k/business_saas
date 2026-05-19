# Table: organization_usage

## Migration

`00010_create_organization_usage.sql`

## Domain

Billing / Usage Metering

## Purpose

Stores per-period organization usage and plan limits.

## Why this table exists

SaaS plans usually limit members, projects, storage, API calls, or other resources. Usage must be tracked separately from subscription state.

## Data owner

Billing/Usage module

## Main user stories supported

- As the system, I can block usage beyond plan limits.
- As an owner, I can see current plan usage.
- As billing, I can evaluate upgrade prompts.

## Business rules

- One usage row per organization per period.
- limits and used are JSONB for early SaaS flexibility.
- period_start/period_end define the measurement window.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal usage primary key. | Internal |
| public_id | TEXT | Yes | 'usage_' + generated UUID | API-facing usage id. | Public identifier |
| org_id | UUID | Yes | none | Organization whose usage is tracked. | Internal |
| subscription_id | UUID | No | NULL | Related subscription for the period. | Internal/billing |
| period_start | TIMESTAMPTZ | Yes | none | Usage period start. | Billing/operational |
| period_end | TIMESTAMPTZ | Yes | none | Usage period end. | Billing/operational |
| limits | JSONB | Yes | {} | Plan limit object such as members/projects/storageGB. | Operational/billing |
| used | JSONB | Yes | {} | Actual usage object such as projects/storage/api requests. | Operational/billing |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## Relationships

- Belongs to organizations.
- Optionally belongs to subscriptions.

## Constraints and indexes

- Unique (org_id, period_start, period_end).
- idx_organization_usage_org_id.
- idx_organization_usage_subscription_id.
- idx_organization_usage_period.

## Deletion behavior

Deleting organization cascades usage. Deleting subscription sets subscription_id null.

## Related API endpoints

- `/organizations/:orgId/usage`
- `/billing/usage`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Normalize frequently queried usage metrics if JSONB becomes hard to query.
- Add usage events table if audit-level metering is needed.
