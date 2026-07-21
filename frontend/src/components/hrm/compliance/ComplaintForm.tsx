// src/components/hrm/compliance/ComplaintForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, CreateComplaintPayload } from "@/types/hrm";

const COMPLAINT_TYPES = [
  { value: "harassment", label: "Harassment" },
  { value: "discrimination", label: "Discrimination" },
  { value: "workplace_safety", label: "Workplace safety" },
  { value: "policy_violation", label: "Policy violation" },
  { value: "manager_conduct", label: "Manager conduct" },
  { value: "wage_dispute", label: "Wage dispute" },
  { value: "retaliation", label: "Retaliation" },
  { value: "general", label: "General" },
];

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  complaint_type: z.string().min(1, "Type is required"),
  title: z.string().min(1, "Title is required"),
  description: z.string().min(1, "Description is required"),
  incident_date: z.string().optional(),
  against_employee_id: z.string().optional(),
  against_details: z.string().optional(),
  is_anonymous: z.boolean().optional(),
});
type ComplaintFormValues = z.infer<typeof schema>;

interface ComplaintFormProps {
  employees: Employee[];
  defaultEmployeeId?: string;
  onSave: (
    employeeId: string,
    payload: CreateComplaintPayload,
  ) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function ComplaintForm({
  employees,
  defaultEmployeeId,
  onSave,
}: ComplaintFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ComplaintFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? "",
      complaint_type: "",
      title: "",
      description: "",
      incident_date: "",
      against_employee_id: "",
      against_details: "",
      is_anonymous: false,
    },
  });

  const onSubmit = async (values: ComplaintFormValues) => {
    setError(null);
    try {
      await onSave(values.employee_id, {
        complaint_type:
          values.complaint_type as CreateComplaintPayload["complaint_type"],
        title: values.title,
        description: values.description,
        incident_date: values.incident_date || undefined,
        against_employee_id: values.against_employee_id || undefined,
        against_details: values.against_details || undefined,
        is_anonymous: values.is_anonymous,
      });
      closeDrawer();
    } catch {
      setError("Failed to submit complaint. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="complaint-form"
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
            Filed by <span className="text-red-400">*</span>
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
            Complaint type <span className="text-red-400">*</span>
          </label>
          <select {...register("complaint_type")} className={inputCls}>
            <option value="">Select type</option>
            {COMPLAINT_TYPES.map((t) => (
              <option
                key={t.value}
                value={t.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {t.label}
              </option>
            ))}
          </select>
          {errors.complaint_type && (
            <p className="text-xs text-red-400">
              {errors.complaint_type.message}
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
            placeholder="Full details"
            className={inputCls}
          />
          {errors.description && (
            <p className="text-xs text-red-400">{errors.description.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Incident date
          </label>
          <input
            {...register("incident_date")}
            type="date"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Against (employee)
          </label>
          <select {...register("against_employee_id")} className={inputCls}>
            <option value="">Not applicable / not sure</option>
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
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Against (other details)
          </label>
          <input
            {...register("against_details")}
            placeholder="If not an employee in the system"
            className={inputCls}
          />
        </div>

        <div className="flex items-center gap-2.5 pt-1">
          <input
            type="checkbox"
            id="is_anonymous"
            {...register("is_anonymous")}
            className="w-4 h-4 accent-purple-600"
          />
          <label
            htmlFor="is_anonymous"
            className="text-sm text-(--text-secondary)"
          >
            Mark as anonymous
          </label>
        </div>
        <p className="text-xs text-(--text-muted) -mt-2">
          Note: this flags the complaint for sensitive handling, but the
          filer&apos;s identity is still stored and visible to HR — it is not
          fully anonymized at the system level.
        </p>
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
          form="complaint-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Submitting…" : "Submit complaint"}
        </button>
      </div>
    </div>
  );
}
