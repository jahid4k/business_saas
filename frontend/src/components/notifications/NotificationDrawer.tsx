// src/components/notifications/NotificationDrawer.tsx
"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCheck, Bell as BellIcon } from "lucide-react";
import {
  listNotifications,
  markNotificationRead,
  markAllNotificationsRead,
} from "@/lib/notifications";
import { queryKeys } from "@/lib/queryKeys";
import type { NotificationListResponse } from "@/types/notification";

function timeAgo(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diffMs / 60_000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

// Content-only — rendered inside the shared <Drawer> shell via useDrawer().openDrawer(),
// which already provides the title bar and close button.
export default function NotificationDrawer() {
  const queryClient = useQueryClient();
  const listKey = queryKeys.notifications.list();

  const query = useQuery({
    queryKey: listKey,
    queryFn: () => listNotifications({ limit: 50 }),
  });

  const notifications = query.data?.notifications ?? [];
  const unreadCount = query.data?.unread_count ?? 0;

  const handleMarkRead = async (id: string) => {
    queryClient.setQueryData<NotificationListResponse | undefined>(
      listKey,
      (old) => {
        if (!old) return old;
        const target = old.notifications.find((n) => n.id === id);
        if (!target || target.read_at) return old;
        return {
          ...old,
          notifications: old.notifications.map((n) =>
            n.id === id ? { ...n, read_at: new Date().toISOString() } : n,
          ),
          unread_count: Math.max(0, old.unread_count - 1),
        };
      },
    );
    try {
      await markNotificationRead(id);
    } catch {
      query.refetch();
    }
  };

  const handleMarkAllRead = async () => {
    queryClient.setQueryData<NotificationListResponse | undefined>(
      listKey,
      (old) =>
        old && {
          ...old,
          notifications: old.notifications.map((n) =>
            n.read_at ? n : { ...n, read_at: new Date().toISOString() },
          ),
          unread_count: 0,
        },
    );
    try {
      await markAllNotificationsRead();
    } catch {
      query.refetch();
    }
  };

  return (
    <>
      <div className="flex items-center justify-between px-6 py-3 border-b border-(--border) shrink-0">
        <span className="text-xs text-(--text-muted)">
          {unreadCount > 0 ? `${unreadCount} unread` : "All caught up"}
        </span>
        {unreadCount > 0 && (
          <button
            onClick={handleMarkAllRead}
            className="flex items-center gap-1.5 text-xs font-medium text-(--text-secondary) hover:text-(--text-primary) transition-colors"
          >
            <CheckCheck size={13} />
            Mark all read
          </button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto">
        {query.isLoading ? (
          <div className="px-6 py-8 text-sm text-(--text-muted) text-center">
            Loading…
          </div>
        ) : notifications.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
            <BellIcon size={28} className="text-(--text-muted)" />
            <p className="text-sm text-(--text-muted)">No notifications yet</p>
          </div>
        ) : (
          <ul className="divide-y divide-(--border)">
            {notifications.map((n) => (
              <li key={n.id}>
                <button
                  onClick={() => !n.read_at && handleMarkRead(n.id)}
                  className={`w-full text-left px-6 py-3.5 transition-colors hover:bg-(--bg-elevated) ${
                    n.read_at ? "" : "bg-purple-500/5"
                  }`}
                >
                  <div className="flex items-start gap-2">
                    {!n.read_at && (
                      <span className="mt-1.5 h-1.5 w-1.5 rounded-full bg-purple-500 shrink-0" />
                    )}
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-(--text-primary)">
                        {n.title}
                      </p>
                      <p className="text-xs text-(--text-muted) mt-0.5 line-clamp-2">
                        {n.body}
                      </p>
                      <p className="text-[11px] text-(--text-muted) mt-1">
                        {timeAgo(n.created_at)}
                      </p>
                    </div>
                  </div>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </>
  );
}
