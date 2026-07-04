// src/lib/profile.ts
import api from "./api";
import type { SafeUser, UserAvatar } from "@/types/auth";

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

export interface UploadAvatarResult {
  user: SafeUser;
  avatar: UserAvatar;
}

// POST /api/v1/me/avatar (multipart/form-data) → data.{user, avatar}
// `blob` is expected to already be cropped/resized client-side (see
// AvatarCropModal) — the backend independently re-validates, center-crops,
// and resizes regardless, but sending a reasonably-sized square from the
// start keeps the upload itself fast.
export async function uploadAvatar(blob: Blob): Promise<UploadAvatarResult> {
  const form = new FormData();
  form.append("avatar", blob, "avatar.jpg");
  const res = await api.post<{
    success: boolean;
    data: UploadAvatarResult;
  }>("/api/v1/me/avatar", form, {
    headers: { "Content-Type": "multipart/form-data" },
  });
  return res.data.data;
}

// GET /api/v1/me/avatars → data.avatars (0 to MaxAvatarsPerUser, newest first)
export async function listAvatars(): Promise<{
  avatars: UserAvatar[];
  max: number;
}> {
  const res = await api.get<{
    success: boolean;
    data: { avatars: UserAvatar[]; max: number };
  }>("/api/v1/me/avatars");
  return res.data.data;
}

// POST /api/v1/me/avatars/:avatarId/activate → data.user
// "Quick-switch" — makes an already-stored avatar the active one without
// uploading anything or consuming a new slot.
export async function activateAvatar(avatarId: string): Promise<SafeUser> {
  const res = await api.post<{ success: boolean; data: { user: SafeUser } }>(
    `/api/v1/me/avatars/${avatarId}/activate`,
  );
  return res.data.data.user;
}

// DELETE /api/v1/me/avatars/:avatarId → data.user
// If the deleted avatar was active, the backend promotes the most recently
// uploaded remaining one automatically — the returned user reflects that.
export async function deleteAvatar(avatarId: string): Promise<SafeUser> {
  const res = await api.delete<{ success: boolean; data: { user: SafeUser } }>(
    `/api/v1/me/avatars/${avatarId}`,
  );
  return res.data.data.user;
}

// Shape of backend/pkg/response's error envelope — used to distinguish
// AVATAR_LIMIT_REACHED / INVALID_AVATAR_TYPE from a generic failure so the
// UI can show the person something actionable instead of "something broke".
export interface ApiErrorBody {
  success: false;
  error: { code: string; message: string };
  request_id?: string;
}

export function apiErrorCode(err: unknown): string | undefined {
  if (
    typeof err === "object" &&
    err !== null &&
    "response" in err &&
    typeof (err as { response?: unknown }).response === "object"
  ) {
    const response = (err as { response?: { data?: unknown } }).response;
    const data = response?.data as Partial<ApiErrorBody> | undefined;
    return data?.error?.code;
  }
  return undefined;
}
