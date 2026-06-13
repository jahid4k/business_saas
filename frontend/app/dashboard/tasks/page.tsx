"use client";
// frontend/app/dashboard/tasks/page.tsx
// Full task CRUD. UI buttons hidden by permission. Backend enforces too.

import { useState, useEffect, FormEvent } from "react";
import { useAuth } from "@/hooks/useAuth";
import * as api from "@/lib/api";
import type { Task } from "@/types";

const STATUS_OPTIONS = ["todo", "in_progress", "done"] as const;

const statusLabel: Record<string, string> = {
  todo: "To Do",
  in_progress: "In Progress",
  done: "Done",
};

const statusBadge: Record<string, string> = {
  todo: "bg-gray-800 text-gray-400",
  in_progress: "bg-blue-900 text-blue-300",
  done: "bg-green-900 text-green-300",
};

export default function TasksPage() {
  const { currentBusiness, hasPermission } = useAuth();

  const businessId = currentBusiness?.id ?? "";

  const [tasks, setTasks] = useState<Task[]>([]);
  const [loadedBusinessId, setLoadedBusinessId] = useState("");
  const [error, setError] = useState("");

  const canRead = hasPermission("tasks.read");
  const canCreate = hasPermission("tasks.create");
  const canUpdate = hasPermission("tasks.update");
  const canDelete = hasPermission("tasks.delete");

  const loading = Boolean(
    businessId && canRead && loadedBusinessId !== businessId,
  );

  // Create form
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({
    title: "",
    description: "",
    status: "todo",
  });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState("");

  // Edit state
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState({
    title: "",
    description: "",
    status: "todo",
  });
  const [saving, setSaving] = useState(false);

  // Delete state
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    if (!businessId || !canRead) return;

    let cancelled = false;

    async function loadInitialTasks() {
      try {
        const res = await api.listTasks();

        if (cancelled) return;

        if (res.success && res.data) {
          setTasks(Array.isArray(res.data) ? res.data : []);
          setError("");
        } else {
          setTasks([]);
          setError(res.error?.message || "Failed to load tasks");
        }
      } catch (err) {
        if (cancelled) return;

        setTasks([]);
        setError(err instanceof Error ? err.message : "Failed to load tasks");
      } finally {
        if (!cancelled) {
          setLoadedBusinessId(businessId);
        }
      }
    }

    void loadInitialTasks();

    return () => {
      cancelled = true;
    };
  }, [businessId, canRead]);

  async function reloadTasks() {
    if (!businessId || !canRead) return;

    setLoadedBusinessId("");
    setError("");

    try {
      const res = await api.listTasks();

      if (res.success && res.data) {
        setTasks(Array.isArray(res.data) ? res.data : []);
        setError("");
      } else {
        setTasks([]);
        setError(res.error?.message || "Failed to load tasks");
      }
    } catch (err) {
      setTasks([]);
      setError(err instanceof Error ? err.message : "Failed to load tasks");
    } finally {
      setLoadedBusinessId(businessId);
    }
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();

    setCreateError("");
    setCreating(true);

    try {
      const res = await api.createTask(createForm);

      if (!res.success) {
        setCreateError(res.error?.message || "Failed to create task");
        return;
      }

      setCreateForm({ title: "", description: "", status: "todo" });
      setShowCreate(false);

      await reloadTasks();
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : "Failed to create task",
      );
    } finally {
      setCreating(false);
    }
  }

  function startEdit(task: Task) {
    setEditingId(task.id);
    setEditForm({
      title: task.title,
      description: task.description || "",
      status: task.status,
    });
  }

  async function handleUpdate(id: string) {
    setSaving(true);
    setError("");

    try {
      const res = await api.updateTask(id, editForm);

      if (!res.success) {
        setError(res.error?.message || "Failed to update task");
        return;
      }

      setEditingId(null);

      await reloadTasks();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update task");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string) {
    if (!confirm("Delete this task?")) return;

    setDeletingId(id);
    setError("");

    try {
      const res = await api.deleteTask(id);

      if (!res.success) {
        setError(res.error?.message || "Failed to delete task");
        return;
      }

      await reloadTasks();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete task");
    } finally {
      setDeletingId(null);
    }
  }

  if (!currentBusiness) {
    return (
      <div className="p-8 max-w-4xl">
        <h2 className="text-xl font-semibold text-white mb-1">Tasks</h2>

        <div className="mt-6 bg-gray-900 border border-gray-800 rounded-xl p-6">
          <p className="text-gray-400 text-sm">
            No workspace selected. Go to Businesses first.
          </p>
        </div>
      </div>
    );
  }

  if (!canRead) {
    return (
      <div className="p-8 max-w-4xl">
        <h2 className="text-xl font-semibold text-white mb-1">Tasks</h2>

        <div className="mt-6 bg-yellow-950 border border-yellow-800 rounded-xl p-6">
          <p className="text-yellow-300 text-sm">
            You don&apos;t have{" "}
            <code className="font-mono text-xs">tasks.read</code> permission in
            this workspace.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-4xl">
      <div className="flex items-center justify-between mb-1">
        <h2 className="text-xl font-semibold text-white">Tasks</h2>

        {canCreate && (
          <button
            onClick={() => setShowCreate((v) => !v)}
            className="bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium rounded-lg px-4 py-2 transition-colors"
          >
            {showCreate ? "Cancel" : "+ New task"}
          </button>
        )}
      </div>

      <p className="text-gray-400 text-sm mb-2">
        Workspace: <span className="text-gray-300">{currentBusiness.name}</span>
      </p>

      {/* Permission indicators */}
      <div className="flex gap-2 mb-8 flex-wrap">
        {(
          [
            "tasks.read",
            "tasks.create",
            "tasks.update",
            "tasks.delete",
          ] as const
        ).map((p) => {
          const allowed = hasPermission(p);

          return (
            <span
              key={p}
              className={`text-xs font-mono px-2.5 py-1 rounded-md ${
                allowed
                  ? "bg-green-950 text-green-400"
                  : "bg-gray-900 text-gray-600 line-through"
              }`}
            >
              {p}
            </span>
          );
        })}
      </div>

      {/* Create form */}
      {showCreate && canCreate && (
        <section className="mb-6">
          <div className="bg-gray-900 border border-indigo-700 rounded-xl p-5">
            <h3 className="text-sm font-medium text-white mb-4">New Task</h3>

            <form onSubmit={handleCreate} className="space-y-3">
              {createError && (
                <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-lg px-4 py-3">
                  {createError}
                </div>
              )}

              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Title *
                </label>

                <input
                  type="text"
                  value={createForm.title}
                  onChange={(e) =>
                    setCreateForm((f) => ({ ...f, title: e.target.value }))
                  }
                  required
                  autoFocus
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  placeholder="Task title"
                />
              </div>

              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Description
                </label>

                <textarea
                  value={createForm.description}
                  onChange={(e) =>
                    setCreateForm((f) => ({
                      ...f,
                      description: e.target.value,
                    }))
                  }
                  rows={2}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 resize-none"
                  placeholder="Optional description"
                />
              </div>

              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Status
                </label>

                <select
                  value={createForm.status}
                  onChange={(e) =>
                    setCreateForm((f) => ({ ...f, status: e.target.value }))
                  }
                  className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm focus:outline-none focus:border-indigo-500"
                >
                  {STATUS_OPTIONS.map((s) => (
                    <option key={s} value={s}>
                      {statusLabel[s]}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex gap-3 pt-1">
                <button
                  type="submit"
                  disabled={creating}
                  className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg px-5 py-2 transition-colors"
                >
                  {creating ? "Creating..." : "Create task"}
                </button>

                <button
                  type="button"
                  onClick={() => setShowCreate(false)}
                  className="text-gray-400 hover:text-white text-sm px-3 py-2 transition-colors"
                >
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </section>
      )}

      {/* Error */}
      {error && (
        <div className="mb-4 bg-red-950 border border-red-800 text-red-300 text-sm rounded-xl px-4 py-3">
          {error}
        </div>
      )}

      {/* Task list */}
      {loading ? (
        <p className="text-gray-500 text-sm">Loading tasks...</p>
      ) : tasks.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
          <p className="text-gray-400 text-sm">
            No tasks yet.{canCreate ? " Create one above." : ""}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {tasks.map((task) => (
            <div
              key={task.id}
              className="bg-gray-900 border border-gray-800 rounded-xl p-5"
            >
              {editingId === task.id ? (
                <div className="space-y-3">
                  <input
                    type="text"
                    value={editForm.title}
                    onChange={(e) =>
                      setEditForm((f) => ({ ...f, title: e.target.value }))
                    }
                    autoFocus
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  />

                  <textarea
                    value={editForm.description}
                    onChange={(e) =>
                      setEditForm((f) => ({
                        ...f,
                        description: e.target.value,
                      }))
                    }
                    rows={2}
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-indigo-500 resize-none"
                  />

                  <select
                    value={editForm.status}
                    onChange={(e) =>
                      setEditForm((f) => ({ ...f, status: e.target.value }))
                    }
                    className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-indigo-500"
                  >
                    {STATUS_OPTIONS.map((s) => (
                      <option key={s} value={s}>
                        {statusLabel[s]}
                      </option>
                    ))}
                  </select>

                  <div className="flex gap-3">
                    <button
                      onClick={() => handleUpdate(task.id)}
                      disabled={saving}
                      className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-medium rounded-lg px-4 py-1.5 transition-colors"
                    >
                      {saving ? "Saving..." : "Save"}
                    </button>

                    <button
                      onClick={() => setEditingId(null)}
                      className="text-gray-400 hover:text-white text-xs px-3 py-1.5 transition-colors"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : (
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <p className="text-white text-sm font-medium">
                        {task.title}
                      </p>

                      <span
                        className={`text-xs px-2 py-0.5 rounded-md ${
                          statusBadge[task.status] ??
                          "bg-gray-800 text-gray-400"
                        }`}
                      >
                        {statusLabel[task.status] ?? task.status}
                      </span>
                    </div>

                    {task.description && (
                      <p className="text-gray-400 text-xs mb-1">
                        {task.description}
                      </p>
                    )}

                    <p className="text-gray-600 text-xs font-mono">{task.id}</p>
                  </div>

                  <div className="flex gap-2 shrink-0">
                    {canUpdate && (
                      <button
                        onClick={() => startEdit(task)}
                        className="text-gray-400 hover:text-white text-xs bg-gray-800 hover:bg-gray-700 rounded-lg px-3 py-1.5 transition-colors"
                      >
                        Edit
                      </button>
                    )}

                    {canDelete && (
                      <button
                        onClick={() => handleDelete(task.id)}
                        disabled={deletingId === task.id}
                        className="text-red-400 hover:text-red-300 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-50 rounded-lg px-3 py-1.5 transition-colors"
                      >
                        {deletingId === task.id ? "..." : "Delete"}
                      </button>
                    )}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
