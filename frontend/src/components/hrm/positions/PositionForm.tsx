// src/components/hrm/positions/PositionForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import { listDepartments } from "@/lib/hrm/departments";
import type { Position, Department } from "@/types/hrm";

const schema = z.object({
  title: z.string().min(1, "Title is required").max(150, "Max 150 characters"),
  description: z.string().optional(),
  department_id: z.string().optional(),
  is_active: z.boolean().optional(),
});
type PositionFormValues = z.infer<typeof schema>;

interface PositionFormProps {
  orgId: string;
  position?: Position | null;
  onSave: (values: PositionFormValues) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function PositionForm({
  orgId,
  position,
  onSave,
}: PositionFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const [departments, setDepartments] = useState<Department[]>([]);
  const isEdit = !!position;

  useEffect(() => {
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
  }, [orgId]);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<PositionFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: position?.title ?? "",
      description: position?.description ?? "",
      department_id: position?.department_id ?? "",
      is_active: position?.is_active ?? true,
    },
  });

  const onSubmit = async (values: PositionFormValues) => {
    setError(null);
    const payload = Object.fromEntries(
      Object.entries(values).map(([k, v]) => [k, v === "" ? undefined : v]),
    ) as PositionFormValues;

    try {
      await onSave(payload);
      closeDrawer();
    } catch {
      setError("Failed to save position. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="position-form"
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
          <label className="block text-sm font-medium text-(--text-secondary)">
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            autoFocus
            placeholder="Senior Software Engineer"
            className={inputCls}
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Description
          </label>
          <textarea
            {...register("description")}
            rows={3}
            placeholder="Role summary"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Department
          </label>
          <select {...register("department_id")} className={inputCls}>
            <option value="">Unassigned</option>
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
        </div>

        {isEdit && (
          <div className="flex items-center gap-2.5 pt-1">
            <input
              type="checkbox"
              id="is_active"
              {...register("is_active")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="is_active"
              className="text-sm text-(--text-secondary)"
            >
              Active
            </label>
          </div>
        )}
      </form>

      <div className="flex items-center gap-3 px-6 py-4 border-t border-(--border) shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-(--text-secondary) border border-(--border) hover:bg-(--bg-elevated) transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          form="position-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create position"}
        </button>
      </div>
    </div>
  );
}
