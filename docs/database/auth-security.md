# Authentication and Security Documentation

## Related tables

- `users` — core user account, profile, security state, 2FA fields.
- `auth_accounts` — linked provider identities and OAuth/OIDC tokens.
- `sessions` — active/historical sessions and refresh-token tracking.
- `verification_tokens` — email verification, password reset, magic link, invitation, and 2FA one-time tokens.
- `audit_logs` — records important auth/security actions.

## SaaS-grade storage rules

| Data | Storage rule |
|---|---|
| Password | Store only password hash. Never plaintext. |
| Session token / refresh token | Store only `token_hash`. Never raw token. |
| Verification token | Store only `token_hash`. Never raw token. |
| OAuth access token | Encrypt before production use. |
| OAuth refresh token | Encrypt before production use. |
| OIDC id token | Encrypt or avoid storing unless required. |
| 2FA secret | Encrypt at rest. |
| Backup codes | Store hashed/encrypted; never plaintext after generation. |

## Recommended auth events for audit_logs

| Event type | When to log |
|---|---|
| `auth.sign_up` | User account created. |
| `auth.sign_in` | Successful login. |
| `auth.sign_in_failed` | Failed login attempt. |
| `auth.logout` | User logs out. |
| `auth.session_revoked` | User/admin revokes a session. |
| `auth.password_reset_requested` | Password reset requested. |
| `auth.password_reset_completed` | Password reset completed. |
| `auth.email_verified` | Email verification completed. |
| `auth.2fa_enabled` | 2FA enabled. |
| `auth.2fa_disabled` | 2FA disabled. |
| `auth.provider_linked` | OAuth provider linked. |
| `auth.provider_unlinked` | OAuth provider removed. |

## Session lifecycle

1. User signs in.
2. App generates raw refresh/session token.
3. App stores only token hash in `sessions.token_hash`.
4. Raw token is returned to the client through secure cookie or equivalent secure channel.
5. On refresh, app hashes presented token and compares with stored hash.
6. On logout/revocation, app sets `revoked_at`.
7. Expired sessions are removed or archived by scheduled cleanup.

## Production checklist

- Use HTTPS-only secure cookies in production.
- Use HttpOnly cookies for refresh tokens.
- Rotate JWT signing secrets carefully.
- Rate-limit login, password reset, magic link, and verification endpoints.
- Lock accounts temporarily using `failed_logins` and `locked_until`.
- Encrypt OAuth and 2FA secrets.
- Log security events to `audit_logs`.
