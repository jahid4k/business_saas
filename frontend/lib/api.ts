// lib/api.ts
// The single HTTP client for all backend calls.
//
// Design (ADR-0006):
//   - Access token stored in module-level variable (_accessToken) — never localStorage
//   - Every request injects: Authorization: Bearer <token>
//   - On 401: silent refresh via httpOnly cookie → retry once
//   - On refresh failure: redirect to /login
//   - All non-2xx responses throw ApiError (never raw Response)

import ky, { type KyResponse, type Options } from "ky";
import { ApiError } from "@/types/api";

// ----------------------------------------------------------
// In-memory token store (ADR-0006)
// ----------------------------------------------------------

let _accessToken: string | null = null;
let _refreshPromise: Promise<boolean> | null = null;

export function setAccessToken(token: string): void {
  _accessToken = token;
}

export function clearAccessToken(): void {
  _accessToken = null;
}

export function getAccessToken(): string | null {
  return _accessToken;
}

// ----------------------------------------------------------
// Silent refresh
// ----------------------------------------------------------

async function attemptSilentRefresh(): Promise<boolean> {
  // Deduplicate: if a refresh is already in-flight, wait for it
  if (_refreshPromise) return _refreshPromise;

  _refreshPromise = (async () => {
    try {
      const baseUrl =
        typeof window !== "undefined"
          ? process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
          : process.env.BACKEND_INTERNAL_URL || "http://localhost:8080";

      const res = await fetch(`${baseUrl}/api/v1/auth/refresh`, {
        method: "POST",
        credentials: "include", // sends httpOnly bsaas_refresh cookie
        headers: { "Content-Type": "application/json" },
      });

      if (!res.ok) return false;

      const body = await res.json();
      const token = body?.data?.access_token;
      if (!token) return false;

      setAccessToken(token);
      return true;
    } catch {
      return false;
    } finally {
      _refreshPromise = null;
    }
  })();

  return _refreshPromise;
}

// ----------------------------------------------------------
// Error normalizer — converts any failed response to ApiError
// ----------------------------------------------------------

async function toApiError(response: KyResponse): Promise<ApiError> {
  let code = "UNKNOWN_ERROR";
  let message = "An unexpected error occurred";
  let fields: Record<string, string> | undefined;
  let requestId: string | undefined;

  try {
    const body = await response.json<{
      success: boolean;
      error?: { code: string; message: string; fields?: Record<string, string> };
      request_id?: string;
    }>();

    if (body.error) {
      code = body.error.code ?? code;
      message = body.error.message ?? message;
      fields = body.error.fields;
    }
    requestId = body.request_id;
  } catch {
    // response body wasn't JSON — use HTTP status text
    message = response.statusText || message;
  }

  return new ApiError({
    code,
    message,
    status: response.status,
    fields,
    requestId,
  });
}

// ----------------------------------------------------------
// ky instance
// ----------------------------------------------------------

const baseUrl =
  typeof window !== "undefined"
    ? process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
    : process.env.BACKEND_INTERNAL_URL || "http://localhost:8080";

export const api = ky.create({
  prefixUrl: baseUrl,
  credentials: "include", // always include cookies (for refresh)
  timeout: 30_000,
  retry: 0, // we handle retry manually in afterResponse
  hooks: {
    beforeRequest: [
      (request) => {
        if (_accessToken) {
          request.headers.set("Authorization", `Bearer ${_accessToken}`);
        }
      },
    ],
    afterResponse: [
      async (request, options, response) => {
        // Non-error responses — pass through
        if (response.ok) return response;

        // 401 → attempt silent refresh, then retry once
        if (response.status === 401) {
          const refreshed = await attemptSilentRefresh();

          if (refreshed) {
            // Retry original request with new token
            const retryOptions: Options = {
              ...options,
              headers: {
                ...Object.fromEntries(request.headers.entries()),
                Authorization: `Bearer ${_accessToken}`,
              },
            };
            return ky(request.url, retryOptions);
          }

          // Refresh failed — clear token and redirect to login
          clearAccessToken();
          if (typeof window !== "undefined") {
            window.location.href = "/login";
          }
          throw await toApiError(response);
        }

        // All other errors — throw ApiError
        throw await toApiError(response);
      },
    ],
  },
});

// ----------------------------------------------------------
// Typed request helpers
// Used by all feature modules. Return typed data directly (unwrap envelope).
// ----------------------------------------------------------

export async function apiGet<T>(
  path: string,
  searchParams?: Record<string, string | number | boolean | undefined>,
): Promise<T> {
  const cleanParams = searchParams
    ? Object.fromEntries(
        Object.entries(searchParams)
          .filter(([, v]) => v !== undefined)
          .map(([k, v]) => [k, String(v)]),
      )
    : undefined;

  const res = await api
    .get(path, { searchParams: cleanParams })
    .json<{ success: boolean; data?: T }>();

  return res.data as T;
}

export async function apiPost<T>(
  path: string,
  body?: unknown,
): Promise<T> {
  const res = await api
    .post(path, { json: body })
    .json<{ success: boolean; data?: T }>();
  return res.data as T;
}

export async function apiPatch<T>(
  path: string,
  body?: unknown,
): Promise<T> {
  const res = await api
    .patch(path, { json: body })
    .json<{ success: boolean; data?: T }>();
  return res.data as T;
}

export async function apiDelete<T = void>(path: string): Promise<T> {
  const response = await api.delete(path);
  if (response.status === 204) return undefined as T;
  const res = await response.json<{ success: boolean; data?: T }>();
  return res.data as T;
}
