# Table: users

## Migration

`00001_create_users.sql`

## Domain

Core Identity

## Purpose

Stores registered BusinessSAAS user accounts, profile data, login/security state, and user-level preferences.

## Why this table exists

A SaaS application needs a global identity record for each person before that person can join one or more organizations. The users table should represent the person/account, not their organization-specific role.

## Data owner

Identity/Auth module

## Main user stories supported

- As a user, I can sign up, sign in, verify my email, and manage my profile.
- As an admin, I can identify who performed business/security actions.
- As the product, I can store user-specific UI settings, shortcuts, onboarding state, and feature flags.

## Business rules

- A user may exist without a password_hash when the account is OAuth-only.
- Email and username are unique case-insensitively when present.
- Raw passwords must never be stored; only password_hash is allowed.
- Soft delete is represented with deleted_at and status = deleted.
- Security lockout is controlled by failed_logins and locked_until.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal primary key for joins and foreign keys. | Internal |
| public_id | TEXT | Yes | 'usr_' + generated UUID | Stable API-facing user identifier. | Public identifier |
| email | TEXT | No | NULL | Login/contact email; nullable for OAuth-only or incomplete provider data. | PII |
| password_hash | TEXT | No | NULL | Hashed password for credential login. | Critical secret |
| username | TEXT | No | NULL | Optional unique handle for the user. | PII/public |
| display_name | TEXT | Yes | '' | Name shown in the UI. | PII |
| first_name | TEXT | Yes | '' | User first/given name. | PII |
| last_name | TEXT | Yes | '' | User last/family name. | PII |
| full_name | TEXT | Yes | '' | Full name for display/search/export. | PII |
| photo_url | TEXT | No | NULL | Profile image URL. | PII |
| cover_photo_url | TEXT | No | NULL | Profile cover image URL. | PII |
| phone | TEXT | No | NULL | User phone number. | PII |
| phone_verified | BOOLEAN | Yes | false | Whether phone number ownership is verified. | Security/PII |
| email_verified | BOOLEAN | Yes | false | Whether email ownership is verified. | Security |
| email_verified_at | TIMESTAMPTZ | No | NULL | Timestamp when email verification completed. | Security |
| country | CHAR(2) | No | NULL | ISO country code for localization/compliance. | Location/PII |
| timezone | TEXT | Yes | 'UTC' | Preferred timezone for displaying dates. | Preference |
| locale | TEXT | Yes | 'en' | Preferred locale for formatting. | Preference |
| language | TEXT | Yes | 'en' | Preferred language. | Preference |
| currency | TEXT | Yes | 'USD' | Preferred/default currency. | Preference |
| status | TEXT | Yes | 'active' | Account lifecycle state: active, suspended, deleted, pending_verification. | Operational |
| account_type | TEXT | Yes | 'saas_customer' | Classifies account type for future product/business logic. | Operational |
| suspended_at | TIMESTAMPTZ | No | NULL | When the account was suspended. | Security |
| suspension_reason | TEXT | No | NULL | Reason for suspension. | Sensitive operational |
| login_redirect_url | TEXT | Yes | '/dashboard' | Default post-login destination. | Operational |
| shortcuts | TEXT[] | Yes | empty array | Fuse app shortcut identifiers. | Preference |
| settings | JSONB | Yes | {} | Fuse-compatible UI/user settings. | Preference |
| preferences | JSONB | Yes | {} | Product preferences. | Preference |
| onboarding | JSONB | Yes | {} | Onboarding progress and completed steps. | Operational |
| feature_flags | JSONB | Yes | {} | Per-user feature flag overrides. | Operational |
| two_fa_enabled | BOOLEAN | Yes | false | Whether two-factor authentication is enabled. | Security |
| two_fa_secret | TEXT | No | NULL | 2FA secret; must be encrypted at rest. | Critical secret |
| backup_codes | JSONB | Yes | [] | Hashed/encrypted backup codes for 2FA recovery. | Critical secret |
| last_login_at | TIMESTAMPTZ | No | NULL | Last successful login time. | Security |
| last_activity_at | TIMESTAMPTZ | No | NULL | Last meaningful product activity time. | Operational |
| failed_logins | INTEGER | Yes | 0 | Consecutive failed login counter for lockout. | Security |
| locked_until | TIMESTAMPTZ | No | NULL | Temporary account lock expiry. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |
| deleted_at | TIMESTAMPTZ | No | NULL | Soft delete timestamp. | Operational |

## Relationships

- Parent of auth_accounts through auth_accounts.user_id.
- Parent of sessions through sessions.user_id.
- Parent of verification_tokens through verification_tokens.user_id.
- Parent of organization_members through organization_members.user_id.
- Referenced by audit_logs.user_id and organization_members.invited_by.

## Constraints and indexes

- idx_users_email_lower_unique: unique lower(email) where email is not null.
- idx_users_username_lower_unique: unique lower(username) where username is not null.
- idx_users_public_id, idx_users_status, idx_users_created_at, idx_users_last_login_at.

## Deletion behavior

Uses soft delete through deleted_at/status. Child auth/session/membership records cascade if a physical delete is performed.

## Related API endpoints

- `/auth/signup`
- `/auth/login`
- `/auth/me`
- `/users/me`
- `/users/me/settings`
- `/admin/users/:publicId`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Encrypt two_fa_secret and backup_codes.
- Add updated_at trigger.
- Consider CITEXT for email/username if preferred.
