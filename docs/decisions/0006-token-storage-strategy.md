# ADR-0006: Token storage — memory-only access token

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

The Go backend issues two tokens on login (see ADR-0003):
- An **access token** (JWT, 15-minute TTL) — returned in the response body
- A **refresh token** (opaque, 7-day TTL) — set as an HttpOnly cookie

The frontend must decide where to store the access token between requests. This is one of the
most security-critical frontend decisions in a SaaS product.

Three options exist: localStorage, a readable cookie, or JavaScript memory (a variable).
This decision evaluates all three against the two primary frontend token attacks.

---

## Threat model

### XSS (Cross-Site Scripting)

An attacker injects malicious JavaScript into the page (via a dependency, a stored comment,
or a reflected parameter). The script runs with full access to `document.cookie` (for readable
cookies) and `localStorage`. It can exfiltrate any data readable by JavaScript.

**Defence:** Store the access token where JavaScript cannot read it.

### CSRF (Cross-Site Request Forgery)

An attacker tricks the user's browser into making a request to the API from a different origin.
The browser automatically sends cookies with the request, so if the auth token is in a cookie,
the forged request carries it.

**Defence:** Use `SameSite=Strict` on cookies (browser won't send cross-site) and/or a CSRF
token header. Memory-stored tokens are never sent automatically — they must be explicitly
added to the `Authorization` header, which only same-origin JavaScript can do.

---

## Decision

Store the access token **in JavaScript memory only** — a module-scoped variable in `lib/api.ts`.
Never write it to `localStorage`, `sessionStorage`, or any cookie readable by JavaScript.

Use **next-auth v5** as a thin session layer that handles CSRF protection and the refresh flow,
but configure it so that the JWT access token from the Go backend lives in memory, not in the
next-auth encrypted session cookie.

The **refresh token** stays exactly as the Go backend set it: `HttpOnly; Secure; SameSite=Strict`.
The browser sends it automatically to `/api/v1/auth/refresh`. JavaScript cannot read it.

---

## Implementation pattern

```typescript
// lib/api.ts
let _accessToken: string | null = null

export function setAccessToken(token: string) {
  _accessToken = token
}

export function clearAccessToken() {
  _accessToken = null
}

// ky v2 hook: inject token into every request
const api = ky.create({
  prefixUrl: process.env.NEXT_PUBLIC_API_URL,
  hooks: {
    beforeRequest: [
      (request) => {
        if (_accessToken) {
          request.headers.set('Authorization', `Bearer ${_accessToken}`)
        }
      }
    ],
    afterResponse: [
      async (_request, _options, response) => {
        if (response.status === 401) {
          // Attempt silent refresh
          const refreshed = await attemptRefresh()
          if (refreshed) {
            // Retry the original request with new token
            return api(_request)
          }
          // Refresh failed — redirect to login
          clearAccessToken()
          window.location.href = '/login'
        }
      }
    ]
  }
})

async function attemptRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/v1/auth/refresh`, {
      method: 'POST',
      credentials: 'include', // sends httpOnly refresh cookie
    })
    if (!res.ok) return false
    const data = await res.json()
    setAccessToken(data.data.access_token)
    return true
  } catch {
    return false
  }
}
```

### On app boot (page load / tab open)

```typescript
// app/layout.tsx or a root AuthProvider
// On mount, silently refresh to get an access token from the existing httpOnly cookie
useEffect(() => {
  attemptRefresh() // if cookie exists and is valid, token is loaded into memory
}, [])
```

---

## Security comparison

| Storage location | XSS can steal | CSRF risk | Survives tab close | Chosen |
|------------------|:-------------:|:---------:|:------------------:|:------:|
| localStorage | Yes | No | Yes | No |
| Readable cookie | Yes | Yes | Yes | No |
| HttpOnly cookie | No | Yes (needs SameSite) | Yes | No |
| Memory (JS variable) | No | No | No | **Yes** |

---

## The UX tradeoff: tab close = re-auth required

When the user closes the browser tab, the access token in memory is lost. When they return,
`attemptRefresh()` runs automatically on page load. If the httpOnly refresh cookie is still
valid (7-day TTL), a new access token is issued silently — the user never sees a login screen.

The user only sees a login screen if:
- Their refresh token has expired (7 days of inactivity), or
- They explicitly logged out, or
- An admin revoked their session

For a business SaaS, this is the correct behaviour. Users expect to stay logged in across
sessions (cookie persists), but a compromised access token is only valid for 15 minutes.

---

## next-auth v5 role in this setup

next-auth v5 is used for:
- CSRF token management (built-in double-submit cookie pattern)
- Middleware integration (`middleware.ts` checks session to redirect unauthenticated users)
- Storing non-sensitive session data (user name, email, active org slug for UI display)

next-auth v5 is **not** used to store the Go backend's access token. The session cookie that
next-auth manages contains only display data, not auth credentials. The actual JWT used for
API calls lives in memory, as described above.

This means: even if the next-auth session cookie were compromised, it would not grant API access.
API access requires the in-memory JWT, which is gone when the tab closes.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| localStorage for access token | XSS readable — one injected script exfiltrates token permanently |
| Store access token in next-auth session cookie | Adds next-auth secret as a new attack vector; HttpOnly but 7-day TTL same as refresh |
| Store access token in readable cookie | XSS readable, CSRF exploitable without SameSite |
| BFF pattern (all requests via Next.js server) | Eliminates client-side token entirely; significant complexity; deferred to future |

---

## Consequences

**Positive:**
- Access token is invisible to XSS attacks
- No CSRF risk — token only sent via explicit `Authorization` header
- Refresh is silent and automatic — user experience is seamless
- Compromised refresh token can be revoked instantly server-side

**Negative:**
- Tab close triggers a refresh round-trip on next open (fast, but still a network request)
- In-memory token is lost on hard refresh — same silent refresh handles it correctly
- Slightly more complex than "just use localStorage" — well worth the security gain

---

## Related decisions

- [ADR-0003](0003-auth-token-strategy.md) — Go backend token design (the source of these tokens)
- [ADR-0004](0004-frontend-framework.md) — Next.js App Router; `middleware.ts` uses next-auth session
