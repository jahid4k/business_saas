# Module: Auth

## What this module does

Handles user identity: signup, login, token refresh, logout, and password reset.
Auth is the entry point for all users — it runs before any other module.

## Backend endpoints

| Method | Path | Auth required | Description |
|--------|------|:-------------:|-------------|
| POST | `/api/v1/auth/signup` | No | Create account |
| POST | `/api/v1/auth/login` | No | Get access + refresh token |
| POST | `/api/v1/auth/refresh` | No (cookie) | Exchange refresh cookie for new access token |
| POST | `/api/v1/auth/logout` | JWT | Revoke current session |
| POST | `/api/v1/auth/logout-all` | JWT | Revoke all sessions |
| GET | `/api/v1/auth/me` | JWT | Current user from token claims |
| POST | `/api/v1/auth/password-reset/request` | No | Send reset email |
| POST | `/api/v1/auth/password-reset/confirm` | No | Apply new password |

## Frontend pages

```
app/(auth)/
├── login/page.tsx          → /login
├── signup/page.tsx         → /signup
└── reset-password/page.tsx → /reset-password
```

## Key components

```
features/auth/
├── LoginForm.tsx       → email + password, error handling, redirect after login
├── SignupForm.tsx      → name + email + password, auto-login on success
└── ResetPasswordForm.tsx
```

## Token flow (summary — full detail in ADR-0006)

1. Login → `access_token` in response body, `bsaas_refresh` cookie set by backend
2. Store `access_token` in memory (`lib/api.ts` module variable)
3. Every API call sends `Authorization: Bearer <access_token>`
4. On 401 → POST `/auth/refresh` (cookie sent automatically) → get new access token
5. On tab open → POST `/auth/refresh` to restore session from cookie
6. Logout → DELETE from sessions table → cookie cleared by backend

## Important rules

- Never store the access token in `localStorage` or any cookie accessible by JavaScript
- Never show detailed error messages for login failures — always "Invalid email or password"
- The refresh endpoint must be called with `credentials: 'include'` so the browser sends the cookie
- After signup, auto-login by calling the login endpoint — do not send the user to `/login`

## Zustand state

```typescript
// store/auth.ts
{
  user: User | null
  isAuthenticated: boolean
  setUser: (user: User) => void
  clearUser: () => void
}
```

## Error codes from backend

| Code | HTTP | When |
|------|------|------|
| `INVALID_CREDENTIALS` | 401 | Wrong email or password |
| `EMAIL_TAKEN` | 409 | Email already registered |
| `INVALID_TOKEN` | 401 | Access token invalid or expired (trigger refresh) |
| `TOKEN_EXPIRED` | 401 | Access token expired (trigger refresh) |
| `SESSION_NOT_FOUND` | 401 | Refresh token invalid or revoked (force login) |
| `RATE_LIMITED` | 429 | Too many login attempts |

## Related ADRs

- [ADR-0003](../decisions/0003-auth-token-strategy.md) — token design
- [ADR-0006](../decisions/0006-token-storage-strategy.md) — token storage in frontend
