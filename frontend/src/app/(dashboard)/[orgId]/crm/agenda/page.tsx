// src/app/(dashboard)/[orgId]/crm/agenda/page.tsx
"use client";

import { use, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  CalendarCheck2,
  Clock,
  CheckCircle2,
  Circle,
  AlertCircle,
  Loader2,
  CalendarClock,
  Link as LinkIcon,
  Plus,
} from "lucide-react";
import Link from "next/link";
import { getAgenda, type AgendaItem } from "@/lib/crm/reports";
import { updateTask, createTask } from "@/lib/tasks";
import type { UpdateTaskRequest } from "@/types/task";

export default function AgendaPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const queryClient = useQueryClient();
  const [newTaskTitle, setNewTaskTitle] = useState("");

  const query = useQuery({
    queryKey: ["crm", "agenda", orgId],
    queryFn: () => getAgenda(orgId),
  });

  const createMutation = useMutation({
    mutationFn: (title: string) =>
      createTask(orgId, {
        title,
        status: "todo",
        dueDate: new Date().toISOString(),
      }),
    onSuccess: () => {
      setNewTaskTitle("");
      queryClient.invalidateQueries({ queryKey: ["crm", "agenda", orgId] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, updates }: { id: string; updates: UpdateTaskRequest }) =>
      updateTask(orgId, id, updates),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["crm", "agenda", orgId] });
    },
  });

  const handleCreateTask = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && newTaskTitle.trim()) {
      createMutation.mutate(newTaskTitle.trim());
    }
  };

  const items = query.data ?? [];

  const endOfToday = new Date();
  endOfToday.setHours(23, 59, 59, 999);

  const startOfToday = new Date();
  startOfToday.setHours(0, 0, 0, 0);

  // Completed today items (backend only returns done tasks if they were updated today)
  const completedTodayItems = items.filter((i) => i.status === "done");
  const openItems = items.filter(
    (i) => i.status !== "done" && i.status !== "cancelled",
  );

  const overdueItems = openItems.filter(
    (i) =>
      i.due_date && new Date(i.due_date).getTime() < startOfToday.getTime(),
  );

  const todayItems = openItems.filter(
    (i) =>
      !i.due_date ||
      (new Date(i.due_date).getTime() >= startOfToday.getTime() &&
        new Date(i.due_date).getTime() <= endOfToday.getTime()),
  );

  const upcomingItems = openItems.filter(
    (i) => i.due_date && new Date(i.due_date).getTime() > endOfToday.getTime(),
  );

  const totalToday =
    overdueItems.length + todayItems.length + completedTodayItems.length;
  const completedCount = completedTodayItems.length;
  const progressPercent =
    totalToday === 0 ? 0 : Math.round((completedCount / totalToday) * 100);

  return (
    <div className="flex flex-col h-full bg-[var(--bg-canvas)]">
      <div className="shrink-0 border-b border-[var(--border)] bg-[var(--bg-surface)] px-8 py-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-xl font-semibold text-[var(--text-primary)]">
              Today&apos;s Agenda
            </h1>
            <p className="mt-1.5 text-sm text-[var(--text-secondary)] max-w-2xl">
              Your high-priority tasks and overdue activities to focus on today.
            </p>
          </div>

          {/* Progress Indicator */}
          <div className="flex flex-col items-end gap-2 w-48">
            <div className="flex items-center justify-between w-full">
              <span className="text-sm font-medium text-[var(--text-secondary)]">
                Daily Progress
              </span>
              <span className="text-sm font-bold text-[var(--text-primary)]">
                {completedCount} / {totalToday}
              </span>
            </div>
            <div className="h-2 w-full bg-gray-200 dark:bg-gray-800 rounded-full overflow-hidden">
              <div
                className="h-full bg-purple-600 dark:bg-purple-500 transition-all duration-500 ease-out"
                style={{ width: `${progressPercent}%` }}
              />
            </div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto p-8">
        <div className="mx-auto max-w-4xl space-y-8">
          {/* Add Task Input */}
          <div className="relative">
            <Plus
              className="absolute left-4 top-1/2 -translate-y-1/2 text-[var(--text-muted)]"
              size={20}
            />
            <input
              type="text"
              placeholder="Add a new task for today... (Press Enter)"
              value={newTaskTitle}
              onChange={(e) => setNewTaskTitle(e.target.value)}
              onKeyDown={handleCreateTask}
              disabled={createMutation.isPending}
              className="w-full pl-12 pr-4 py-3 bg-[var(--bg-surface)] border border-[var(--border)] rounded-xl text-sm placeholder:text-[var(--text-muted)] text-[var(--text-primary)] focus:outline-none focus:ring-2 focus:ring-purple-500/50 shadow-sm"
            />
          </div>

          {query.isLoading ? (
            <div className="flex justify-center p-12">
              <Loader2 className="animate-spin text-purple-600" size={32} />
            </div>
          ) : totalToday === 0 && upcomingItems.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-16 text-center bg-[var(--bg-surface)] rounded-xl border border-[var(--border)] border-dashed">
              <div className="h-12 w-12 rounded-full bg-purple-50 dark:bg-purple-500/10 flex items-center justify-center mb-4">
                <CalendarCheck2
                  className="text-purple-600 dark:text-purple-400"
                  size={24}
                />
              </div>
              <h3 className="text-base font-medium text-[var(--text-primary)] mb-1">
                You&apos;re all caught up!
              </h3>
              <p className="text-sm text-[var(--text-secondary)] max-w-sm">
                There are no tasks or activities scheduled.
              </p>
            </div>
          ) : (
            <>
              {overdueItems.length > 0 && (
                <section>
                  <h2 className="text-lg font-medium text-red-600 dark:text-red-400 mb-4 flex items-center gap-2">
                    <AlertCircle size={18} />
                    Overdue
                  </h2>
                  <div className="bg-[var(--bg-surface)] border border-red-200 dark:border-red-500/20 rounded-xl overflow-hidden shadow-sm divide-y divide-[var(--border)]">
                    {overdueItems.map((item) => (
                      <AgendaItemRow
                        key={item.id}
                        item={item}
                        orgId={orgId}
                        onUpdate={(updates) =>
                          updateMutation.mutate({ id: item.id, updates })
                        }
                      />
                    ))}
                  </div>
                </section>
              )}

              {(todayItems.length > 0 || completedTodayItems.length > 0) && (
                <section>
                  <h2 className="text-lg font-medium text-[var(--text-primary)] mb-4 flex items-center gap-2">
                    <Clock
                      size={18}
                      className="text-purple-600 dark:text-purple-400"
                    />
                    Today
                  </h2>
                  <div className="bg-[var(--bg-surface)] border border-[var(--border)] rounded-xl overflow-hidden shadow-sm divide-y divide-[var(--border)]">
                    {todayItems.map((item) => (
                      <AgendaItemRow
                        key={item.id}
                        item={item}
                        orgId={orgId}
                        onUpdate={(updates) =>
                          updateMutation.mutate({ id: item.id, updates })
                        }
                      />
                    ))}
                    {completedTodayItems.map((item) => (
                      <AgendaItemRow
                        key={item.id}
                        item={item}
                        orgId={orgId}
                        onUpdate={(updates) =>
                          updateMutation.mutate({ id: item.id, updates })
                        }
                      />
                    ))}
                  </div>
                </section>
              )}

              {upcomingItems.length > 0 && (
                <section>
                  <h2 className="text-lg font-medium text-[var(--text-primary)] mb-4 flex items-center gap-2 mt-8">
                    <CalendarClock size={18} className="text-gray-500" />
                    Upcoming
                  </h2>
                  <div className="bg-[var(--bg-surface)] border border-[var(--border)] rounded-xl overflow-hidden shadow-sm divide-y divide-[var(--border)]">
                    {upcomingItems.map((item) => (
                      <AgendaItemRow
                        key={item.id}
                        item={item}
                        orgId={orgId}
                        onUpdate={(updates) =>
                          updateMutation.mutate({ id: item.id, updates })
                        }
                      />
                    ))}
                  </div>
                </section>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function AgendaItemRow({
  item,
  orgId,
  onUpdate,
}: {
  item: AgendaItem;
  orgId: string;
  onUpdate: (updates: UpdateTaskRequest) => void;
}) {
  const isDone = item.status === "done";

  const getEntityUrl = () => {
    if (!item.related_type || !item.related_id) return "#";
    const type = item.related_type.toLowerCase();
    if (type === "lead") return `/${orgId}/crm/leads/${item.related_id}`;
    if (type === "contact") return `/${orgId}/crm/contacts/${item.related_id}`;
    if (type === "deal") return `/${orgId}/crm/pipeline`; // Specific deal deep-link usually handled via query params in pipelines
    if (type === "company") return `/${orgId}/crm/companies/${item.related_id}`;
    return "#";
  };

  const handleSnooze = () => {
    const tmrw = new Date();
    tmrw.setDate(tmrw.getDate() + 1);
    onUpdate({ dueDate: tmrw.toISOString() });
  };

  return (
    <div
      className={`group flex items-start gap-4 p-4 hover:bg-gray-50/50 dark:hover:bg-white/[0.02] transition-colors ${isDone ? "opacity-60" : ""}`}
    >
      <button
        onClick={() => onUpdate({ status: isDone ? "todo" : "done" })}
        className="pt-0.5 shrink-0 text-[var(--text-muted)] hover:text-purple-600 transition-colors focus:outline-none"
      >
        {isDone ? (
          <CheckCircle2 size={20} className="text-emerald-500" />
        ) : item.status === "in_progress" ? (
          <Clock size={20} className="text-amber-500" />
        ) : (
          <Circle size={20} />
        )}
      </button>

      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-4 mb-1">
          <h4
            className={`font-medium text-[var(--text-primary)] truncate transition-all ${isDone ? "line-through text-[var(--text-muted)]" : ""}`}
          >
            {item.title}
          </h4>
          <div className="flex items-center gap-3">
            {item.due_date && (
              <span className="shrink-0 text-xs font-medium text-[var(--text-secondary)] bg-gray-100 dark:bg-white/5 px-2 py-1 rounded-md">
                {new Date(item.due_date).toLocaleDateString("en-US", {
                  month: "short",
                  day: "numeric",
                })}
              </span>
            )}

            {/* Quick Actions (Hover) */}
            {!isDone && (
              <button
                onClick={handleSnooze}
                className="opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-[var(--text-muted)] hover:text-purple-600 focus:outline-none bg-gray-100 dark:bg-white/10 rounded px-2 py-1 text-xs font-medium"
                title="Snooze to Tomorrow"
              >
                Snooze
              </button>
            )}
          </div>
        </div>

        {item.description && (
          <p className="text-sm text-[var(--text-secondary)] line-clamp-2 mb-2">
            {item.description}
          </p>
        )}

        {item.related_type && (
          <div className="flex items-center gap-2 mt-2">
            <Link
              href={getEntityUrl()}
              className="inline-flex items-center gap-1 rounded-full bg-blue-50 dark:bg-blue-500/10 px-2.5 py-1 text-[10px] font-semibold text-blue-700 dark:text-blue-400 uppercase tracking-wider hover:bg-blue-100 dark:hover:bg-blue-500/20 transition-colors"
            >
              <LinkIcon size={10} />
              {item.related_type}
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
