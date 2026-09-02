// src/types/notification.ts

export interface Notification {
  id: string;
  org_id?: string | null;
  user_id: string;
  event_type: string;
  channel: string;
  title: string;
  body: string;
  action_url?: string | null;
  metadata?: string | null;
  status: string;
  error_message?: string | null;
  read_at?: string | null;
  sent_at?: string | null;
  created_at: string;
}

// GET /api/v1/notifications → data: { notifications, total, unread_count }
export interface NotificationListResponse {
  notifications: Notification[];
  total: number;
  unread_count: number;
}

export interface NotificationPreference {
  id: string;
  user_id: string;
  event_type: string;
  channel: string;
  is_enabled: boolean;
  updated_at: string;
}
