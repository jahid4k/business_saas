// src/components/hrm/attendance/AttendanceRecordForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, CreateAttendanceRecordPayload } from "@/types/hrm";

const DAY_TYPES = [
  { value: "present", label: "Present" },
  { value: "absent", label: "Absent" },
  { value: "half_day", label: "Half day" },
  { value: "late", label: "Late" },
  { value: "on_leave", label: "On leave" },
  { value: "holiday", label: "Holiday" },
  { value: "weekend", label: "Weekend" },
  { value: "work_from_home", label: "Work from home" },
];

const optionalInt = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().int().min(0).optional(),
);

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  date: z.string().min(1, "Date is required"),
  check_in: z.string().optional(),
  check_out: z.string().optional(),
  break_minutes: optionalInt,
  day_type: z.string().min(1, "Day type is required"),
  notes: z.string().optional(),
});
type AttendanceRecordFormInput = z.input<typeof schema>;
type AttendanceRecordFormValues = z.infer<typeof schema>;

interface AttendanceRecordFormProps {
  employees: Employee[];
  defaultEmployeeId?: string;
  defaultDate?: string;
  onSave: (payload: CreateAttendanceRecordPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function AttendanceRecordForm({
  employees,
  defaultEmployeeId,
  defaultDate,
  onSave,
}: AttendanceRecordFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<AttendanceRecordFormInput, undefined, AttendanceRecordFormValues>(
    {
      resolver: zodResolver(schema),
      defaultValues: {
        employee_id: defaultEmployeeId ?? "",
        date: defaultDate ?? "",
        check_in: "",
        check_out: "",
        break_minutes: "",
        day_type: "present",
        notes: "",
      },
    },
  );

  const onSubmit = async (values: AttendanceRecordFormValues) => {
    setError(null);
    try {
      await onSave({
        employee_id: values.employee_id,
        date: values.date,
        check_in: values.check_in || undefined,
        check_out: values.check_out || undefined,
        break_minutes: values.break_minutes,
        day_type: values.day_type as CreateAttendanceRecordPayload["day_type"],
        source: "manual",
        notes: values.notes || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to save attendance record. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="attendance-record-form"
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

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Date <span className="text-red-400">*</span>
            </label>
            <input {...register("date")} type="date" className={inputCls} />
            {errors.date && (
              <p className="text-xs text-red-400">{errors.date.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Day type <span className="text-red-400">*</span>
            </label>
            <select {...register("day_type")} className={inputCls}>
              {DAY_TYPES.map((d) => (
                <option
                  key={d.value}
                  value={d.value}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {d.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Check-in
            </label>
            <input {...register("check_in")} type="time" className={inputCls} />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Check-out
            </label>
            <input
              {...register("check_out")}
              type="time"
              className={inputCls}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Break (minutes)
          </label>
          <input
            {...register("break_minutes")}
            type="number"
            min={0}
            step={1}
            placeholder="0"
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
            placeholder="Optional"
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
          form="attendance-record-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Saving…" : "Save record"}
        </button>
      </div>
    </div>
  );
}
