# Table: subscriptions

## Migration

`00009_create_subscriptions.sql`

## Domain

Billing / SaaS Subscription

## Purpose

Stores organization subscription and billing state.

## Why this table exists

In B2B SaaS, subscription usually belongs to the organization. One paid workspace may contain many members under one plan.

## Data owner

Billing module

## Main user stories supported

- As an owner, I can subscribe my organization to a plan.
- As the app, I can enforce features based on plan/status.
- As support, I can map local subscriptions to payment provider records.

## Business rules

- Subscription belongs to an organization.
- Plan controls entitlement level.
- Status controls whether paid features are available.
- External provider subscription ids are unique per provider when present.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal subscription primary key. | Internal |
| public_id | TEXT | Yes | 'sub_' + generated UUID | API-facing subscription id. | Public identifier |
| org_id | UUID | Yes | none | Organization that owns subscription. | Internal |
| plan | TEXT | Yes | 'free' | Machine-readable plan: free/pro/business/enterprise. | Operational/billing |
| plan_name | TEXT | Yes | 'Free' | Human-readable plan label. | Operational/billing |
| status | TEXT | Yes | 'active' | Billing lifecycle: trialing, active, past_due, cancelled, expired. | Billing |
| billing_cycle | TEXT | No | NULL | monthly/yearly/lifetime. | Billing |
| currency | TEXT | Yes | 'USD' | Billing currency. | Billing |
| amount | NUMERIC(12,2) | Yes | 0 | Subscription price amount. | Billing |
| trial_started_at | TIMESTAMPTZ | No | NULL | Trial start time. | Billing |
| trial_ends_at | TIMESTAMPTZ | No | NULL | Trial end time. | Billing |
| current_period_start | TIMESTAMPTZ | No | NULL | Current billing period start. | Billing |
| current_period_end | TIMESTAMPTZ | No | NULL | Current billing period end. | Billing |
| cancel_at_period_end | BOOLEAN | Yes | false | Whether cancellation is scheduled for period end. | Billing |
| payment_provider | TEXT | No | NULL | External payment provider name. | Billing |
| payment_customer_id | TEXT | No | NULL | External customer id. | Sensitive billing identifier |
| payment_subscription_id | TEXT | No | NULL | External subscription id. | Sensitive billing identifier |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## Relationships

- Belongs to organizations.
- Parent/optional reference for organization_usage.

## Constraints and indexes

- idx_subscriptions_org_id, status, plan, current_period_end.
- Unique (payment_provider, payment_subscription_id) where both are not null.

## Deletion behavior

Deleting organization cascades subscription. Usage rows keep history with subscription_id set null if subscription is deleted.

## Related API endpoints

- `/billing/subscription`
- `/organizations/:orgId/subscription`
- `/billing/webhooks/:provider`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Add invoice/payment tables when actual payments are implemented.
- Audit every plan/status change.
