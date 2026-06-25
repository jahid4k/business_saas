// src/lib/jwt.ts
// Client-side JWT payload reader.
// NOT for security — the server validates signatures.
// Used ONLY to read claims (uid, bid, role) for client-side UX decisions.

interface JWTPayload {
  uid?: string; // user ID
  bid?: string; // business/org ID (set after org switch)
  email?: string;
  role?: string;
  exp?: number;
  iat?: number;
  iss?: string;
  sub?: string;
}

export function decodeToken(token: string): JWTPayload {
  try {
    // JWT uses base64url: replace - with + and _ with /
    const base64Url = token.split(".")[1];
    if (!base64Url) return {};
    const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    const json = decodeURIComponent(
      atob(base64)
        .split("")
        .map((c) => "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2))
        .join(""),
    );
    return JSON.parse(json) as JWTPayload;
  } catch {
    return {};
  }
}
