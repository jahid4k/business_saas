// src/lib/api.ts
import axios, { AxiosError, InternalAxiosRequestConfig } from "axios";
import { getToken, setToken } from "./token";
import { API_BASE_URL } from "./constants";

interface RetryConfig extends InternalAxiosRequestConfig {
  _retry?: boolean;
}

const api = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true, // required: sends httpOnly bsaas_refresh cookie
  headers: { "Content-Type": "application/json" },
});

// ── Request: inject Bearer token ─────────────────────────────────────
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// ── Response: silent refresh on 401 ─────────────────────────────────
api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as RetryConfig | undefined;
    if (!original) return Promise.reject(error);

    // Critical: never retry the refresh endpoint — infinite loop otherwise
    const isRefreshCall = original.url?.includes("/auth/refresh");

    if (error.response?.status === 401 && !original._retry && !isRefreshCall) {
      original._retry = true;
      try {
        const res = await api.post<{
          success: boolean;
          data: { access_token: string };
        }>("/api/v1/auth/refresh");
        const newToken = res.data.data.access_token;
        setToken(newToken);
        original.headers.Authorization = `Bearer ${newToken}`;
        return api(original);
      } catch {
        setToken(null);
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
      }
    }

    return Promise.reject(error);
  },
);

export default api;
