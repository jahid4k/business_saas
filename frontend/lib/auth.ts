/**
 * lib/auth.ts — Auth state for httpOnly cookie token architecture.
 *
 * TOKEN ARCHITECTURE:
 * ─────────────────────────────────────────────────────────────────
 * Access token  → stored in MEMORY only (this module's variable).
 *                 Never written to localStorage or any JS-accessible storage.
 *                 Lost on page refresh — re-acquired via /auth/refresh.
 *
 * Refresh token → stored in httpOnly cookie set by the BACKEND.
 *                 The frontend JavaScript NEVER reads or writes it.
 *                 The browser sends it automatically on every request
 *                 to the backend when withCredentials: true.
 *
 * On page load → call /auth/refresh (cookie sent automatically)
 *               → backend validates cookie, returns new access token
 *               → store in memory, user is logged in silently
 *
 * BACKEND REQUIREMENTS (Phase 1-B):
 *   POST /api/v1/auth/login
 *     Response: { access_token: "..." }  (body)
 *               Set-Cookie: bsaas_refresh=<token>; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth
 *
 *   POST /api/v1/auth/refresh
 *     Request:  Cookie: bsaas_refresh=<token>  (automatic)
 *     Response: { access_token: "..." }  (body)
 *               Set-Cookie: bsaas_refresh=<new_token>; HttpOnly; ...  (rotation)
 *
 *   POST /api/v1/auth/logout
 *     Response: Set-Cookie: bsaas_refresh=; HttpOnly; Max-Age=0  (clear cookie)
 * ─────────────────────────────────────────────────────────────────
 *
 * EVENT BUS:
 * setAccessToken() dispatches a "bsaas:token-set" CustomEvent on window.
 * usePermission subscribes to this event so it can update the role
 * reactively when the silent refresh sets the token after mount —
 * without polling, without setState-in-useEffect, without cascading renders.
 */

import type { JwtClaims } from "@/types/auth";

// In-memory access token — survives re-renders, lost on page refresh (by design)
let _accessToken: string | null = null;

// Custom event name — used to notify subscribers when the token changes
const TOKEN_SET_EVENT = "bsaas:token-set";

// ------------------------------------------------------------------
// In-memory access token management
// ------------------------------------------------------------------

export function getAccessToken(): string | null {
  return _accessToken;
}

export function setAccessToken(token: string): void {
  _accessToken = token;

  // Notify subscribers (usePermission, any other hook that cares)
  // Only dispatch in the browser — this module is also imported server-side
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(TOKEN_SET_EVENT));
  }
}

export function clearAccessToken(): void {
  _accessToken = null;

  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(TOKEN_SET_EVENT));
  }
}

// Token set event name — exported so subscribers can use the same constant
export { TOKEN_SET_EVENT };

// ------------------------------------------------------------------
// JWT decode — for UI display only, NOT for auth decisions.
// The backend re-validates the token on every request regardless.
// ------------------------------------------------------------------

export function decodeAccessToken(token?: string | null): JwtClaims | null {
  const t = token ?? _accessToken;
  if (!t) return null;

  try {
    const parts = t.split(".");
    if (parts.length !== 3) return null;
    const payload = parts[1];
    const padded = payload + "=".repeat((4 - (payload.length % 4)) % 4);
    return JSON.parse(atob(padded)) as JwtClaims;
  } catch {
    return null;
  }
}

// ------------------------------------------------------------------
// Auth state checks — based on in-memory token
// ------------------------------------------------------------------

export function isAuthenticated(): boolean {
  if (!_accessToken) return false;
  const claims = decodeAccessToken(_accessToken);
  if (!claims) return false;
  return claims.exp * 1000 > Date.now();
}

export function getCurrentUserID(): string | null {
  return decodeAccessToken()?.uid ?? null;
}

export function getCurrentBusinessID(): string | null {
  return decodeAccessToken()?.bid ?? null;
}

export function getCurrentRole(): string | null {
  return decodeAccessToken()?.role ?? null;
}
