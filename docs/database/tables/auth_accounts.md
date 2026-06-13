# Table: auth_accounts

## Migration

`00006_create_auth_accounts.sql`

## Domain

Core Identity / Authentication

## Purpose

Stores linked authentication provider accounts such as credentials, Google, Facebook, GitHub, OIDC, email, and WebAuthn.

## Why this table exists

A single app user may sign in with multiple providers. Provider identity must be separate from the core users table.

## Data owner

Identity/Auth module

## Main user stories supported

- As a user, I can link Google/GitHub/Facebook login.
- As auth logic, I can map external provider identity to an internal user.
- As security, I can track when a provider was last used.

## Business rules

- Provider + provider_account_id must be unique.
- OAuth tokens must be encrypted before production use.
- Deleting a user cascades linked auth accounts.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal account link primary key. | Internal |
| public_id | TEXT | Yes | 'acct_' + generated UUID | API-facing account-link identifier. | Public identifier |
| user_id | UUID | Yes | none | Internal user this provider account belongs to. | Internal |
| provider | TEXT | Yes | none | Provider id such as credentials/google/facebook. | Operational |
| provider_account_id | TEXT | Yes | none | Unique account id received from provider. | Sensitive identifier |
| provider_type | TEXT | Yes | 'oauth' | Provider type: oauth, oidc, credentials, email, webauthn. | Operational |
| access_token | TEXT | No | NULL | OAuth access token. | Critical secret |
| refresh_token | TEXT | No | NULL | OAuth refresh token. | Critical secret |
| id_token | TEXT | No | NULL | OIDC ID token. | Critical secret |
| token_type | TEXT | No | NULL | OAuth token type. | Operational |
| scope | TEXT | No | NULL | Granted provider scopes. | Security |
| expires_at | TIMESTAMPTZ | No | NULL | Provider token expiry time. | Security |
| connected_at | TIMESTAMPTZ | Yes | now() | When provider was linked. | Operational |
| last_used_at | TIMESTAMPTZ | No | NULL | Last time this provider was used. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## Relationships

- Belongs to users through user_id.

## Constraints and indexes

- Unique (provider, provider_account_id).
- idx_auth_accounts_user_id.
- idx_auth_accounts_provider.
- idx_auth_accounts_provider_account.

## Deletion behavior

Deleting user cascades auth_accounts.

## Related API endpoints

- `/auth/providers`
- `/auth/link-provider`
- `/auth/unlink-provider`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Encrypt access_token, refresh_token, and id_token.
- Consider token rotation/audit events.
