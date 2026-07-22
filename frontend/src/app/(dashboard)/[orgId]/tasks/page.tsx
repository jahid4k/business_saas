// src/app/(dashboard)/[orgId]/tasks/page.tsx
"use client";

import { use, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, CheckCircle2 } from "lucide-react";
import type { Task, TaskStatus } from "@/types/task";
import { listTasks, createTask, updateTask, deleteTask } from "@/lib/tasks";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import TaskForm from "@/components/tasks/TaskForm";
import TaskRow from "@/components/tasks/TaskRow";
import TaskGroupSection from "@/components/tasks/TaskGroupSection";
import TaskQuickAddRow from "@/components/tasks/TaskQuickAddRow";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

// ── Types & config ──────────────────────────────────────────
type FilterKey = "all" | TaskStatus;
type ViewMode = "grouped" | "flat";
type CollapsedGroups = Partial<Record<TaskStatus, boolean>>;

const FILTER_TABS: { key: FilterKey; label: string }[] = [
  { key: "all", label: "All" },
  { key: "todo", label: "Todo" },
  { key: "in_progress", label: "In Progress" },
  { key: "done", label: "Done" },
  { key: "cancelled", label: "Cancelled" },
];

const GROUP_ORDER: TaskStatus[] = ["todo", "in_progress", "done", "cancelled"];

export type StatusStyle = { label: string; dot: string; badge: string };

export const STATUS_STYLE: Record<TaskStatus, StatusStyle> = {
  todo: {
    label: "Todo",
    dot: "bg-slate-400 dark:bg-slate-500",
    badge:
      "bg-slate-100 text-slate-600 dark:bg-slate-500/15 dark:text-slate-300",
  },
  in_progress: {
    label: "In Progress",
    dot: "bg-indigo-500",
    badge:
      "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300",
  },
  done: {
    label: "Done",
    dot: "bg-emerald-500",
    badge:
      "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300",
  },
  cancelled: {
    label: "Cancelled",
    dot: "bg-red-400",
    badge: "bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-300",
  },
};

// ── Helpers ───────────────────────────────────────────────
export function formatDate(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function dueDateColor(iso?: string, status?: TaskStatus) {
  if (!iso || status === "done" || status === "cancelled")
    return "text-(--text-muted)";
  const daysLeft = (new Date(iso).getTime() - Date.now()) / 86_400_000;
  if (daysLeft < 0) return "text-red-500 dark:text-red-400";
  if (daysLeft < 3) return "text-amber-500 dark:text-amber-400";
  return "text-(--text-muted)";
}

// ── Page ──────────────────────────────────────────────────
export default function TasksPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { openDrawer } = useDrawer();
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();

  const [viewMode, setViewMode] = useState<ViewMode>("grouped");
  const [activeFilter, setActiveFilter] = useState<FilterKey>("all");
  const [collapsedGroups, setCollapsedGroups] = useState<CollapsedGroups>({});
  const [addingGroup, setAddingGroup] = useState<TaskStatus | null>(null);
  const [addingFlat, setAddingFlat] = useState(false);

  // ── Permissions ───────────────────────────────────────
  const canCreate = hasPermission("tasks.create");
  const canUpdate = hasPermission("tasks.update");
  const canDelete = hasPermission("tasks.delete");

  // ── Query ─────────────────────────────────────────────
  const tasksKey = queryKeys.tasks.list(orgId);
  const tasksQuery = useQuery({
    queryKey: tasksKey,
    queryFn: () => listTasks(orgId).then((r) => r.tasks),
  });

  const tasks = tasksQuery.data ?? [];

  const filtered =
    activeFilter === "all"
      ? tasks
      : tasks.filter((t) => t.status === activeFilter);

  const groupedTasks: Record<TaskStatus, Task[]> = {
    todo: [],
    in_progress: [],
    done: [],
    cancelled: [],
  };
  for (const t of tasks) groupedTasks[t.status].push(t);

  // ── Handlers ──────────────────────────────────────────
  const openEdit = (task: Task) => {
    openDrawer({
      title: "Edit task",
      content: (
        <TaskForm
          task={task}
          onSave={async (values) => {
            const body = {
              title: values.title,
              description: values.description || undefined,
              status: values.status,
              dueDate: values.dueDate
                ? `${values.dueDate}T00:00:00.000Z`
                : undefined,
            };
            const updated = await updateTask(orgId, task.id, body);
            queryClient.setQueryData<Task[]>(tasksKey, (old) =>
              (old ?? []).map((t) => (t.id === updated.id ? updated : t)),
            );
            toast.success("Task updated.");
          }}
        />
      ),
    });
  };

  const handleInlineUpdate = async (task: Task, updates: Partial<Task>) => {
    queryClient.setQueryData<Task[]>(tasksKey, (old) =>
      (old ?? []).map((t) => (t.id === task.id ? { ...t, ...updates } : t)),
    );
    try {
      const updated = await updateTask(orgId, task.id, updates);
      queryClient.setQueryData<Task[]>(tasksKey, (old) =>
        (old ?? []).map((t) => (t.id === updated.id ? updated : t)),
      );
    } catch {
      toast.error("Failed to update task.");
      queryClient.setQueryData<Task[]>(tasksKey, (old) =>
        (old ?? []).map((t) => (t.id === task.id ? task : t)),
      );
    }
  };

  const handleDelete = async (taskId: string) => {
    try {
      await deleteTask(orgId, taskId);
      queryClient.setQueryData<Task[]>(tasksKey, (old) =>
        (old ?? []).filter((t) => t.id !== taskId),
      );
      toast.success("Task deleted.");
    } catch {
      toast.error("Failed to delete task.");
    }
  };

  const handleQuickCreate = async (title: string, status: TaskStatus) => {
    try {
      const created = await createTask(orgId, { title, status });
      queryClient.setQueryData<Task[]>(tasksKey, (old) => [
        created,
        ...(old ?? []),
      ]);
    } catch {
      toast.error("Failed to create task.");
      throw new Error("create failed");
    }
  };

  const toggleGroupCollapse = (status: TaskStatus) => {
    setCollapsedGroups((prev) => ({ ...prev, [status]: !prev[status] }));
  };

  const handleHeaderNewTask = () => {
    if (viewMode === "grouped") {
      setCollapsedGroups((prev) => ({ ...prev, todo: false }));
      setAddingGroup("todo");
    } else {
      setAddingFlat(true);
    }
  };

  // ── Render ────────────────────────────────────────────
  return (
    <div className="p-6 md:p-8 max-w-3xl">
      {/* Page header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-(--text-primary) tracking-tight mb-1">
            Tasks
          </h1>
          <p className="text-sm text-(--text-muted) tabular-nums">
            {tasks.length} {tasks.length === 1 ? "task" : "tasks"} total
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-0.5 p-0.5 rounded-lg bg-(--bg-elevated)">
            {(["grouped", "flat"] as ViewMode[]).map((mode) => (
              <button
                key={mode}
                onClick={() => setViewMode(mode)}
                className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
                  viewMode === mode
                    ? "bg-(--bg-surface) text-(--text-primary) shadow-sm"
                    : "text-(--text-muted) hover:text-(--text-secondary)"
                }`}
              >
                {mode === "grouped" ? "Grouped" : "List"}
              </button>
            ))}
          </div>
          {canCreate && (
            <button
              onClick={handleHeaderNewTask}
              className="flex items-center gap-2 px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-(--accent) hover:bg-(--accent-hover) transition-colors focus:outline-none focus:ring-2 focus:ring-(--accent) focus:ring-offset-2 focus:ring-offset-(--bg-base)"
            >
              <Plus size={15} />
              New task
            </button>
          )}
        </div>
      </div>

      {/* Fetch error */}
      {tasksQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-(--destructive) bg-(--destructive)/5 border border-(--destructive)/20">
          Failed to load tasks. Please refresh.
        </div>
      )}

      {/* Filter tabs — List view only; Grouped view is already segmented by status */}
      {viewMode === "flat" && (
        <div className="flex items-center gap-0.5 mb-4 border-b border-(--border)">
          {FILTER_TABS.map((tab) => {
            const count =
              tab.key === "all"
                ? tasks.length
                : tasks.filter((t) => t.status === tab.key).length;
            const active = activeFilter === tab.key;
            return (
              <button
                key={tab.key}
                onClick={() => setActiveFilter(tab.key)}
                className={`flex items-center gap-2 px-3 py-2 text-sm font-medium -mb-px border-b-2 transition-colors ${
                  active
                    ? "text-(--accent) border-(--accent)"
                    : "text-(--text-muted) border-transparent hover:text-(--text-secondary)"
                }`}
              >
                {tab.label}
                {count > 0 && (
                  <span
                    className={`text-xs px-1.5 py-0.5 rounded-md tabular-nums ${
                      active
                        ? "bg-indigo-50 text-(--accent) dark:bg-indigo-500/10"
                        : "bg-(--bg-elevated) text-(--text-muted)"
                    }`}
                  >
                    {count}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      )}

      {/* Loading skeleton */}
      {tasksQuery.isPending ? (
        <div className="space-y-1">
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center gap-2.5 pl-1.5 pr-1.5 py-1.25 animate-pulse"
            >
              <div className="w-3.75 h-3.75 rounded-sm bg-(--bg-elevated)" />
              <div className="h-3 rounded bg-(--bg-elevated) flex-1 max-w-xs" />
              <div className="h-3 rounded bg-(--bg-elevated) w-14" />
            </div>
          ))}
        </div>
      ) : viewMode === "grouped" ? (
        /* ── Grouped view ─────────────────────────────── */
        <div>
          {GROUP_ORDER.map((status) => (
            <TaskGroupSection
              key={status}
              status={status}
              style={STATUS_STYLE[status]}
              tasks={groupedTasks[status]}
              collapsed={!!collapsedGroups[status]}
              onToggleCollapse={() => toggleGroupCollapse(status)}
              canCreate={canCreate}
              canUpdate={canUpdate}
              canDelete={canDelete}
              onUpdate={handleInlineUpdate}
              onDelete={handleDelete}
              onOpenDrawer={openEdit}
              isAdding={addingGroup === status}
              onStartAdding={() => setAddingGroup(status)}
              onCancelAdding={() => setAddingGroup(null)}
              onCreate={(title) => handleQuickCreate(title, status)}
            />
          ))}
        </div>
      ) : (
        /* ── List (flat) view ──────────────────────────── */
        <div>
          {filtered.length === 0 && !addingFlat ? (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <div className="w-11 h-11 rounded-lg bg-(--bg-elevated) flex items-center justify-center mb-4">
                <CheckCircle2 size={19} className="text-(--text-muted)" />
              </div>
              <p className="text-sm font-medium text-(--text-secondary) mb-1">
                {activeFilter === "all"
                  ? "No tasks yet"
                  : `No ${STATUS_STYLE[activeFilter as TaskStatus]?.label.toLowerCase()} tasks`}
              </p>
              <p className="text-xs text-(--text-muted) mb-4">
                {canCreate && activeFilter === "all"
                  ? "Create your first task to get started."
                  : "Nothing here for this filter."}
              </p>
              {canCreate && activeFilter === "all" && (
                <button
                  onClick={() => setAddingFlat(true)}
                  className="flex items-center gap-2 px-3.5 py-2 rounded-lg text-sm font-medium text-white bg-(--accent) hover:bg-(--accent-hover) transition-colors"
                >
                  <Plus size={14} />
                  New task
                </button>
              )}
            </div>
          ) : (
            filtered.map((task) => (
              <TaskRow
                key={task.id}
                task={task}
                canUpdate={canUpdate}
                canDelete={canDelete}
                showStatus
                onUpdate={handleInlineUpdate}
                onDelete={handleDelete}
                onOpenDrawer={openEdit}
              />
            ))
          )}
          {canCreate && (
            <TaskQuickAddRow
              isAdding={addingFlat}
              onStartAdding={() => setAddingFlat(true)}
              onCancel={() => setAddingFlat(false)}
              onCreate={(title) => handleQuickCreate(title, "todo")}
            />
          )}
        </div>
      )}
    </div>
  );
}
