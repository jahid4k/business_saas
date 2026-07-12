// src/components/hrm/warningtypes/WarningTypeForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { WarningType, CreateWarningTypePayload } from "@/types/hrm";

const ROLES = ["owner", "admin", "manager", "member"];

const optionalInt = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().int().optional(),
);

const schema = z.object({
  name: z.string().min(1, "Name is required").max(100, "Max 100 characters"),
  description: z.string().optional(),
  severity_level: optionalInt,
  can_be_issued_by: z.array(z.string()).optional(),
  requires_hr_approval: z.boolean().optional(),
  employee_can_respond: z.boolean().optional(),
  response_window_days: optionalInt,
  auto_generate_document: z.boolean().optional(),
  valid_duration_days: optionalInt,
  is_active: z.boolean().optional(),
});
type WarningTypeFormInput = z.input<typeof schema>;
type WarningTypeFormValues = z.infer<typeof schema>;

interface WarningTypeFormProps {
  warningType?: WarningType | null;
  onSave: (payload: CreateWarningTypePayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function WarningTypeForm({
  warningType,
  onSave,
}: WarningTypeFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!warningType;

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<WarningTypeFormInput, undefined, WarningTypeFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: warningType?.name ?? "",
      description: warningType?.description ?? "",
      severity_level: warningType?.severity_level ?? 5,
      can_be_issued_by: warningType?.can_be_issued_by ?? ["admin", "manager"],
      requires_hr_approval: warningType?.requires_hr_approval ?? false,
      employee_can_respond: warningType?.employee_can_respond ?? true,
      response_window_days: warningType?.response_window_days ?? 5,
      auto_generate_document: warningType?.auto_generate_document ?? false,
      valid_duration_days: warningType?.valid_duration_days ?? 0,
      is_active: warningType?.is_active ?? true,
    },
  });

  const onSubmit = async (values: WarningTypeFormValues) => {
    setError(null);
    try {
      await onSave({
        name: values.name,
        description: values.description || undefined,
        severity_level: values.severity_level,
        can_be_issued_by: values.can_be_issued_by,
        requires_hr_approval: values.requires_hr_approval,
        employee_can_respond: values.employee_can_respond,
        response_window_days: values.response_window_days,
        auto_generate_document: values.auto_generate_document,
        valid_duration_days: values.valid_duration_days,
      });
      closeDrawer();
    } catch {
      setError("Failed to save warning type. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="warning-type-form"
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
            placeholder="e.g. Verbal Warning, Final Written Warning"
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
            rows={2}
            placeholder="Optional"
            className={inputCls}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Severity (1–10)
            </label>
            <input
              {...register("severity_level")}
              type="number"
              min={1}
              max={10}
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Valid for (days)
            </label>
            <input
              {...register("valid_duration_days")}
              type="number"
              min={0}
              placeholder="0 = permanent"
              className={inputCls}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Can be issued by
          </label>
          <select
            multiple
            {...register("can_be_issued_by")}
            className={`${inputCls} h-24`}
          >
            {ROLES.map((r) => (
              <option
                key={r}
                value={r}
                style={{ background: "var(--bg-elevated)" }}
              >
                {r}
              </option>
            ))}
          </select>
          <p className="text-xs text-[var(--text-muted)]">
            Hold Ctrl/Cmd to select multiple.
          </p>
        </div>

        <div className="space-y-2">
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="requires_hr_approval"
              {...register("requires_hr_approval")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="requires_hr_approval"
              className="text-sm text-[var(--text-secondary)]"
            >
              Requires HR approval before issuing
            </label>
          </div>
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="employee_can_respond"
              {...register("employee_can_respond")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="employee_can_respond"
              className="text-sm text-[var(--text-secondary)]"
            >
              Employee can respond / appeal
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

        {warningType?.employee_can_respond !== undefined && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Response window (days)
            </label>
            <input
              {...register("response_window_days")}
              type="number"
              min={0}
              className={inputCls}
            />
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
          form="warning-type-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create warning type"}
        </button>
      </div>
    </div>
  );
}
