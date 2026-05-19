# Table: verification_tokens

## Migration

`00008_create_verification_tokens.sql`

## Domain

Core Identity / Verification

## Purpose

Stores one-time email verification, password reset, magic link, invitation, and 2FA tokens.

## Why this table exists

Verification and recovery flows require short-lived tokens. Storing only a hash prevents token theft from the database.

## Data owner

Identity/Auth module

## Main user stories supported

- As a user, I can verify email and reset password.
- As an invited member, I can accept an invitation.
- As security, tokens expire and cannot be reused.

## Business rules

- Only token_hash is stored; raw token is sent once to the user.
- expires_at is mandatory.
- used_at prevents token reuse.
- type constrains token purpose.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal verification token primary key. | Internal |
| public_id | TEXT | Yes | 'vt_' + generated UUID | API-facing token record id. | Public identifier |
| user_id | UUID | No | NULL | Related user if known. | Internal |
| email | TEXT | No | NULL | Email target for token. | PII |
| token_hash | TEXT | Yes | none | Hash of one-time token. | Critical secret |
| type | TEXT | Yes | none | Token purpose. | Security |
| verified_at | TIMESTAMPTZ | No | NULL | When verification completed. | Security |
| used_at | TIMESTAMPTZ | No | NULL | When token was consumed. | Security |
| expires_at | TIMESTAMPTZ | Yes | none | Expiry time. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |

## Relationships

- Optionally belongs to users.

## Constraints and indexes

- Unique token_hash.
- idx_verification_tokens_user_id.
- idx_verification_tokens_email_lower.
- idx_verification_tokens_type.
- idx_verification_tokens_expires_at.

## Deletion behavior

Deleting a user cascades user-bound tokens. Expired tokens should be removed by scheduled cleanup.

## Related API endpoints

- `/auth/verify-email`
- `/auth/password-reset/request`
- `/auth/password-reset/confirm`
- `/auth/magic-link`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Add rate-limit metadata if needed.
- Add consumed_by_ip/user_agent if stricter audit is required.
