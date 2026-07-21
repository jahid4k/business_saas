// src/components/hrm/attendance/RegularizeForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type {
  AttendanceRecord,
  RegularizeAttendancePayload,
} from "@/types/hrm";

const schema = z.object({
  new_check_in: z.string().optional(),
  new_check_out: z.string().optional(),
  reason: z.string().min(1, "Reason is required"),
});
type RegularizeFormValues = z.infer<typeof schema>;

interface RegularizeFormProps {
  record: AttendanceRecord;
  onSave: (payload: RegularizeAttendancePayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function RegularizeForm({
  record,
  onSave,
}: RegularizeFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegularizeFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      new_check_in: record.check_in_time ?? "",
      new_check_out: record.check_out_time ?? "",
      reason: "",
    },
  });

  const onSubmit = async (values: RegularizeFormValues) => {
    setError(null);
    try {
      await onSave({
        new_check_in: values.new_check_in || undefined,
        new_check_out: values.new_check_out || undefined,
        reason: values.reason,
      });
      closeDrawer();
    } catch {
      setError("Failed to submit regularization. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="regularize-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Corrected check-in
            </label>
            <input
              {...register("new_check_in")}
              type="time"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Corrected check-out
            </label>
            <input
              {...register("new_check_out")}
              type="time"
              className={inputCls}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Reason <span className="text-red-400">*</span>
          </label>
          <textarea
            {...register("reason")}
            rows={3}
            placeholder="Why does this need correcting?"
            className={inputCls}
          />
          {errors.reason && (
            <p className="text-xs text-red-400">{errors.reason.message}</p>
          )}
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
          form="regularize-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Submitting…" : "Submit correction"}
        </button>
      </div>
    </div>
  );
}
