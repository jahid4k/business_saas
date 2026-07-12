// src/components/hrm/holidays/HolidayCalendarForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { CreateCalendarPayload } from "@/types/hrm";

const schema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
  country_code: z.string().optional(),
  year: z.coerce.number().int().min(2000).max(2100),
});
type HolidayCalendarFormInput = z.input<typeof schema>;
type HolidayCalendarFormValues = z.infer<typeof schema>;

interface HolidayCalendarFormProps {
  onSave: (payload: CreateCalendarPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function HolidayCalendarForm({
  onSave,
}: HolidayCalendarFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const now = new Date();

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<HolidayCalendarFormInput, undefined, HolidayCalendarFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      description: "",
      country_code: "BD",
      year: now.getFullYear(),
    },
  });

  const onSubmit = async (values: HolidayCalendarFormValues) => {
    setError(null);
    try {
      await onSave({
        name: values.name,
        description: values.description || undefined,
        country_code: values.country_code || undefined,
        year: values.year,
      });
      closeDrawer();
    } catch {
      setError(
        "Failed to create calendar. It may already exist for this name/year.",
      );
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="holiday-calendar-form"
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
            Name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("name")}
            autoFocus
            placeholder="e.g. Bangladesh Public Holidays"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Year
            </label>
            <input {...register("year")} type="number" className={inputCls} />
            {errors.year && (
              <p className="text-xs text-red-400">{errors.year.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Country code
            </label>
            <input
              {...register("country_code")}
              placeholder="BD"
              className={inputCls}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Description
          </label>
          <textarea
            {...register("description")}
            rows={2}
            placeholder="Optional"
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
          form="holiday-calendar-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create calendar"}
        </button>
      </div>
    </div>
  );
}
