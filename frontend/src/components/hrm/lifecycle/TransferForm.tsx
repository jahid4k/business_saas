// src/components/hrm/lifecycle/TransferForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, Department, CreateTransferPayload } from "@/types/hrm";

const TRANSFER_TYPES = [
  { value: "department", label: "Department change" },
  { value: "location", label: "Location change" },
  { value: "reporting", label: "Reporting line change" },
  { value: "full", label: "Full transfer (all of the above)" },
];

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  transfer_type: z.string().min(1, "Transfer type is required"),
  to_department_id: z.string().optional(),
  to_manager_employee_id: z.string().optional(),
  to_location: z.string().optional(),
  effective_date: z.string().min(1, "Effective date is required"),
  reason: z.string().optional(),
  notes: z.string().optional(),
});
type TransferFormValues = z.infer<typeof schema>;

interface TransferFormProps {
  employees: Employee[];
  departments: Department[];
  defaultEmployeeId?: string;
  onSave: (employeeId: string, payload: CreateTransferPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function TransferForm({
  employees,
  departments,
  defaultEmployeeId,
  onSave,
}: TransferFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<TransferFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? "",
      transfer_type: "department",
      to_department_id: "",
      to_manager_employee_id: "",
      to_location: "",
      effective_date: "",
      reason: "",
      notes: "",
    },
  });

  const onSubmit = async (values: TransferFormValues) => {
    setError(null);
    try {
      await onSave(values.employee_id, {
        transfer_type:
          values.transfer_type as CreateTransferPayload["transfer_type"],
        to_department_id: values.to_department_id || undefined,
        to_manager_employee_id: values.to_manager_employee_id || undefined,
        to_location: values.to_location || undefined,
        effective_date: values.effective_date,
        reason: values.reason || undefined,
        notes: values.notes || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to create transfer record. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="transfer-form"
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

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Transfer type <span className="text-red-400">*</span>
          </label>
          <select {...register("transfer_type")} className={inputCls}>
            {TRANSFER_TYPES.map((t) => (
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

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            New department
          </label>
          <select {...register("to_department_id")} className={inputCls}>
            <option value="">No change</option>
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

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            New manager
          </label>
          <select {...register("to_manager_employee_id")} className={inputCls}>
            <option value="">No change</option>
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
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              New location
            </label>
            <input
              {...register("to_location")}
              placeholder="e.g. Dhaka office"
              className={inputCls}
            />
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
          <input
            {...register("reason")}
            placeholder="e.g. Team restructuring"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Notes
          </label>
          <textarea
            {...register("notes")}
            rows={3}
            placeholder="Internal notes"
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
          form="transfer-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create transfer"}
        </button>
      </div>
    </div>
  );
}
