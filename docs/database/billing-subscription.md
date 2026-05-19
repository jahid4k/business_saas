# Billing and Subscription Documentation

## Related tables

- `subscriptions` — current organization subscription and payment provider mapping.
- `organization_usage` — plan limits and actual usage by billing period.
- `organizations` — owner of subscription and usage records.
- `audit_logs` — records subscription and billing changes.

## Business model

The current schema follows a B2B SaaS model where billing belongs to the organization/workspace, not directly to a single user.

That means:

- One organization can have one or more subscription records over time.
- One subscription can cover many organization members.
- Feature access should be calculated from subscription plan/status plus organization usage.

## Subscription fields that control access

| Field | Meaning |
|---|---|
| `plan` | Machine-readable plan level: free, pro, business, enterprise. |
| `status` | Billing state: trialing, active, past_due, cancelled, expired. |
| `billing_cycle` | monthly, yearly, or lifetime. |
| `current_period_end` | Used to know when the current billing period ends. |
| `cancel_at_period_end` | Indicates scheduled cancellation. |
| `payment_provider` | External payment platform name. |
| `payment_customer_id` | External customer identifier. |
| `payment_subscription_id` | External subscription identifier. |

## Usage model

`organization_usage.limits` and `organization_usage.used` are JSONB because this is flexible for early SaaS development.

Example:

```json
{
  "limits": {
    "members": 10,
    "projects": 25,
    "storageGB": 20,
    "apiRequestsPerMonth": 100000
  },
  "used": {
    "members": 7,
    "projects": 12,
    "storageGB": 4.2,
    "apiRequestsPerMonth": 18420
  }
}
```

## Access decision example

```text
Can create project?
1. Check user has projects.create permission.
2. Check organization subscription status is trialing or active.
3. Check organization_usage.used.projects < organization_usage.limits.projects.
4. If all true, allow.
```

## Recommended billing audit events

| Event type | When to log |
|---|---|
| `billing.subscription_created` | New subscription created. |
| `billing.subscription_changed` | Plan/status/provider ID changed. |
| `billing.subscription_cancelled` | Subscription cancelled. |
| `billing.payment_failed` | Payment provider reports failure. |
| `billing.usage_limit_reached` | Organization hits a plan limit. |
| `billing.plan_upgraded` | Plan upgraded. |
| `billing.plan_downgraded` | Plan downgraded. |

## Future billing tables

When payment handling becomes more complete, add:

- `invoices`
- `payments`
- `payment_methods`
- `subscription_events`
- `usage_events`
