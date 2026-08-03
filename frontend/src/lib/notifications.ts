// src/lib/notifications.ts
import api from "./api";
import type {
  NotificationListResponse,
  NotificationPreference,
} from "@/types/notification";

// Not org-scoped — /api/v1/notifications is per-user, like /api/v1/me.

export async function listNotifications(params?: {
  limit?: number;
  offset?: number;
}): Promise<NotificationListResponse> {
  const res = await api.get<{ success: boolean; data: NotificationListResponse }>(
    `/api/v1/notifications`,
    { params },
  );
  return res.data.data;
}

export async function markNotificationRead(id: string): Promise<void> {
  await api.post(`/api/v1/notifications/${id}/read`);
}

export async function markAllNotificationsRead(): Promise<void> {
  await api.post(`/api/v1/notifications/read-all`);
}

export async function getNotificationPreferences(): Promise<
  NotificationPreference[]
> {
  const res = await api.get<{
    success: boolean;
    data: { preferences: NotificationPreference[] };
  }>(`/api/v1/notifications/preferences`);
  return res.data.data.preferences;
}

export async function updateNotificationPreference(
  eventType: string,
  channel: string,
  isEnabled: boolean,
): Promise<void> {
  await api.patch(`/api/v1/notifications/preferences`, {
    event_type: eventType,
    channel,
    is_enabled: isEnabled,
  });
}
