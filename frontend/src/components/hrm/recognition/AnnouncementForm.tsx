// src/components/hrm/recognition/AnnouncementForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type {
  Employee,
  Department,
  CreateAnnouncementPayload,
} from "@/types/hrm";

const CATEGORIES = [
  { value: "general", label: "General" },
  { value: "policy", label: "Policy" },
  { value: "event", label: "Event" },
  { value: "award", label: "Award" },
  { value: "reminder", label: "Reminder" },
  { value: "emergency", label: "Emergency" },
  { value: "hr_update", label: "HR update" },
];

const schema = z.object({
  title: z.string().min(1, "Title is required"),
  content: z.string().min(1, "Content is required"),
  category: z.string().min(1, "Category is required"),
  scope_type: z.string().min(1, "Scope is required"),
  scope_ids: z.array(z.string()).optional(),
  scheduled_at: z.string().optional(),
  expires_at: z.string().optional(),
  requires_acknowledgement: z.boolean().optional(),
  is_pinned: z.boolean().optional(),
});
type AnnouncementFormValues = z.infer<typeof schema>;

interface AnnouncementFormProps {
  employees: Employee[];
  departments: Department[];
  onSave: (payload: CreateAnnouncementPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function AnnouncementForm({
  employees,
  departments,
  onSave,
}: AnnouncementFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<AnnouncementFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: "",
      content: "",
      category: "general",
      scope_type: "organization",
      scope_ids: [],
      scheduled_at: "",
      expires_at: "",
      requires_acknowledgement: false,
      is_pinned: false,
    },
  });

  const scopeType = watch("scope_type");

  const onSubmit = async (values: AnnouncementFormValues) => {
    setError(null);
    try {
      await onSave({
        title: values.title,
        content: values.content,
        category: values.category as CreateAnnouncementPayload["category"],
        scope_type:
          values.scope_type as CreateAnnouncementPayload["scope_type"],
        scope_ids:
          values.scope_type === "organization" ? undefined : values.scope_ids,
        scheduled_at: values.scheduled_at || undefined,
        expires_at: values.expires_at || undefined,
        requires_acknowledgement: values.requires_acknowledgement,
        is_pinned: values.is_pinned,
      });
      closeDrawer();
    } catch {
      setError("Failed to create announcement. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="announcement-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            autoFocus
            placeholder="Announcement title"
            className={inputCls}
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Content <span className="text-red-400">*</span>
          </label>
          <textarea
            {...register("content")}
            rows={4}
            placeholder="Announcement body"
            className={inputCls}
          />
          {errors.content && (
            <p className="text-xs text-red-400">{errors.content.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Category
          </label>
          <select {...register("category")} className={inputCls}>
            {CATEGORIES.map((c) => (
              <option
                key={c.value}
                value={c.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {c.label}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Audience
          </label>
          <select {...register("scope_type")} className={inputCls}>
            <option
              value="organization"
              style={{ background: "var(--bg-elevated)" }}
            >
              Everyone
            </option>
            <option
              value="department"
              style={{ background: "var(--bg-elevated)" }}
            >
              Specific departments
            </option>
            <option
              value="individual"
              style={{ background: "var(--bg-elevated)" }}
            >
              Specific employees
            </option>
          </select>
        </div>

        {scopeType === "department" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Departments
            </label>
            <select
              multiple
              {...register("scope_ids")}
              className={`${inputCls} h-28`}
            >
              {departments.map((d) => (
                <option
                  key={d.id}
                  value={d.id}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {d.name}
                </option>
              ))}
            </select>
            <p className="text-xs text-[var(--text-muted)]">
              Hold Ctrl/Cmd to select multiple.
            </p>
          </div>
        )}

        {scopeType === "individual" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Employees
            </label>
            <select
              multiple
              {...register("scope_ids")}
              className={`${inputCls} h-28`}
            >
              {employees.map((e) => (
                <option
                  key={e.id}
                  value={e.id}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {e.first_name} {e.last_name ?? ""}
                </option>
              ))}
            </select>
            <p className="text-xs text-[var(--text-muted)]">
              Hold Ctrl/Cmd to select multiple.
            </p>
          </div>
        )}

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Schedule for (optional)
            </label>
            <input
              {...register("scheduled_at")}
              type="datetime-local"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Expires on
            </label>
            <input
              {...register("expires_at")}
              type="datetime-local"
              className={inputCls}
            />
          </div>
        </div>

        <div className="flex items-center gap-6 pt-1">
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="requires_acknowledgement"
              {...register("requires_acknowledgement")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="requires_acknowledgement"
              className="text-sm text-[var(--text-secondary)]"
            >
              Requires acknowledgement
            </label>
          </div>
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="is_pinned"
              {...register("is_pinned")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="is_pinned"
              className="text-sm text-[var(--text-secondary)]"
            >
              Pin to top
            </label>
          </div>
        </div>
      </form>

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
          form="announcement-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create announcement"}
        </button>
      </div>
    </div>
  );
}
