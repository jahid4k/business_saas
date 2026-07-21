// src/components/hrm/lifecycle/PromotionForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type {
  Employee,
  Position,
  Department,
  CreatePromotionPayload,
} from "@/types/hrm";

const optionalNumber = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().min(0).optional(),
);

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  to_position_id: z.string().min(1, "New position is required"),
  to_department_id: z.string().optional(),
  new_basic_pay: optionalNumber,
  effective_date: z.string().min(1, "Effective date is required"),
  reason: z.string().optional(),
  notes: z.string().optional(),
});
type PromotionFormInput = z.input<typeof schema>;
type PromotionFormValues = z.infer<typeof schema>;

interface PromotionFormProps {
  employees: Employee[];
  positions: Position[];
  departments: Department[];
  defaultEmployeeId?: string;
  onSave: (
    employeeId: string,
    payload: CreatePromotionPayload,
  ) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function PromotionForm({
  employees,
  positions,
  departments,
  defaultEmployeeId,
  onSave,
}: PromotionFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<PromotionFormInput, undefined, PromotionFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? "",
      to_position_id: "",
      to_department_id: "",
      new_basic_pay: "",
      effective_date: "",
      reason: "",
      notes: "",
    },
  });

  const onSubmit = async (values: PromotionFormValues) => {
    setError(null);
    try {
      await onSave(values.employee_id, {
        to_position_id: values.to_position_id,
        to_department_id: values.to_department_id || undefined,
        new_basic_pay: values.new_basic_pay,
        effective_date: values.effective_date,
        reason: values.reason || undefined,
        notes: values.notes || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to create promotion record. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="promotion-form"
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
            New position <span className="text-red-400">*</span>
          </label>
          <select {...register("to_position_id")} className={inputCls}>
            <option value="">Select position</option>
            {positions.map((p) => (
              <option
                key={p.id}
                value={p.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {p.title}
              </option>
            ))}
          </select>
          {errors.to_position_id && (
            <p className="text-xs text-red-400">
              {errors.to_position_id.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
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

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              New basic pay
            </label>
            <input
              {...register("new_basic_pay")}
              type="number"
              min={0}
              step="0.01"
              placeholder="Optional"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
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
          <label className="block text-sm font-medium text-(--text-secondary)">
            Reason
          </label>
          <input
            {...register("reason")}
            placeholder="e.g. Annual review"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
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
          form="promotion-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create promotion"}
        </button>
      </div>
    </div>
  );
}
