// src/components/hrm/payroll/PayrollRunForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { AttendancePeriod, CreatePayslipRunPayload } from "@/types/hrm";

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const schema = z.object({
  year: z.coerce.number().int().min(2000),
  month: z.coerce.number().int().min(1).max(12),
  description: z.string().optional(),
  currency: z.string().optional(),
  attendance_period_id: z.string().optional(),
});
type PayrollRunFormInput = z.input<typeof schema>;
type PayrollRunFormValues = z.infer<typeof schema>;

interface PayrollRunFormProps {
  attendancePeriods: AttendancePeriod[];
  onSave: (payload: CreatePayslipRunPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function PayrollRunForm({
  attendancePeriods,
  onSave,
}: PayrollRunFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const now = new Date();

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<PayrollRunFormInput, undefined, PayrollRunFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      year: now.getFullYear(),
      month: now.getMonth() + 1,
      description: "",
      currency: "BDT",
      attendance_period_id: "",
    },
  });

  const onSubmit = async (values: PayrollRunFormValues) => {
    setError(null);
    try {
      await onSave({
        year: values.year,
        month: values.month,
        description: values.description || undefined,
        currency: values.currency || undefined,
        attendance_period_id: values.attendance_period_id || undefined,
      });
      closeDrawer();
    } catch {
      setError(
        "Failed to create payroll run. It may already exist for this period.",
      );
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="payroll-run-form"
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
              Month
            </label>
            <select {...register("month")} autoFocus className={inputCls}>
              {MONTHS.map((m, i) => (
                <option
                  key={m}
                  value={i + 1}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {m}
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Year
            </label>
            <input {...register("year")} type="number" className={inputCls} />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Description
          </label>
          <input
            {...register("description")}
            placeholder="Optional"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Currency
          </label>
          <input
            {...register("currency")}
            placeholder="BDT"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Link attendance period
          </label>
          <select {...register("attendance_period_id")} className={inputCls}>
            <option value="">None — skip work-day tracking</option>
            {attendancePeriods.map((p) => (
              <option
                key={p.id}
                value={p.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {MONTHS[p.period_month - 1]} {p.period_year} ({p.status})
              </option>
            ))}
          </select>
          <p className="text-xs text-(--text-muted)">
            If linked, that period must be finalized or locked before you can
            compute this run.
          </p>
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
          form="payroll-run-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create run"}
        </button>
      </div>
    </div>
  );
}
