# Database Relationships

This document explains how records connect and how deletion behavior should be interpreted by the application.

## Relationship summary

| Table | Relationship notes |
| --- | --- |
| users | Parent of auth_accounts through auth_accounts.user_id. |
| users | Parent of sessions through sessions.user_id. |
| users | Parent of verification_tokens through verification_tokens.user_id. |
| users | Parent of organization_members through organization_members.user_id. |
| users | Referenced by audit_logs.user_id and organization_members.invited_by. |
| organizations | Parent of organization_members, roles, sessions, subscriptions, organization_usage, and audit_logs. |
| permissions | No FK from roles.permissions because roles stores permission keys as TEXT[]. Application must validate these keys against permissions.key. |
| roles | May belong to organizations through roles.org_id. |
| roles | Referenced by organization_members.role_id. |
| organization_members | Belongs to organizations. |
| organization_members | Belongs to users. |
| organization_members | Optionally belongs to roles. |
| organization_members | invited_by references users. |
| auth_accounts | Belongs to users through user_id. |
| sessions | Belongs to users. |
| sessions | Optionally belongs to organizations. |
| verification_tokens | Optionally belongs to users. |
| subscriptions | Belongs to organizations. |
| subscriptions | Parent/optional reference for organization_usage. |
| organization_usage | Belongs to organizations. |
| organization_usage | Optionally belongs to subscriptions. |
| audit_logs | Optionally belongs to organizations. |
| audit_logs | Optionally belongs to users. |
| tasks | Belongs to organizations via org_id. |
| tasks | Optionally belongs to users via created_by. |
| tasks | Optionally belongs to users via assigned_to. |

## Foreign key deletion behavior

| Child table | Foreign key | Parent table | Delete behavior | Business meaning |
| --- | --- | --- | --- | --- |
| auth_accounts | user_id | users | ON DELETE CASCADE | If a user is physically removed, linked provider accounts are removed. |
| sessions | user_id | users | ON DELETE CASCADE | If a user is physically removed, sessions are removed. |
| sessions | org_id | organizations | ON DELETE CASCADE | Organization-scoped sessions are removed if organization is physically removed. |
| verification_tokens | user_id | users | ON DELETE CASCADE | User-bound verification/recovery tokens are removed. |
| roles | org_id | organizations | ON DELETE CASCADE | Tenant-specific roles are removed with the tenant. |
| organization_members | org_id | organizations | ON DELETE CASCADE | Memberships are removed if organization is removed. |
| organization_members | user_id | users | ON DELETE CASCADE | Memberships are removed if user is removed. |
| organization_members | role_id | roles | ON DELETE SET NULL | Membership survives if assigned role is removed; role_key remains as fallback. |
| organization_members | invited_by | users | ON DELETE SET NULL | Invitation record survives if inviter is removed. |
| subscriptions | org_id | organizations | ON DELETE CASCADE | Subscription belongs to organization. |
| organization_usage | org_id | organizations | ON DELETE CASCADE | Usage belongs to organization. |
| organization_usage | subscription_id | subscriptions | ON DELETE SET NULL | Usage history can remain even if subscription reference is removed. |
| audit_logs | org_id | organizations | ON DELETE CASCADE | Current schema removes tenant audit logs if organization is physically deleted. |
| audit_logs | user_id | users | ON DELETE SET NULL | Audit event remains even if actor user is removed. |
| tasks | org_id | organizations | ON DELETE CASCADE | All tasks are removed when organization is deleted. |
| tasks | created_by | users | ON DELETE SET NULL | Task survives if creator is removed; created_by becomes NULL. |
| tasks | assigned_to | users | ON DELETE SET NULL | Task survives if assignee is removed; assigned_to becomes NULL. |

## Recommended SaaS policy

Prefer soft delete for `users` and `organizations`. Physical deletion should be rare and controlled because many child records cascade. For compliance or privacy deletion, create a formal deletion/anonymization procedure instead of casually deleting parent rows.
