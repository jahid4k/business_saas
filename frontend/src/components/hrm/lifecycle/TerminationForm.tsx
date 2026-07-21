// src/components/hrm/lifecycle/TerminationForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, CreateTerminationPayload } from "@/types/hrm";

const TERMINATION_TYPES = [
  { value: "voluntary", label: "Voluntary" },
  { value: "involuntary", label: "Involuntary" },
  { value: "layoff", label: "Layoff" },
  { value: "retirement", label: "Retirement" },
  { value: "contract_end", label: "Contract end" },
  { value: "probation_fail", label: "Probation fail" },
];

const optionalNumber = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().min(0).optional(),
);

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  termination_type: z.string().min(1, "Termination type is required"),
  termination_date: z.string().min(1, "Termination date is required"),
  last_working_date: z.string().min(1, "Last working date is required"),
  reason: z.string().optional(),
  internal_notes: z.string().optional(),
  severance_amount: optionalNumber,
  severance_currency: z.string().optional(),
  is_rehire_eligible: z.boolean().optional(),
});
type TerminationFormInput = z.input<typeof schema>;
type TerminationFormValues = z.infer<typeof schema>;

interface TerminationFormProps {
  employees: Employee[];
  defaultEmployeeId?: string;
  onSave: (
    employeeId: string,
    payload: CreateTerminationPayload,
  ) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function TerminationForm({
  employees,
  defaultEmployeeId,
  onSave,
}: TerminationFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<TerminationFormInput, undefined, TerminationFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? "",
      termination_type: "involuntary",
      termination_date: "",
      last_working_date: "",
      reason: "",
      internal_notes: "",
      severance_amount: "",
      severance_currency: "BDT",
      is_rehire_eligible: true,
    },
  });

  const onSubmit = async (values: TerminationFormValues) => {
    setError(null);
    try {
      await onSave(values.employee_id, {
        termination_type:
          values.termination_type as CreateTerminationPayload["termination_type"],
        termination_date: values.termination_date,
        last_working_date: values.last_working_date,
        reason: values.reason || undefined,
        internal_notes: values.internal_notes || undefined,
        severance_amount: values.severance_amount,
        severance_currency: values.severance_currency || undefined,
        is_rehire_eligible: values.is_rehire_eligible,
      });
      closeDrawer();
    } catch {
      setError("Failed to create termination record. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="termination-form"
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

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Termination type <span className="text-red-400">*</span>
          </label>
          <select {...register("termination_type")} className={inputCls}>
            {TERMINATION_TYPES.map((t) => (
              <option
                key={t.value}
                value={t.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {t.label}
              </option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Termination date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("termination_date")}
              type="date"
              className={inputCls}
            />
            {errors.termination_date && (
              <p className="text-xs text-red-400">
                {errors.termination_date.message}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Last working date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("last_working_date")}
              type="date"
              className={inputCls}
            />
            {errors.last_working_date && (
              <p className="text-xs text-red-400">
                {errors.last_working_date.message}
              </p>
            )}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Severance amount
            </label>
            <input
              {...register("severance_amount")}
              type="number"
              min={0}
              step="0.01"
              placeholder="Optional"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Currency
            </label>
            <input
              {...register("severance_currency")}
              placeholder="BDT"
              className={inputCls}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Reason
          </label>
          <input
            {...register("reason")}
            placeholder="Visible in employee record"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Internal notes
          </label>
          <textarea
            {...register("internal_notes")}
            rows={3}
            placeholder="HR-only notes"
            className={inputCls}
          />
        </div>

        <div className="flex items-center gap-2.5 pt-1">
          <input
            type="checkbox"
            id="is_rehire_eligible"
            {...register("is_rehire_eligible")}
            className="w-4 h-4 accent-purple-600"
          />
          <label
            htmlFor="is_rehire_eligible"
            className="text-sm text-(--text-secondary)"
          >
            Eligible for rehire
          </label>
        </div>
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
          form="termination-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create termination"}
        </button>
      </div>
    </div>
  );
}
