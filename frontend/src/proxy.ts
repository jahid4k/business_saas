// src/proxy.ts
// ─── Minimal: only handles root redirect ─────────────────────────────
// Why minimal? bsaas_refresh cookie has path=/api/v1/auth — browser only
// sends it to API calls, never to Next.js page requests. So middleware
// cannot detect auth state. AuthProvider handles all auth checks client-side.

import { NextRequest, NextResponse } from "next/server";

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (pathname === "/login") {
    return NextResponse.redirect(new URL("/", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/"],
};
