# Audit Logging Documentation

## Purpose

`audit_logs` stores security and business events that matter for accountability, debugging, compliance, support, and incident investigation.

## What should be logged

| Area | Example events |
|---|---|
| Authentication | sign in, failed sign in, logout, password reset, email verification |
| Session security | session created, session revoked, suspicious login |
| Organization | organization created, organization updated, organization suspended |
| Members | member invited, invitation accepted, role changed, member removed |
| RBAC | role created, role updated, permission changed |
| Billing | subscription created, plan changed, payment failed, cancellation |
| Settings | organization settings updated, security settings changed |
| API keys | API key created, revoked, permission changed |

## Event structure

| Column | Usage |
|---|---|
| `org_id` | Organization context. May be null for global events. |
| `user_id` | Actor user. May be null for system/provider events. |
| `event_type` | Stable event key, e.g. `auth.sign_in`. |
| `description` | Human-readable summary. |
| `resource_type` | Affected entity type. |
| `resource_id` | Affected entity id, preferably public_id. |
| `changes` | Before/after JSON for changed fields. |
| `metadata` | Additional structured details. |
| `ip_address` | Request IP. |
| `user_agent` | Request user agent. |
| `status` | success, failure, or warning. |
| `error_message` | Failure message, sanitized. |

## Example audit log payload

```json
{
  "event_type": "members.role_changed",
  "description": "Member role changed from member to admin",
  "resource_type": "organization_member",
  "resource_id": "mem_123",
  "changes": {
    "before": { "role_key": "member" },
    "after": { "role_key": "admin" }
  },
  "metadata": {
    "reason": "Promotion by owner"
  },
  "status": "success"
}
```

## Safety rules

- Do not store raw tokens.
- Do not store passwords.
- Do not store full Authorization headers.
- Sanitize error messages before storing.
- Treat logs as append-only from application code.
- Define a retention policy before production.
