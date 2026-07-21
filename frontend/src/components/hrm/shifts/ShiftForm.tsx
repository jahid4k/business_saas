// src/components/hrm/shifts/ShiftForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Shift, CreateShiftPayload } from "@/types/hrm";

const DAYS = [
  { value: "MO", label: "Mon" },
  { value: "TU", label: "Tue" },
  { value: "WE", label: "Wed" },
  { value: "TH", label: "Thu" },
  { value: "FR", label: "Fri" },
  { value: "SA", label: "Sat" },
  { value: "SU", label: "Sun" },
];

const optionalNumber = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().min(0).optional(),
);

const schema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
  shift_type: z.string().min(1),
  start_time: z.string().optional(),
  end_time: z.string().optional(),
  weekly_hours_target: optionalNumber,
  break_minutes: optionalNumber,
  working_days: z.array(z.string()).optional(),
  track_overtime: z.boolean().optional(),
  overtime_threshold_hours: optionalNumber,
  track_breaks: z.boolean().optional(),
  is_default: z.boolean().optional(),
});
type ShiftFormInput = z.input<typeof schema>;
type ShiftFormValues = z.infer<typeof schema>;

interface ShiftFormProps {
  shift?: Shift | null;
  onSave: (payload: CreateShiftPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function ShiftForm({ shift, onSave }: ShiftFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!shift;

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<ShiftFormInput, undefined, ShiftFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: shift?.name ?? "",
      description: shift?.description ?? "",
      shift_type: shift?.shift_type ?? "fixed",
      start_time: shift?.start_time ?? "09:00",
      end_time: shift?.end_time ?? "18:00",
      weekly_hours_target: shift?.weekly_hours_target ?? "",
      break_minutes: shift?.break_minutes ?? 60,
      working_days: shift?.working_days ?? ["MO", "TU", "WE", "TH", "FR"],
      track_overtime: shift?.track_overtime ?? false,
      overtime_threshold_hours: shift?.overtime_threshold_hours ?? "",
      track_breaks: shift?.track_breaks ?? true,
      is_default: shift?.is_default ?? false,
    },
  });

  const shiftType = watch("shift_type");

  const onSubmit = async (values: ShiftFormValues) => {
    setError(null);
    if (shiftType === "fixed" && (!values.start_time || !values.end_time)) {
      setError("Start and end time are required for fixed shifts.");
      return;
    }
    if (shiftType === "flexible" && !values.weekly_hours_target) {
      setError("Weekly hours target is required for flexible shifts.");
      return;
    }
    try {
      await onSave({
        name: values.name,
        description: values.description || undefined,
        shift_type: values.shift_type as CreateShiftPayload["shift_type"],
        start_time: shiftType === "fixed" ? values.start_time : undefined,
        end_time: shiftType === "fixed" ? values.end_time : undefined,
        weekly_hours_target: values.weekly_hours_target,
        break_minutes: values.break_minutes,
        working_days: values.working_days,
        track_overtime: values.track_overtime,
        overtime_threshold_hours: values.overtime_threshold_hours,
        track_breaks: values.track_breaks,
        is_default: values.is_default,
      });
      closeDrawer();
    } catch {
      setError("Failed to save shift. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="shift-form"
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
            Name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("name")}
            autoFocus
            placeholder="e.g. General Shift"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Type
          </label>
          <select {...register("shift_type")} className={inputCls}>
            <option value="fixed" style={{ background: "var(--bg-elevated)" }}>
              Fixed hours
            </option>
            <option
              value="flexible"
              style={{ background: "var(--bg-elevated)" }}
            >
              Flexible hours
            </option>
          </select>
        </div>

        {shiftType === "fixed" ? (
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-(--text-secondary)">
                Start time
              </label>
              <input
                {...register("start_time")}
                type="time"
                className={inputCls}
              />
            </div>
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-(--text-secondary)">
                End time
              </label>
              <input
                {...register("end_time")}
                type="time"
                className={inputCls}
              />
            </div>
          </div>
        ) : (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Weekly hours target
            </label>
            <input
              {...register("weekly_hours_target")}
              type="number"
              step="0.5"
              min={0}
              className={inputCls}
            />
          </div>
        )}

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Working days
          </label>
          <div className="flex flex-wrap gap-2">
            {DAYS.map((d) => (
              <label
                key={d.value}
                className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-(--bg-elevated) border border-(--border) text-xs text-(--text-secondary) cursor-pointer"
              >
                <input
                  type="checkbox"
                  value={d.value}
                  {...register("working_days")}
                  className="w-3.5 h-3.5 accent-purple-600"
                />
                {d.label}
              </label>
            ))}
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Break minutes
          </label>
          <input
            {...register("break_minutes")}
            type="number"
            min={0}
            className={inputCls}
          />
        </div>

        <div className="space-y-2">
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="track_overtime"
              {...register("track_overtime")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="track_overtime"
              className="text-sm text-(--text-secondary)"
            >
              Track overtime
            </label>
          </div>
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="track_breaks"
              {...register("track_breaks")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="track_breaks"
              className="text-sm text-(--text-secondary)"
            >
              Track breaks
            </label>
          </div>
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="is_default"
              {...register("is_default")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="is_default"
              className="text-sm text-(--text-secondary)"
            >
              Default shift for the org
            </label>
          </div>
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
          form="shift-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create shift"}
        </button>
      </div>
    </div>
  );
}
