// src/components/hrm/salary/EmployeeSalaryForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { SalaryStructure, AssignSalaryPayload } from "@/types/hrm";

const REASONS = [
  { value: "joining", label: "Joining" },
  { value: "promotion", label: "Promotion" },
  { value: "annual_revision", label: "Annual revision" },
  { value: "transfer", label: "Transfer" },
  { value: "correction", label: "Correction" },
  { value: "other", label: "Other" },
];

const schema = z.object({
  structure_id: z.string().optional(),
  basic_pay: z.coerce.number().min(0, "Must be 0 or more"),
  effective_date: z.string().min(1, "Effective date is required"),
  change_reason: z.string().min(1, "Reason is required"),
  change_notes: z.string().optional(),
});
type EmployeeSalaryFormInput = z.input<typeof schema>;
type EmployeeSalaryFormValues = z.infer<typeof schema>;

interface EmployeeSalaryFormProps {
  employeeName: string;
  structures: SalaryStructure[];
  onSave: (payload: AssignSalaryPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function EmployeeSalaryForm({
  employeeName,
  structures,
  onSave,
}: EmployeeSalaryFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<EmployeeSalaryFormInput, undefined, EmployeeSalaryFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      structure_id: "",
      basic_pay: "" as unknown as number,
      effective_date: "",
      change_reason: "annual_revision",
      change_notes: "",
    },
  });

  const onSubmit = async (values: EmployeeSalaryFormValues) => {
    setError(null);
    try {
      await onSave({
        structure_id: values.structure_id || undefined,
        basic_pay: values.basic_pay,
        effective_date: values.effective_date,
        change_reason:
          values.change_reason as AssignSalaryPayload["change_reason"],
        change_notes: values.change_notes || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to assign salary. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="employee-salary-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <p className="text-sm text-[var(--text-secondary)]">
          Assigning new salary for{" "}
          <span className="text-[var(--text-primary)] font-medium">
            {employeeName}
          </span>
          . This adds a new record — history is preserved.
        </p>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Salary structure
          </label>
          <select {...register("structure_id")} autoFocus className={inputCls}>
            <option value="">No structure (basic pay only)</option>
            {structures.map((s) => (
              <option
                key={s.id}
                value={s.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {s.name}
                {s.grade_label ? ` (${s.grade_label})` : ""}
              </option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Basic pay <span className="text-red-400">*</span>
            </label>
            <input
              {...register("basic_pay")}
              type="number"
              min={0}
              step="0.01"
              className={inputCls}
            />
            {errors.basic_pay && (
              <p className="text-xs text-red-400">{errors.basic_pay.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Effective date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("effective_date")}
              type="date"
              className={inputCls}
            />
            {errors.effective_date && (
              <p className="text-xs text-red-400">
                {errors.effective_date.message}
              </p>
            )}
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Reason
          </label>
          <select {...register("change_reason")} className={inputCls}>
            {REASONS.map((r) => (
              <option
                key={r.value}
                value={r.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {r.label}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Notes
          </label>
          <textarea
            {...register("change_notes")}
            rows={2}
            placeholder="Optional"
            className={inputCls}
          />
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
          form="employee-salary-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Saving…" : "Assign salary"}
        </button>
      </div>
    </div>
  );
}
