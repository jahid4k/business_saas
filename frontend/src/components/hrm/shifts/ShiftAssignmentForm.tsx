// src/components/hrm/shifts/ShiftAssignmentForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type {
  Shift,
  Employee,
  Department,
  AssignShiftPayload,
} from "@/types/hrm";

const schema = z.object({
  shift_id: z.string().min(1, "Shift is required"),
  assignee_type: z.string().min(1),
  assignee_id: z.string().min(1, "Select who this applies to"),
  effective_date: z.string().min(1, "Effective date is required"),
  end_date: z.string().optional(),
});
type ShiftAssignmentFormValues = z.infer<typeof schema>;

interface ShiftAssignmentFormProps {
  shifts: Shift[];
  employees: Employee[];
  departments: Department[];
  onSave: (payload: AssignShiftPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function ShiftAssignmentForm({
  shifts,
  employees,
  departments,
  onSave,
}: ShiftAssignmentFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<ShiftAssignmentFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      shift_id: "",
      assignee_type: "organization",
      assignee_id: "",
      effective_date: "",
      end_date: "",
    },
  });

  const assigneeType = watch("assignee_type");

  const onSubmit = async (values: ShiftAssignmentFormValues) => {
    setError(null);
    try {
      await onSave({
        shift_id: values.shift_id,
        assignee_type:
          values.assignee_type as AssignShiftPayload["assignee_type"],
        assignee_id:
          values.assignee_type === "organization" ? "org" : values.assignee_id,
        effective_date: values.effective_date,
        end_date: values.end_date || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to assign shift. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="shift-assignment-form"
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
            Shift <span className="text-red-400">*</span>
          </label>
          <select {...register("shift_id")} autoFocus className={inputCls}>
            <option value="">Select shift</option>
            {shifts.map((s) => (
              <option
                key={s.id}
                value={s.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {s.name}
              </option>
            ))}
          </select>
          {errors.shift_id && (
            <p className="text-xs text-red-400">{errors.shift_id.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Applies to
          </label>
          <select {...register("assignee_type")} className={inputCls}>
            <option
              value="organization"
              style={{ background: "var(--bg-elevated)" }}
            >
              Whole organization
            </option>
            <option
              value="department"
              style={{ background: "var(--bg-elevated)" }}
            >
              A department
            </option>
            <option
              value="employee"
              style={{ background: "var(--bg-elevated)" }}
            >
              A specific employee
            </option>
          </select>
        </div>

        {assigneeType === "department" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Department
            </label>
            <select {...register("assignee_id")} className={inputCls}>
              <option value="">Select department</option>
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
            {errors.assignee_id && (
              <p className="text-xs text-red-400">
                {errors.assignee_id.message}
              </p>
            )}
          </div>
        )}

        {assigneeType === "employee" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Employee
            </label>
            <select {...register("assignee_id")} className={inputCls}>
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
            {errors.assignee_id && (
              <p className="text-xs text-red-400">
                {errors.assignee_id.message}
              </p>
            )}
          </div>
        )}

        <div className="grid grid-cols-2 gap-3">
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
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              End date
            </label>
            <input {...register("end_date")} type="date" className={inputCls} />
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
          form="shift-assignment-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Assigning…" : "Assign shift"}
        </button>
      </div>
    </div>
  );
}
