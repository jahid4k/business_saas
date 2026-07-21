// src/components/hrm/warnings/WarningForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, WarningType, CreateWarningPayload } from "@/types/hrm";

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  warning_type_id: z.string().min(1, "Warning type is required"),
  title: z.string().min(1, "Title is required"),
  description: z.string().min(1, "Description is required"),
  incident_date: z.string().min(1, "Incident date is required"),
  witness_ids: z.array(z.string()).optional(),
});
type WarningFormValues = z.infer<typeof schema>;

interface WarningFormProps {
  employees: Employee[];
  warningTypes: WarningType[];
  onSave: (employeeId: string, payload: CreateWarningPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function WarningForm({
  employees,
  warningTypes,
  onSave,
}: WarningFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<WarningFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: "",
      warning_type_id: "",
      title: "",
      description: "",
      incident_date: "",
      witness_ids: [],
    },
  });

  const activeTypes = warningTypes.filter((t) => t.is_active);

  const onSubmit = async (values: WarningFormValues) => {
    setError(null);
    try {
      await onSave(values.employee_id, {
        warning_type_id: values.warning_type_id,
        title: values.title,
        description: values.description,
        incident_date: values.incident_date,
        witness_ids: values.witness_ids,
      });
      closeDrawer();
    } catch {
      setError("Failed to create warning. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="warning-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {activeTypes.length === 0 && (
          <div className="px-4 py-3 rounded-lg text-sm text-amber-400 bg-amber-500/10 border border-amber-500/20">
            No active warning types configured yet — go to Warning Types setup
            first.
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
            Warning type <span className="text-red-400">*</span>
          </label>
          <select {...register("warning_type_id")} className={inputCls}>
            <option value="">Select type</option>
            {activeTypes.map((t) => (
              <option
                key={t.id}
                value={t.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {t.name} (severity {t.severity_level})
              </option>
            ))}
          </select>
          {errors.warning_type_id && (
            <p className="text-xs text-red-400">
              {errors.warning_type_id.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            placeholder="Short summary"
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
            rows={4}
            placeholder="Full details of the incident"
            className={inputCls}
          />
          {errors.description && (
            <p className="text-xs text-red-400">{errors.description.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Incident date <span className="text-red-400">*</span>
          </label>
          <input
            {...register("incident_date")}
            type="date"
            className={inputCls}
          />
          {errors.incident_date && (
            <p className="text-xs text-red-400">
              {errors.incident_date.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Witnesses
          </label>
          <select
            multiple
            {...register("witness_ids")}
            className={`${inputCls} h-24`}
          >
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
          <p className="text-xs text-(--text-muted)">
            Optional. Hold Ctrl/Cmd to select multiple.
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
          form="warning-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create warning"}
        </button>
      </div>
    </div>
  );
}
