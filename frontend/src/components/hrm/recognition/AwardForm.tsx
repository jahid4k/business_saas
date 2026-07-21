// src/components/hrm/recognition/AwardForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, CreateAwardPayload } from "@/types/hrm";

const AWARD_TYPES = [
  { value: "spot_recognition", label: "Spot recognition" },
  { value: "performance", label: "Performance" },
  { value: "tenure", label: "Tenure" },
  { value: "team", label: "Team" },
  { value: "innovation", label: "Innovation" },
  { value: "customer_service", label: "Customer service" },
  { value: "custom", label: "Custom" },
];

const optionalNumber = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().min(0).optional(),
);

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  award_type: z.string().min(1, "Type is required"),
  title: z.string().min(1, "Title is required"),
  description: z.string().min(1, "Description is required"),
  points: optionalNumber,
  monetary_value: optionalNumber,
  currency: z.string().optional(),
  award_date: z.string().optional(),
});
type AwardFormInput = z.input<typeof schema>;
type AwardFormValues = z.infer<typeof schema>;

interface AwardFormProps {
  employees: Employee[];
  onSave: (payload: CreateAwardPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function AwardForm({ employees, onSave }: AwardFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<AwardFormInput, undefined, AwardFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: "",
      award_type: "spot_recognition",
      title: "",
      description: "",
      points: "",
      monetary_value: "",
      currency: "BDT",
      award_date: "",
    },
  });

  const onSubmit = async (values: AwardFormValues) => {
    setError(null);
    try {
      await onSave({
        employee_id: values.employee_id,
        award_type: values.award_type as CreateAwardPayload["award_type"],
        title: values.title,
        description: values.description,
        points: values.points,
        monetary_value: values.monetary_value,
        currency: values.currency || undefined,
        award_date: values.award_date || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to create award. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="award-form"
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
            Award type
          </label>
          <select {...register("award_type")} className={inputCls}>
            {AWARD_TYPES.map((t) => (
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
          <label className="block text-sm font-medium text-(--text-secondary)">
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            placeholder="e.g. Employee of the Month"
            className={inputCls}
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Description <span className="text-red-400">*</span>
          </label>
          <textarea
            {...register("description")}
            rows={3}
            placeholder="Why this award?"
            className={inputCls}
          />
          {errors.description && (
            <p className="text-xs text-red-400">{errors.description.message}</p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Points
            </label>
            <input
              {...register("points")}
              type="number"
              min={0}
              step={1}
              placeholder="Optional"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Award date
            </label>
            <input
              {...register("award_date")}
              type="date"
              className={inputCls}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Monetary value
            </label>
            <input
              {...register("monetary_value")}
              type="number"
              min={0}
              step="0.01"
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
          form="award-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create award"}
        </button>
      </div>
    </div>
  );
}
