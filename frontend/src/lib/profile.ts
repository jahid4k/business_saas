// src/lib/profile.ts
import api from "./api";
import type { SafeUser } from "@/types/auth";

export interface UpdateProfilePayload {
  displayName?: string;
  firstName?: string;
  lastName?: string;
  timezone?: string;
  language?: string;
}

// GET /api/v1/me → data.user (full profile, fresher than authStore)
export async function getProfile(): Promise<SafeUser> {
  const res = await api.get<{ success: boolean; data: { user: SafeUser } }>(
    "/api/v1/me",
  );
  return res.data.data.user;
}

// PATCH /api/v1/me → data.user
export async function updateProfile(
  body: UpdateProfilePayload,
): Promise<SafeUser> {
  const res = await api.patch<{ success: boolean; data: { user: SafeUser } }>(
    "/api/v1/me",
    body,
  );
  return res.data.data.user;
}

// POST /api/v1/me/avatar (multipart/form-data) → data.user
export async function uploadAvatar(file: File): Promise<SafeUser> {
  const form = new FormData();
  form.append("avatar", file);
  const res = await api.post<{ success: boolean; data: { user: SafeUser } }>(
    "/api/v1/me/avatar",
    form,
    { headers: { "Content-Type": "multipart/form-data" } },
  );
  return res.data.data.user;
}
