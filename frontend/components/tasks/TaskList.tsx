"use client";

import { useState, useEffect } from "react";
import { api, extractApiError } from "@/lib/api";
import { usePermission } from "@/hooks/usePermission";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import type {
  Task,
  CreateTaskRequest,
  UpdateTaskRequest,
  TaskStatus,
} from "@/types/task";
import type { ApiSuccess } from "@/types/api";
import clsx from "clsx";

const STATUS_BADGE: Record<TaskStatus, "neutral" | "info" | "success"> = {
  todo: "neutral",
  in_progress: "info",
  done: "success",
};

const STATUS_LABELS: Record<TaskStatus, string> = {
  todo: "To do",
  in_progress: "In progress",
  done: "Done",
};

export function TaskList() {
  const { can } = usePermission();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function loadTasks() {
      try {
        const { data } =
          await api.get<ApiSuccess<{ tasks: Task[]; total: number }>>("/tasks");

        if (!cancelled) {
          setTasks(data.data?.tasks ?? []);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(extractApiError(err));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    void loadTasks();

    return () => {
      cancelled = true;
    };
  }, []);

  async function handleCreate(req: CreateTaskRequest) {
    const { data } = await api.post<ApiSuccess<Task>>("/tasks", req);
    if (data.data) {
      setTasks((t) => [data.data!, ...t]);
      setShowCreate(false);
    }
  }

  async function handleUpdate(id: string, req: UpdateTaskRequest) {
    const { data } = await api.patch<ApiSuccess<Task>>(`/tasks/${id}`, req);
    if (data.data) {
      setTasks((t) => t.map((task) => (task.id === id ? data.data! : task)));
      setEditingId(null);
    }
  }

  async function handleDelete(id: string) {
    await api.delete(`/tasks/${id}`);
    setTasks((t) => t.filter((task) => task.id !== id));
  }

  if (isLoading) return <TaskSkeleton />;

  return (
    <div className="space-y-3">
      {/* Toolbar */}
      <div className="flex items-center justify-between">
        <p className="text-xs font-mono text-ink-muted">
          {tasks.length} task{tasks.length !== 1 ? "s" : ""}
        </p>
        {can("tasks.create") && (
          <Button
            size="sm"
            onClick={() => setShowCreate(true)}
            disabled={showCreate}
          >
            + New task
          </Button>
        )}
      </div>

      {error && (
        <p className="text-xs text-status-error font-mono bg-status-error/5 border border-status-error/20 rounded px-3 py-2">
          {error}
        </p>
      )}

      {/* Create form */}
      {showCreate && can("tasks.create") && (
        <TaskForm
          onSubmit={handleCreate}
          onCancel={() => setShowCreate(false)}
          submitLabel="Create task"
        />
      )}

      {/* Task rows */}
      {tasks.length === 0 && !showCreate ? (
        <div className="text-center py-12 text-ink-muted font-mono text-sm">
          <p className="text-2xl mb-2">◈</p>
          <p>No tasks yet.</p>
          {can("tasks.create") && (
            <p className="mt-1 text-xs">
              Click &quot;+ New task&quot; to create one.
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-1.5">
          {tasks.map((task) =>
            editingId === task.id && can("tasks.update") ? (
              <TaskForm
                key={task.id}
                initial={task}
                onSubmit={(req) => handleUpdate(task.id, req)}
                onCancel={() => setEditingId(null)}
                submitLabel="Save"
              />
            ) : (
              <TaskRow
                key={task.id}
                task={task}
                canUpdate={can("tasks.update")}
                canDelete={can("tasks.delete")}
                onEdit={() => setEditingId(task.id)}
                onDelete={() => handleDelete(task.id)}
                onStatusChange={(status) => handleUpdate(task.id, { status })}
              />
            ),
          )}
        </div>
      )}
    </div>
  );
}

// ── Task row ──────────────────────────────────────────────────────

function TaskRow({
  task,
  canUpdate,
  canDelete,
  onEdit,
  onDelete,
  onStatusChange,
}: {
  task: Task;
  canUpdate: boolean;
  canDelete: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onStatusChange: (s: TaskStatus) => void;
}) {
  return (
    <div className="group flex items-center gap-3 px-4 py-3 bg-surface-raised border border-surface-border rounded hover:border-surface-muted transition-colors">
      {/* Status selector */}
      {canUpdate ? (
        <select
          value={task.status}
          onChange={(e) => onStatusChange(e.target.value as TaskStatus)}
          className="text-2xs font-mono bg-transparent border-0 text-ink-muted focus:outline-none cursor-pointer"
          title="Change status"
        >
          {(["todo", "in_progress", "done"] as TaskStatus[]).map((s) => (
            <option key={s} value={s}>
              {STATUS_LABELS[s]}
            </option>
          ))}
        </select>
      ) : (
        <Badge variant={STATUS_BADGE[task.status]}>
          {STATUS_LABELS[task.status]}
        </Badge>
      )}

      {/* Title + description */}
      <div className="flex-1 min-w-0">
        <p
          className={clsx(
            "text-sm font-body text-ink-primary truncate",
            task.status === "done" && "line-through text-ink-muted",
          )}
        >
          {task.title}
        </p>
        {task.description && (
          <p className="text-xs text-ink-muted truncate mt-0.5">
            {task.description}
          </p>
        )}
      </div>

      {/* Actions — visible on hover */}
      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        {canUpdate && (
          <Button size="sm" variant="ghost" onClick={onEdit}>
            edit
          </Button>
        )}
        {canDelete && (
          <Button size="sm" variant="danger" onClick={onDelete}>
            del
          </Button>
        )}
      </div>
    </div>
  );
}

// ── Task form ─────────────────────────────────────────────────────

function TaskForm({
  initial,
  onSubmit,
  onCancel,
  submitLabel,
}: {
  initial?: Task;
  onSubmit: (req: CreateTaskRequest) => Promise<void>;
  onCancel: () => void;
  submitLabel: string;
}) {
  const [title, setTitle] = useState(initial?.title ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [status, setStatus] = useState<TaskStatus>(initial?.status ?? "todo");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    setIsLoading(true);
    setError(null);
    try {
      await onSubmit({
        title: title.trim(),
        description: description.trim(),
        status,
      });
    } catch (err) {
      setError(extractApiError(err));
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="p-4 bg-surface-overlay border border-brand/20 rounded-md space-y-3"
    >
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="Task title"
        required
        className="w-full bg-transparent text-sm text-ink-primary placeholder:text-ink-disabled focus:outline-none font-body"
      />
      <input
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="Description (optional)"
        className="w-full bg-transparent text-xs text-ink-secondary placeholder:text-ink-disabled focus:outline-none font-body"
      />
      <div className="flex items-center justify-between">
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value as TaskStatus)}
          className="text-xs font-mono bg-surface-muted border border-surface-border rounded px-2 py-1 text-ink-secondary focus:outline-none"
        >
          {(["todo", "in_progress", "done"] as TaskStatus[]).map((s) => (
            <option key={s} value={s}>
              {STATUS_LABELS[s]}
            </option>
          ))}
        </select>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="ghost" type="button" onClick={onCancel}>
            cancel
          </Button>
          <Button size="sm" type="submit" isLoading={isLoading}>
            {submitLabel}
          </Button>
        </div>
      </div>
      {error && <p className="text-xs text-status-error font-mono">{error}</p>}
    </form>
  );
}

function TaskSkeleton() {
  return (
    <div className="space-y-2 animate-pulse">
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="h-12 bg-surface-raised rounded border border-surface-border"
        />
      ))}
    </div>
  );
}
