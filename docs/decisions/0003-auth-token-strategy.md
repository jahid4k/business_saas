# ADR-0003: Auth strategy — JWT access tokens + opaque refresh tokens

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS is a multi-tenant SaaS platform where authentication is the foundation of all
security. The auth system must:

- Support thousands of concurrent authenticated sessions
- Allow instant revocation of a single session or all sessions for a user
- Embed tenant context (org ID + role) so API routes don't need an extra DB lookup per request
- Resist token theft — if an access token is stolen, the damage window must be minimal
- Resist replay attacks
- Never expose raw secrets in the database

---

## Decision

Use a **dual-token strategy**:

1. **JWT access token** — stateless, short TTL (15 minutes), signed with HMAC-SHA256
2. **Opaque refresh token** — stateful, long TTL (7 days), stored as SHA-256 hash in PostgreSQL

The access token is returned in the response body. The refresh token is set as an `HttpOnly;
Secure; SameSite=Strict` cookie — never in the response body, never readable by JavaScript.

---

## Token design

### Access token (JWT)

```
Claims:
  uid   string   — user UUID
  bid   string   — active organization UUID (empty before workspace selection)
  email string   — for display convenience only, not for auth decisions
  role  string   — role key in active org (empty before workspace selection)
  iss   string   — "businesssaas"
  iat   int64    — issued at
  exp   int64    — expires at (now + 15 minutes)
```

The JWT is verified by the `RequireAuth` middleware on every protected request. No database
lookup is needed for verification — the signature is checked against the HMAC secret.

The `bid` and `role` claims enable tenant-aware responses without a DB call. However, the
backend always re-checks permissions from the database for any state-changing operation —
the JWT claims are trusted for routing, not for authorization decisions.

### Refresh token (opaque)

```
Raw value:    crypto/rand — 32 bytes → base64url encoded (43 chars)
Stored as:    SHA-256(raw) — never the raw value
Cookie name:  bsaas_refresh
Cookie flags: HttpOnly, Secure, SameSite=Strict, Path=/api/v1/auth
```

The raw token is only ever seen by the client. The database stores only the hash. If the
database is compromised, stolen hashes cannot be used — the attacker needs the raw token
which was never persisted anywhere.

`Path=/api/v1/auth` scopes the cookie so it is only sent to the refresh endpoint, not to
every API call. This limits the CSRF surface area.

---

## Flow

### Login

```
POST /auth/login
  ← access_token in response body
  ← Set-Cookie: bsaas_refresh=<raw_token>; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth
```

### Authenticated request

```
GET /api/v1/organizations/:orgId/tasks
  Authorization: Bearer <access_token>
  → middleware verifies JWT signature and expiry
  → no DB lookup
```

### Refresh

```
POST /auth/refresh
  Cookie: bsaas_refresh=<raw_token>   (browser sends automatically)
  → backend: SHA-256(raw_token), lookup in sessions table
  → if found and not expired: issue new access_token, rotate refresh token
  ← new access_token in response body
  ← new Set-Cookie with rotated refresh token
```

### Logout

```
POST /auth/logout
  Authorization: Bearer <access_token>
  → backend deletes the session row from PostgreSQL
  ← Set-Cookie: bsaas_refresh=; Max-Age=0  (clears cookie)
```

### Logout all sessions

```
POST /auth/logout-all
  Authorization: Bearer <access_token>
  → backend deletes ALL session rows for this user
  ← clears cookie
```

---

## Security properties

| Property | How achieved |
|----------|-------------|
| Short damage window on access token theft | 15-minute TTL; token is useless after expiry |
| Refresh token unreadable by JavaScript | HttpOnly cookie — JS cannot access it |
| CSRF on refresh endpoint blocked | SameSite=Strict cookie; only same-site requests carry it |
| Raw refresh token never stored | SHA-256 hash only in DB; DB breach doesn't yield usable tokens |
| Instant single-session revocation | Delete one row from `sessions` table |
| Instant all-sessions revocation | Delete all rows for user from `sessions` table |
| Replay attack resistance | Refresh token rotation: old token is deleted on use |

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Long-lived JWT only | Cannot revoke without a blocklist; one stolen token = long exposure window |
| Session cookies only | Stateful on every request; doesn't scale horizontally without sticky sessions or shared session store |
| OAuth2 (self-hosted) | Extreme complexity for a first-party auth system; appropriate only for third-party integrations |
| Storing access token in localStorage | Readable by any JS on the page — XSS instantly steals it |
| Storing access token in a cookie | HttpOnly cookie prevents JS read but adds CSRF surface; memory-only is cleaner (see ADR-0006) |

---

## Consequences

**Positive:**
- Stateless access token verification (no DB hit on every request)
- Refresh tokens can be revoked instantly
- Refresh token value is never persisted anywhere readable
- Scales horizontally — any backend instance can verify JWTs with the shared secret

**Negative:**
- Access tokens cannot be individually revoked before expiry (only refresh rotation forces re-auth)
- 15-minute window means a stolen access token is valid for up to 15 minutes
- Requires the frontend to handle 401 → refresh → retry silently (see ADR-0006)

---

## Related decisions

- [ADR-0006](0006-token-storage-strategy.md) — how the frontend stores and uses these tokens
- [ADR-0002](0002-database-and-cache.md) — sessions table in PostgreSQL; rate limiting in Redis
