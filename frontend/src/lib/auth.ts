// src/lib/auth.ts
import api from "./api";
import type { SafeUser, ClientTokenPair, SignupRequest } from "@/types/auth";

// ── Important response shapes (confirmed from backend handler.go) ────
// POST /auth/login  → { success, data: ClientTokenPair }
// POST /auth/refresh → { success, data: ClientTokenPair }
// GET  /auth/me     → { success, data: { user: SafeUser } }  ← user is NESTED

export async function login(
  email: string,
  password: string,
): Promise<ClientTokenPair> {
  const res = await api.post<{ success: boolean; data: ClientTokenPair }>(
    "/api/v1/auth/login",
    { email: email.trim().toLowerCase(), password },
  );
  return res.data.data;
}

export async function silentRefresh(): Promise<string> {
  const res = await api.post<{ success: boolean; data: ClientTokenPair }>(
    "/api/v1/auth/refresh",
  );
  return res.data.data.access_token;
}

export async function getMe(): Promise<SafeUser> {
  const res = await api.get<{ success: boolean; data: { user: SafeUser } }>(
    "/api/v1/auth/me",
  );
  return res.data.data.user; // ← .user is required, not .data directly
}

export async function signup(body: SignupRequest): Promise<SafeUser> {
  const res = await api.post<{ success: boolean; data: { user: SafeUser } }>(
    "/api/v1/auth/signup",
    body,
  );
  return res.data.data.user;
}

export async function logout(): Promise<void> {
  await api.post("/api/v1/auth/logout");
}

export async function logoutAll(): Promise<void> {
  await api.post("/api/v1/auth/logout-all");
}
