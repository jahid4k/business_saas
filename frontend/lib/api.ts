// lib/api.ts
import ky, {
  type KyResponse,
  type BeforeRequestState,
  type AfterResponseState,
} from "ky";
import { ApiError } from "@/types/api";

// ----------------------------------------------------------
// In-memory token store
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
  if (_refreshPromise) return _refreshPromise;

  _refreshPromise = (async () => {
    try {
      const baseUrl =
        typeof window !== "undefined"
          ? process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
          : process.env.BACKEND_INTERNAL_URL || "http://localhost:8080";

      const res = await fetch(`${baseUrl}/api/v1/auth/refresh`, {
        method: "POST",
        credentials: "include",
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
// Error normalizer
// ----------------------------------------------------------

async function toApiError(response: KyResponse): Promise<ApiError> {
  let code = "UNKNOWN_ERROR";
  let message = "An unexpected error occurred";
  let fields: Record<string, string> | undefined;
  let requestId: string | undefined;

  try {
    const body = await response.json<{
      success: boolean;
      error?: {
        code: string;
        message: string;
        fields?: Record<string, string>;
      };
      request_id?: string;
    }>();

    if (body.error) {
      code = body.error.code ?? code;
      message = body.error.message ?? message;
      fields = body.error.fields;
    }
    requestId = body.request_id;
  } catch {
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
// ky instance — v2 correct hook signatures
// ----------------------------------------------------------

const baseUrl =
  typeof window !== "undefined"
    ? process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
    : process.env.BACKEND_INTERNAL_URL || "http://localhost:8080";

export const api = ky.create({
  baseUrl: baseUrl,
  credentials: "include",
  timeout: 30_000,
  retry: 0,
  hooks: {
    // ✅ ky v2: state object — access request via state.request
    beforeRequest: [
      ({ request }: BeforeRequestState) => {
        if (_accessToken) {
          request.headers.set("Authorization", `Bearer ${_accessToken}`);
        }
      },
    ],

    // ✅ ky v2: state object — access via state.request, state.response, state.options
    afterResponse: [
      async ({ request, response }: AfterResponseState) => {
        if (response.ok) return response;

        if (response.status === 401) {
          const refreshed = await attemptSilentRefresh();

          if (refreshed) {
            // ✅ ky v2: retry with ky.retry() — not raw ky() call
            const newHeaders = new Headers(request.headers);
            newHeaders.set("Authorization", `Bearer ${_accessToken}`);

            return ky.retry({
              request: new Request(request, { headers: newHeaders }),
              code: "TOKEN_REFRESHED",
            });
          }

          clearAccessToken();
          if (typeof window !== "undefined") {
            window.location.href = "/login";
          }
          throw await toApiError(response);
        }

        throw await toApiError(response);
      },
    ],
  },
});

// ----------------------------------------------------------
// Typed request helpers
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

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const res = await api
    .post(path, { json: body })
    .json<{ success: boolean; data?: T }>();
  return res.data as T;
}

export async function apiPatch<T>(path: string, body?: unknown): Promise<T> {
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
