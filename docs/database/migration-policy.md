# Database Migration Policy

## Goal

Every migration should be safe, reviewable, reversible where practical, and documented.

## Required migration checklist

Before merging a migration:

- The migration file has a clear name and sequence number.
- `up` migration includes schema changes.
- `down` migration exists and is safe for the environment.
- Every new table has `COMMENT ON TABLE`.
- Every important column has `COMMENT ON COLUMN`.
- Indexes are named consistently.
- Foreign keys explicitly define delete behavior.
- Sensitive columns are reflected in `data-classification.md`.
- Table docs are updated in `docs/database/tables/`.
- Data dictionary is updated.
- ERD/relationships are updated if relations changed.
- API docs are updated if behavior changed.

## Naming conventions

| Object | Convention | Example |
|---|---|---|
| Table | plural snake_case | `organization_members` |
| Primary key | `id` | `id UUID PRIMARY KEY` |
| Public id | `public_id` | `usr_...`, `org_...` |
| Foreign key | `{parent_singular}_id` or domain-specific | `user_id`, `org_id` |
| Index | `idx_{table}_{columns}` | `idx_sessions_user_id` |
| Unique index | `idx_{table}_{columns}_unique` | `idx_users_email_lower_unique` |
| Timestamp | `{action}_at` | `created_at`, `revoked_at` |

## Public ID policy

Use internal UUIDs for database joins and prefixed `public_id` for API responses.

Examples:

| Entity | Prefix |
|---|---|
| users | `usr_` |
| organizations | `org_` |
| permissions | `perm_` |
| roles | `role_` |
| organization_members | `mem_` |
| auth_accounts | `acct_` |
| sessions | `sess_` |
| verification_tokens | `vt_` |
| subscriptions | `sub_` |
| organization_usage | `usage_` |
| audit_logs | `audit_` |

## Safe migration rules

- Avoid destructive migrations in production without a rollback/backup plan.
- Add nullable columns first, backfill data, then add NOT NULL constraints later if needed.
- Create indexes carefully on large tables.
- Avoid locking large production tables during peak usage.
- Do not rename/drop columns without updating application code and docs in the same release plan.

## Documentation rule

No database migration should be merged without documentation updates.
