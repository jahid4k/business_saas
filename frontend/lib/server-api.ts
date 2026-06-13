/**
 * lib/server-api.ts — Server-side fetch helpers for Next.js Server Components.
 *
 * Server components run inside Docker and use the internal backend URL
 * (http://backend:8080) which is NOT accessible from the browser.
 *
 * Client components use lib/api.ts (Axios) with NEXT_PUBLIC_API_URL.
 */

const INTERNAL_URL =
  process.env.BACKEND_INTERNAL_URL ?? "http://localhost:8080";

export async function serverFetch<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(`${INTERNAL_URL}/api/v1${path}`, {
      // No caching for dynamic data
      cache: "no-store",
    });

    if (!res.ok) return null;

    const json = await res.json();
    return json as T;
  } catch {
    return null;
  }
}
