# Table: sessions

## Migration

`00007_create_sessions.sql`

## Domain

Core Identity / Session Security

## Purpose

Stores active and historical user sessions/devices with revocation support.

## Why this table exists

JWT-only auth is hard to revoke and hard to display in security dashboards. A session table enables device management, refresh-token tracking, and forced logout.

## Data owner

Identity/Auth module

## Main user stories supported

- As a user, I can see and revoke active devices.
- As security, I can invalidate a compromised session.
- As the app, I can track session expiry and last activity.

## Business rules

- Raw tokens must never be stored; only token_hash.
- revoked_at marks invalidated sessions.
- expires_at is required for cleanup and security.

## Column data dictionary

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal session primary key. | Internal |
| public_id | TEXT | Yes | 'sess_' + generated UUID | API-facing session identifier. | Public identifier |
| user_id | UUID | Yes | none | User who owns the session. | Internal |
| org_id | UUID | No | NULL | Organization context for session if applicable. | Internal |
| token_hash | TEXT | Yes | none | Hash of refresh/session token. | Critical secret |
| device_name | TEXT | No | NULL | Human-friendly device name. | Device data |
| device_type | TEXT | No | NULL | Desktop/mobile/tablet/etc. | Device data |
| browser | TEXT | No | NULL | Browser name/version. | Device data |
| os | TEXT | No | NULL | Operating system. | Device data |
| user_agent | TEXT | No | NULL | Full user-agent string. | Device/PII-like |
| ip_address | INET | No | NULL | IP address at session creation/activity. | PII/security |
| country | TEXT | No | NULL | GeoIP country. | Location/PII |
| city | TEXT | No | NULL | GeoIP city. | Location/PII |
| region | TEXT | No | NULL | GeoIP region. | Location/PII |
| last_activity_at | TIMESTAMPTZ | Yes | now() | Last activity timestamp. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Session creation time. | Operational |
| expires_at | TIMESTAMPTZ | Yes | none | Session expiry timestamp. | Security |
| revoked_at | TIMESTAMPTZ | No | NULL | When user/admin revoked the session. | Security |

## Relationships

- Belongs to users.
- Optionally belongs to organizations.

## Constraints and indexes

- idx_sessions_user_id, org_id, token_hash, expires_at, revoked_at.

## Deletion behavior

Deleting a user cascades sessions. Deleting an organization cascades organization-scoped sessions.

## Related API endpoints

- `/auth/sessions`
- `/auth/sessions/:sessionId/revoke`
- `/auth/logout`

## Security and privacy notes

- Apply RBAC checks before exposing this table through API endpoints.
- Do not expose internal `id` unless there is a deliberate internal-admin use case.
- Prefer returning `public_id` to clients.
- Review sensitivity classification before adding new fields.

## Future improvement notes

- Add scheduled cleanup for expired sessions.
- Hash tokens with strong keyed hash/HMAC.
