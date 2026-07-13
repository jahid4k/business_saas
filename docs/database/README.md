# BusinessSAAS Database Documentation

Generated from migration files on 2026-05-19.

This documentation describes the current Phase 1 SaaS foundation schema: identity, organizations, RBAC, membership, authentication/session security, billing/subscription state, usage metering, and audit logging.

## Documentation map

- `erd.md` — entity relationship diagram and parent/child flow.
- `data-dictionary.md` — combined column dictionary for every table.
- `relationships.md` — foreign key and deletion behavior.
- `rbac-permissions.md` — RBAC design, permission keys, seeded roles, and authorization rules.
- `auth-security.md` — authentication/session/token storage notes.
- `billing-subscription.md` — subscription and usage model.
- `audit-logging.md` — audit trail design and event examples.
- `data-classification.md` — sensitivity classification and storage rules.
- `migration-policy.md` — rules for future database changes.
- `tables/*.md` — one page per table.
- `dbml/schema.dbml` — DBML representation for ERD tools.

## Core design principles

1. Use UUID internal primary keys for database relations.
2. Use prefixed `public_id` values for API-facing identifiers.
3. Keep user identity separate from organization membership.
4. Keep authorization permission-based, not only role-name-based.
5. Store raw passwords, refresh tokens, session tokens, and verification tokens nowhere.
6. Treat OAuth tokens, 2FA secrets, and backup codes as critical secrets.
7. Track important business/security actions in `audit_logs`.
8. Update these docs in the same pull request as every migration.

## Current table domains

| Domain                | Tables                                                      |
| --------------------- | ----------------------------------------------------------- |
| Core Identity         | `users`, `auth_accounts`, `sessions`, `verification_tokens` |
| Tenant / Workspace    | `organizations`, `organization_members`                     |
| Authorization / RBAC  | `permissions`, `roles`                                      |
| Billing / SaaS        | `subscriptions`, `organization_usage`                       |
| Security / Compliance | `audit_logs`                                                |

## SaaS-grade documentation rule

A migration is not complete until the following are updated:

- PostgreSQL `COMMENT ON TABLE` / `COMMENT ON COLUMN` statements.
- Table page under `docs/database/tables/`.
- `data-dictionary.md` if a column changed.
- `relationships.md` and `erd.md` if a relationship changed.
- `data-classification.md` if sensitive fields were added or changed.
- API documentation if request/response behavior changed.
