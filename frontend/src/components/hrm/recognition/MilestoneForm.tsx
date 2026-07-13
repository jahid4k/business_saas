// src/components/hrm/recognition/MilestoneForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, CreateMilestonePayload } from "@/types/hrm";

const MILESTONE_TYPES = [
  { value: "work_anniversary", label: "Work anniversary" },
  { value: "birthday", label: "Birthday" },
  { value: "probation_complete", label: "Probation complete" },
  { value: "promotion", label: "Promotion" },
  { value: "contract_renewal", label: "Contract renewal" },
  { value: "retirement", label: "Retirement" },
  { value: "custom", label: "Custom" },
];

const optionalInt = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().int().min(0).optional(),
);

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  milestone_type: z.string().min(1, "Type is required"),
  title: z.string().min(1, "Title is required"),
  description: z.string().optional(),
  milestone_date: z.string().min(1, "Date is required"),
  years_count: optionalInt,
  create_award: z.boolean().optional(),
  create_announcement: z.boolean().optional(),
  create_calendar_event: z.boolean().optional(),
});
type MilestoneFormInput = z.input<typeof schema>;
type MilestoneFormValues = z.infer<typeof schema>;

interface MilestoneFormProps {
  employees: Employee[];
  onSave: (payload: CreateMilestonePayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function MilestoneForm({
  employees,
  onSave,
}: MilestoneFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<MilestoneFormInput, undefined, MilestoneFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: "",
      milestone_type: "work_anniversary",
      title: "",
      description: "",
      milestone_date: "",
      years_count: "",
      create_award: false,
      create_announcement: false,
      create_calendar_event: false,
    },
  });

  const onSubmit = async (values: MilestoneFormValues) => {
    setError(null);
    try {
      await onSave({
        employee_id: values.employee_id,
        milestone_type:
          values.milestone_type as CreateMilestonePayload["milestone_type"],
        title: values.title,
        description: values.description || undefined,
        milestone_date: values.milestone_date,
        years_count: values.years_count,
        create_award: values.create_award,
        create_announcement: values.create_announcement,
        create_calendar_event: values.create_calendar_event,
      });
      closeDrawer();
    } catch {
      setError("Failed to create milestone. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="milestone-form"
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
            Type
          </label>
          <select {...register("milestone_type")} className={inputCls}>
            {MILESTONE_TYPES.map((t) => (
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
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            placeholder="e.g. 5 Years at BusinessSAAS"
            className={inputCls}
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("milestone_date")}
              type="date"
              className={inputCls}
            />
            {errors.milestone_date && (
              <p className="text-xs text-red-400">
                {errors.milestone_date.message}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Years count
            </label>
            <input
              {...register("years_count")}
              type="number"
              min={0}
              step={1}
              placeholder="Optional"
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

        <p className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] pt-2">
          Also create
        </p>
        <div className="space-y-2">
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="create_award"
              {...register("create_award")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="create_award"
              className="text-sm text-[var(--text-secondary)]"
            >
              An award
            </label>
          </div>
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="create_announcement"
              {...register("create_announcement")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="create_announcement"
              className="text-sm text-[var(--text-secondary)]"
            >
              An announcement
            </label>
          </div>
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="create_calendar_event"
              {...register("create_calendar_event")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="create_calendar_event"
              className="text-sm text-[var(--text-secondary)]"
            >
              A calendar event
            </label>
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
          form="milestone-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create milestone"}
        </button>
      </div>
    </div>
  );
}
