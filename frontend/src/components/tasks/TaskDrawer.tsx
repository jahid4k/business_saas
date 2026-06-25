// src/components/tasks/TaskDrawer.tsx
"use client";

import { useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { X } from "lucide-react";
import gsap from "gsap";
import type { Task, TaskStatus } from "@/types/task";

// ── Validation ────────────────────────────────────────────
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

// ── Props ─────────────────────────────────────────────────
interface TaskDrawerProps {
  open: boolean;
  task: Task | null; // null = create mode, Task = edit mode
  onClose: () => void;
  onSave: (values: TaskFormValues) => Promise<void>;
}

// ── Component ─────────────────────────────────────────────
export default function TaskDrawer({
  open,
  task,
  onClose,
  onSave,
}: TaskDrawerProps) {
  const overlayRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const initRef = useRef(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const isEdit = !!task;

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<TaskFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { title: "", description: "", status: "todo", dueDate: "" },
  });

  // Set initial off-screen state once on mount (no animation)
  useEffect(() => {
    if (!panelRef.current || !overlayRef.current) return;
    gsap.set(panelRef.current, { x: "100%" });
    gsap.set(overlayRef.current, { opacity: 0, pointerEvents: "none" });
    initRef.current = true;
  }, []);

  // Slide in / slide out when `open` changes
  useEffect(() => {
    if (!initRef.current) return;
    if (open) {
      gsap.to(overlayRef.current, {
        opacity: 1,
        pointerEvents: "auto",
        duration: 0.2,
      });
      gsap.to(panelRef.current, { x: "0%", duration: 0.3, ease: "power3.out" });
    } else {
      gsap.to(panelRef.current, {
        x: "100%",
        duration: 0.25,
        ease: "power2.in",
      });
      gsap.to(overlayRef.current, {
        opacity: 0,
        pointerEvents: "none",
        duration: 0.2,
      });
    }
  }, [open]);

  // Reset form values when switching between create / edit
  useEffect(() => {
    setSaveError(null);
    reset({
      title: task?.title ?? "",
      description: task?.description ?? "",
      status: task?.status ?? "todo",
      dueDate: task?.dueDate ? task.dueDate.split("T")[0] : "",
    });
  }, [task, reset]);

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    if (open) document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onClose]);

  const onSubmit = async (values: TaskFormValues) => {
    setSaveError(null);
    try {
      await onSave(values);
    } catch {
      setSaveError("Failed to save. Please try again.");
    }
  };

  return (
    // Always in DOM — GSAP controls visibility via translate + opacity
    <div className="fixed inset-0 z-50 pointer-events-none">
      {/* Dim overlay */}
      <div
        ref={overlayRef}
        className="absolute inset-0 bg-black/60"
        onClick={onClose}
      />

      {/* Slide panel */}
      <div
        ref={panelRef}
        className="absolute right-0 top-0 h-full w-full max-w-[440px] pointer-events-auto flex flex-col bg-[var(--bg-surface)] border-l border-[var(--border)] shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)] flex-shrink-0">
          <h2
            className="text-base font-semibold text-[var(--text-primary)]"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            {isEdit ? "Edit task" : "New task"}
          </h2>
          <button
            onClick={onClose}
            className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors"
          >
            <X size={16} />
          </button>
        </div>

        {/* Scrollable body */}
        <div className="flex-1 overflow-y-auto px-6 py-5">
          <form
            id="task-form"
            onSubmit={handleSubmit(onSubmit)}
            noValidate
            className="space-y-5"
          >
            {/* Save error */}
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
                autoComplete="off"
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
        </div>

        {/* Footer */}
        <div className="flex items-center gap-3 px-6 py-4 border-t border-[var(--border)] flex-shrink-0">
          <button
            type="button"
            onClick={onClose}
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
    </div>
  );
}
