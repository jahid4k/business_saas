// src/components/hrm/lifecycle/ResignationForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, SubmitResignationPayload } from "@/types/hrm";

const REASON_CATEGORIES = [
  { value: "personal", label: "Personal" },
  { value: "career_growth", label: "Career growth" },
  { value: "better_opportunity", label: "Better opportunity" },
  { value: "relocation", label: "Relocation" },
  { value: "health", label: "Health" },
  { value: "retirement", label: "Retirement" },
  { value: "other", label: "Other" },
];

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  resignation_date: z.string().min(1, "Resignation date is required"),
  reason_category: z.string().min(1, "Reason is required"),
  reason_remarks: z.string().optional(),
  last_working_date: z.string().optional(),
  is_notice_waived: z.boolean().optional(),
});
type ResignationFormValues = z.infer<typeof schema>;

interface ResignationFormProps {
  employees: Employee[];
  defaultEmployeeId?: string;
  onSave: (
    employeeId: string,
    payload: SubmitResignationPayload,
  ) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function ResignationForm({
  employees,
  defaultEmployeeId,
  onSave,
}: ResignationFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ResignationFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? "",
      resignation_date: "",
      reason_category: "",
      reason_remarks: "",
      last_working_date: "",
      is_notice_waived: false,
    },
  });

  const onSubmit = async (values: ResignationFormValues) => {
    setError(null);
    try {
      await onSave(values.employee_id, {
        resignation_date: values.resignation_date,
        reason_category:
          values.reason_category as SubmitResignationPayload["reason_category"],
        reason_remarks: values.reason_remarks || undefined,
        last_working_date: values.last_working_date || undefined,
        is_notice_waived: values.is_notice_waived,
      });
      closeDrawer();
    } catch {
      setError("Failed to submit resignation. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="resignation-form"
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
            Employee <span className="text-red-400">*</span>
          </label>
          <select {...register("employee_id")} autoFocus className={inputCls}>
            <option value="">Select employee</option>
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
          {errors.employee_id && (
            <p className="text-xs text-red-400">{errors.employee_id.message}</p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Resignation date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("resignation_date")}
              type="date"
              className={inputCls}
            />
            {errors.resignation_date && (
              <p className="text-xs text-red-400">
                {errors.resignation_date.message}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Last working date
            </label>
            <input
              {...register("last_working_date")}
              type="date"
              className={inputCls}
            />
            <p className="text-xs text-[var(--text-muted)]">
              Blank = auto from notice period.
            </p>
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Reason category <span className="text-red-400">*</span>
          </label>
          <select {...register("reason_category")} className={inputCls}>
            <option value="">Select reason</option>
            {REASON_CATEGORIES.map((r) => (
              <option
                key={r.value}
                value={r.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {r.label}
              </option>
            ))}
          </select>
          {errors.reason_category && (
            <p className="text-xs text-red-400">
              {errors.reason_category.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Remarks
          </label>
          <textarea
            {...register("reason_remarks")}
            rows={3}
            placeholder="Optional detail"
            className={inputCls}
          />
        </div>

        <div className="flex items-center gap-2.5 pt-1">
          <input
            type="checkbox"
            id="is_notice_waived"
            {...register("is_notice_waived")}
            className="w-4 h-4 accent-purple-600"
          />
          <label
            htmlFor="is_notice_waived"
            className="text-sm text-[var(--text-secondary)]"
          >
            Waive notice period
          </label>
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
          form="resignation-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Submitting…" : "Submit resignation"}
        </button>
      </div>
    </div>
  );
}
