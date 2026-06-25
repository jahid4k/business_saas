// src/app/(dashboard)/[orgId]/tasks/page.tsx
"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import {
  Plus,
  MoreHorizontal,
  Pencil,
  Trash2,
  CalendarDays,
  CheckCircle2,
  Loader2,
} from "lucide-react";
import gsap from "gsap";
import type { Task, TaskStatus } from "@/types/task";
import { listTasks, createTask, updateTask, deleteTask } from "@/lib/tasks";
import { usePermissionStore } from "@/stores/permissionStore";

import { useDrawer } from "@/contexts/DrawerContext";
import TaskForm, { type TaskFormValues } from "@/components/tasks/TaskForm";

// ── Status config ─────────────────────────────────────────
type FilterKey = "all" | TaskStatus;

const FILTER_TABS: { key: FilterKey; label: string }[] = [
  { key: "all", label: "All" },
  { key: "todo", label: "Todo" },
  { key: "in_progress", label: "In Progress" },
  { key: "done", label: "Done" },
  { key: "cancelled", label: "Cancelled" },
];

const STATUS_STYLE: Record<
  TaskStatus,
  { label: string; dot: string; badge: string }
> = {
  todo: {
    label: "Todo",
    dot: "bg-zinc-500",
    badge: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  },
  in_progress: {
    label: "In Progress",
    dot: "bg-blue-400",
    badge: "bg-blue-500/10  text-blue-400  border-blue-500/20",
  },
  done: {
    label: "Done",
    dot: "bg-emerald-400",
    badge: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  },
  cancelled: {
    label: "Cancelled",
    dot: "bg-red-400",
    badge: "bg-red-500/10   text-red-400   border-red-500/20",
  },
};

// ── Helpers ───────────────────────────────────────────────
function formatDate(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function dueDateColor(iso?: string, status?: TaskStatus) {
  if (!iso || status === "done" || status === "cancelled")
    return "text-[var(--text-muted)]";
  const daysLeft = (new Date(iso).getTime() - Date.now()) / 86_400_000;
  if (daysLeft < 0) return "text-red-400";
  if (daysLeft < 3) return "text-amber-400";
  return "text-[var(--text-muted)]";
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

  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [pageError, setPageError] = useState<string | null>(null);
  const [activeFilter, setActiveFilter] = useState<FilterKey>("all");
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  // ── Permissions ───────────────────────────────────────
  const canCreate = hasPermission("tasks.create");
  const canUpdate = hasPermission("tasks.update");
  const canDelete = hasPermission("tasks.delete");

  // ── Fetch ─────────────────────────────────────────────
  const fetchTasks = useCallback(async () => {
    setLoading(true);
    setPageError(null);
    try {
      const data = await listTasks(orgId);
      setTasks(data.tasks);
    } catch {
      setPageError("Failed to load tasks. Please refresh.");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  // ── GSAP: animate rows on load / filter change ────────
  useEffect(() => {
    if (loading || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".task-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [loading, activeFilter]);

  // ── Close action menu on outside click ────────────────
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      let inside = false;
      menuRefs.current.forEach((el) => {
        if (el.contains(e.target as Node)) inside = true;
      });
      if (!inside) setOpenMenuId(null);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  // ── Filtered list ─────────────────────────────────────
  const filtered =
    activeFilter === "all"
      ? tasks
      : tasks.filter((t) => t.status === activeFilter);

  // ── Handlers ──────────────────────────────────────────
  const openCreate = () => {
    openDrawer({
      title: "New task",
      content: (
        <TaskForm
          onSave={async (values) => {
            const body = {
              title: values.title,
              description: values.description || undefined,
              status: values.status,
              dueDate: values.dueDate
                ? `${values.dueDate}T00:00:00.000Z`
                : undefined,
            };
            const created = await createTask(orgId, body);
            setTasks((prev) => [created, ...prev]);
          }}
        />
      ),
    });
  };
  const openEdit = (task: Task) => {
    setOpenMenuId(null);
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
            setTasks((prev) =>
              prev.map((t) => (t.id === updated.id ? updated : t)),
            );
          }}
        />
      ),
    });
  };

  const handleSave = async (values: TaskFormValues) => {
    const body = {
      title: values.title,
      description: values.description || undefined,
      status: values.status,
      // Convert YYYY-MM-DD → ISO 8601; undefined if empty
      dueDate: values.dueDate ? `${values.dueDate}T00:00:00.000Z` : undefined,
    };

    if (editingTask) {
      const updated = await updateTask(orgId, editingTask.id, body);
      setTasks((prev) => prev.map((t) => (t.id === updated.id ? updated : t)));
    } else {
      const created = await createTask(orgId, body);
      setTasks((prev) => [created, ...prev]);
    }
    setDrawerOpen(false);
  };

  const handleDelete = async (taskId: string) => {
    try {
      await deleteTask(orgId, taskId);
      setTasks((prev) => prev.filter((t) => t.id !== taskId));
    } catch {
      setPageError("Failed to delete task.");
    }
    setDeleteConfirm(null);
    setOpenMenuId(null);
  };

  // ── Render ────────────────────────────────────────────
  return (
    <>
      <div className="p-6 md:p-8 max-w-4xl">
        {/* Page header */}
        <div className="flex items-start justify-between mb-8">
          <div>
            <h1
              className="text-2xl font-bold text-[var(--text-primary)] mb-1"
              style={{
                fontFamily: "var(--font-syne, Syne, sans-serif)",
                letterSpacing: "-0.02em",
              }}
            >
              Tasks
            </h1>
            <p className="text-sm text-[var(--text-muted)]">
              {tasks.length} {tasks.length === 1 ? "task" : "tasks"} total
            </p>
          </div>
          {canCreate && (
            <button
              onClick={openCreate}
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={15} />
              New task
            </button>
          )}
        </div>

        {/* Page-level error */}
        {pageError && (
          <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
            {pageError}
          </div>
        )}

        {/* Filter tabs */}
        <div className="flex items-center gap-0.5 mb-6 border-b border-[var(--border)]">
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
                className={`
                  flex items-center gap-2 px-3.5 py-2.5 text-sm font-medium -mb-px border-b-2 transition-colors
                  ${
                    active
                      ? "text-purple-400 border-purple-500"
                      : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
                  }
                `}
              >
                {tab.label}
                {count > 0 && (
                  <span
                    className={`
                    text-xs px-1.5 py-0.5 rounded-full min-w-[20px] text-center
                    ${
                      active
                        ? "bg-purple-500/15 text-purple-400"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"
                    }
                  `}
                  >
                    {count}
                  </span>
                )}
              </button>
            );
          })}
        </div>

        {/* Loading */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <div className="flex items-center gap-3 text-sm text-[var(--text-muted)]">
              <Loader2 size={16} className="animate-spin text-purple-500" />
              Loading tasks…
            </div>
          </div>
        ) : /* Empty state */
        filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
              <CheckCircle2 size={20} className="text-[var(--text-muted)]" />
            </div>
            <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
              {activeFilter === "all"
                ? "No tasks yet"
                : `No ${STATUS_STYLE[activeFilter as TaskStatus]?.label.toLowerCase()} tasks`}
            </p>
            <p className="text-xs text-[var(--text-muted)] mb-4">
              {canCreate && activeFilter === "all"
                ? "Create your first task to get started."
                : "Nothing here for this filter."}
            </p>
            {canCreate && activeFilter === "all" && (
              <button
                onClick={openCreate}
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
              >
                <Plus size={14} />
                New task
              </button>
            )}
          </div>
        ) : (
          /* Task list */
          <div ref={listRef} className="space-y-1.5">
            {filtered.map((task) => {
              const s = STATUS_STYLE[task.status];
              const confirming = deleteConfirm === task.id;
              const menuOpen = openMenuId === task.id;

              return (
                <div
                  key={task.id}
                  className="task-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
                >
                  {/* Status dot */}
                  <div
                    className={`w-2.5 h-2.5 rounded-full flex-shrink-0 mt-1.5 ${s.dot}`}
                  />

                  {/* Main content */}
                  <div className="flex-1 min-w-0">
                    <p
                      className={`text-sm font-medium leading-snug ${
                        task.status === "done" || task.status === "cancelled"
                          ? "line-through text-[var(--text-muted)]"
                          : "text-[var(--text-primary)]"
                      }`}
                    >
                      {task.title}
                    </p>
                    {task.description && (
                      <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                        {task.description}
                      </p>
                    )}
                    <div className="flex items-center gap-3 mt-2 flex-wrap">
                      <span
                        className={`text-xs px-2 py-0.5 rounded-full border font-medium ${s.badge}`}
                      >
                        {s.label}
                      </span>
                      {task.dueDate && (
                        <span
                          className={`flex items-center gap-1.5 text-xs ${dueDateColor(task.dueDate, task.status)}`}
                        >
                          <CalendarDays size={11} />
                          {formatDate(task.dueDate)}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Right: delete confirm OR action menu */}
                  {confirming ? (
                    <div className="flex items-center gap-2 flex-shrink-0 pt-0.5">
                      <span className="text-xs text-[var(--text-muted)]">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(task.id)}
                        className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(null)}
                        className="px-2.5 py-1 rounded-md text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    (canUpdate || canDelete) && (
                      <div
                        className="relative flex-shrink-0"
                        ref={(el) => {
                          if (el) menuRefs.current.set(task.id, el);
                          else menuRefs.current.delete(task.id);
                        }}
                      >
                        {/* 3-dot button — visible on hover */}
                        <button
                          onClick={() =>
                            setOpenMenuId(menuOpen ? null : task.id)
                          }
                          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                        >
                          <MoreHorizontal size={15} />
                        </button>

                        {/* Dropdown menu */}
                        {menuOpen && (
                          <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                            {canUpdate && (
                              <button
                                onClick={() => openEdit(task)}
                                className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors text-left"
                              >
                                <Pencil size={13} />
                                Edit
                              </button>
                            )}
                            {canDelete && (
                              <button
                                onClick={() => {
                                  setDeleteConfirm(task.id);
                                  setOpenMenuId(null);
                                }}
                                className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 transition-colors text-left"
                              >
                                <Trash2 size={13} />
                                Delete
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                    )
                  )}
                </div>
              );
            })}
          </div>
        )}

        {/* Footer count */}
        {!loading && filtered.length > 0 && (
          <p className="mt-5 text-xs text-[var(--text-muted)]">
            Showing {filtered.length} of {tasks.length}{" "}
            {tasks.length === 1 ? "task" : "tasks"}
          </p>
        )}
      </div>
    </>
  );
}
