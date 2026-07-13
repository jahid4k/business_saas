// src/components/hrm/leave/LeaveTypeForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { LeaveType } from "@/types/hrm";

const schema = z.object({
  name: z.string().min(1, "Name is required").max(100, "Max 100 characters"),
  description: z.string().optional(),
  max_days_per_year: z.coerce.number().min(0, "Must be 0 or more"),
  is_paid: z.boolean().optional(),
  requires_approval: z.boolean().optional(),
  is_active: z.boolean().optional(),
});
type LeaveTypeFormInput = z.input<typeof schema>;
type LeaveTypeFormValues = z.infer<typeof schema>;

interface LeaveTypeFormProps {
  leaveType?: LeaveType | null;
  onSave: (values: LeaveTypeFormValues) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function LeaveTypeForm({
  leaveType,
  onSave,
}: LeaveTypeFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!leaveType;

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LeaveTypeFormInput, undefined, LeaveTypeFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: leaveType?.name ?? "",
      description: leaveType?.description ?? "",
      max_days_per_year: leaveType?.max_days_per_year ?? 0,
      is_paid: leaveType?.is_paid ?? true,
      requires_approval: leaveType?.requires_approval ?? true,
      is_active: leaveType?.is_active ?? true,
    },
  });

  const onSubmit = async (values: LeaveTypeFormValues) => {
    setError(null);
    try {
      await onSave({ ...values, description: values.description || undefined });
      closeDrawer();
    } catch {
      setError("Failed to save leave type. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="leave-type-form"
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
            Name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("name")}
            autoFocus
            placeholder="Annual Leave"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Description
          </label>
          <textarea
            {...register("description")}
            rows={3}
            placeholder="When to use this leave type"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Max days per year
          </label>
          <input
            {...register("max_days_per_year")}
            type="number"
            min={0}
            step={1}
            placeholder="0 = unlimited"
            className={inputCls}
          />
          {errors.max_days_per_year && (
            <p className="text-xs text-red-400">
              {errors.max_days_per_year.message}
            </p>
          )}
        </div>

        <div className="flex items-center gap-6 pt-1">
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="is_paid"
              {...register("is_paid")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="is_paid"
              className="text-sm text-[var(--text-secondary)]"
            >
              Paid leave
            </label>
          </div>
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="requires_approval"
              {...register("requires_approval")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="requires_approval"
              className="text-sm text-[var(--text-secondary)]"
            >
              Requires approval
            </label>
          </div>
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
              className="text-sm text-[var(--text-secondary)]"
            >
              Active
            </label>
          </div>
        )}
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
          form="leave-type-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create leave type"}
        </button>
      </div>
    </div>
  );
}
