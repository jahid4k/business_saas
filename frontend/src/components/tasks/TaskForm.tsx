// src/components/tasks/TaskForm.tsx
// Pure form — no GSAP, no overlay, no header.
// Drawer shell is handled by Drawer.tsx.
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { Task, TaskStatus } from "@/types/task";
import { useDrawer } from "@/contexts/DrawerContext";

const schema = z.object({
  title: z.string().min(1, "Title is required").max(255),
  description: z.string().optional(),
  status: z.enum(["todo", "in_progress", "done", "cancelled"]),
  dueDate: z.string().optional(),
});
export type TaskFormValues = z.infer<typeof schema>;

const STATUS_OPTIONS: { value: TaskStatus; label: string }[] = [
  { value: "todo", label: "Todo" },
  { value: "in_progress", label: "In Progress" },
  { value: "done", label: "Done" },
  { value: "cancelled", label: "Cancelled" },
];

interface TaskFormProps {
  task?: Task | null; // undefined/null = create, Task = edit
  onSave: (values: TaskFormValues) => Promise<void>;
}

export default function TaskForm({ task, onSave }: TaskFormProps) {
  const { closeDrawer } = useDrawer();
  const [saveError, setSaveError] = useState<string | null>(null);
  const isEdit = !!task;

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<TaskFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: task?.title ?? "",
      description: task?.description ?? "",
      status: task?.status ?? "todo",
      dueDate: task?.dueDate ? task.dueDate.split("T")[0] : "",
    },
  });

  const onSubmit = async (values: TaskFormValues) => {
    setSaveError(null);
    try {
      await onSave(values);
      closeDrawer(); // form closes itself after successful save
    } catch {
      setSaveError("Failed to save. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Scrollable fields */}
      <form
        id="task-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-5"
      >
        {saveError && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {saveError}
          </div>
        )}

        {/* Title */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            type="text"
            placeholder="What needs to be done?"
            autoFocus
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        {/* Description */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Description
            <span className="ml-2 text-xs font-normal text-[var(--text-muted)]">
              optional
            </span>
          </label>
          <textarea
            {...register("description")}
            rows={4}
            placeholder="Add more details…"
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all resize-none"
          />
        </div>

        {/* Status */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Status
          </label>
          <select
            {...register("status")}
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
          >
            {STATUS_OPTIONS.map((o) => (
              <option
                key={o.value}
                value={o.value}
                style={{
                  background: "var(--bg-elevated)",
                  color: "var(--text-primary)",
                }}
              >
                {o.label}
              </option>
            ))}
          </select>
        </div>

        {/* Due date */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Due date
            <span className="ml-2 text-xs font-normal text-[var(--text-muted)]">
              optional
            </span>
          </label>
          <input
            {...register("dueDate")}
            type="date"
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
          />
        </div>
      </form>

      {/* Footer — always pinned to bottom */}
      <div className="flex items-center gap-3 px-6 py-4 border-t border-[var(--border)] flex-shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-elevated)] transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          form="task-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create task"}
        </button>
      </div>
    </div>
  );
}
