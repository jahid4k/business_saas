// src/components/hrm/leave/LeaveRequestForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, LeaveType } from "@/types/hrm";

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  leave_type_id: z.string().min(1, "Leave type is required"),
  start_date: z.string().min(1, "Start date is required"),
  end_date: z.string().min(1, "End date is required"),
  total_days: z.coerce.number().gt(0, "Must be greater than 0"),
  reason: z.string().optional(),
});
type LeaveRequestFormInput = z.input<typeof schema>;
type LeaveRequestFormValues = z.infer<typeof schema>;

interface LeaveRequestFormProps {
  employees: Employee[];
  leaveTypes: LeaveType[];
  defaultEmployeeId?: string;
  onSave: (values: LeaveRequestFormValues) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

// Inclusive calendar-day count between two YYYY-MM-DD dates
function daysBetween(start: string, end: string): number {
  if (!start || !end) return 0;
  const s = new Date(start);
  const e = new Date(end);
  const diff = Math.round((e.getTime() - s.getTime()) / 86_400_000) + 1;
  return diff > 0 ? diff : 0;
}

export default function LeaveRequestForm({
  employees,
  leaveTypes,
  defaultEmployeeId,
  onSave,
}: LeaveRequestFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const [autoDays, setAutoDays] = useState(true);

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<LeaveRequestFormInput, undefined, LeaveRequestFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? "",
      leave_type_id: "",
      start_date: "",
      end_date: "",
      total_days: 0,
      reason: "",
    },
  });

  const startDate = watch("start_date");
  const endDate = watch("end_date");
  const totalDaysField = register("total_days");

  // Suggest total_days from the date range until the user edits it manually
  useEffect(() => {
    if (!autoDays) return;
    setValue("total_days", daysBetween(startDate, endDate));
  }, [startDate, endDate, autoDays, setValue]);

  const onSubmit = async (values: LeaveRequestFormValues) => {
    setError(null);
    try {
      await onSave({ ...values, reason: values.reason || undefined });
      closeDrawer();
    } catch {
      setError("Failed to submit leave request. Please try again.");
    }
  };

  const activeLeaveTypes = leaveTypes.filter((lt) => lt.is_active);

  return (
    <div className="flex flex-col h-full">
      <form
        id="leave-request-form"
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
            Leave type <span className="text-red-400">*</span>
          </label>
          <select {...register("leave_type_id")} className={inputCls}>
            <option value="">Select leave type</option>
            {activeLeaveTypes.map((lt) => (
              <option
                key={lt.id}
                value={lt.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {lt.name}
                {lt.max_days_per_year > 0
                  ? ` (${lt.max_days_per_year} days/yr)`
                  : ""}
              </option>
            ))}
          </select>
          {errors.leave_type_id && (
            <p className="text-xs text-red-400">
              {errors.leave_type_id.message}
            </p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Start date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("start_date")}
              type="date"
              className={inputCls}
            />
            {errors.start_date && (
              <p className="text-xs text-red-400">
                {errors.start_date.message}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              End date <span className="text-red-400">*</span>
            </label>
            <input {...register("end_date")} type="date" className={inputCls} />
            {errors.end_date && (
              <p className="text-xs text-red-400">{errors.end_date.message}</p>
            )}
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Total days <span className="text-red-400">*</span>
          </label>
          <input
            type="number"
            min={0.5}
            step={0.5}
            name={totalDaysField.name}
            ref={totalDaysField.ref}
            onBlur={totalDaysField.onBlur}
            onChange={(e) => {
              setAutoDays(false);
              totalDaysField.onChange(e);
            }}
            className={inputCls}
          />
          <p className="text-xs text-(--text-muted)">
            Auto-filled from the date range — edit for half-days or excluded
            weekends.
          </p>
          {errors.total_days && (
            <p className="text-xs text-red-400">{errors.total_days.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Reason
          </label>
          <textarea
            {...register("reason")}
            rows={3}
            placeholder="Optional note"
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
          form="leave-request-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Submitting…" : "Submit request"}
        </button>
      </div>
    </div>
  );
}
