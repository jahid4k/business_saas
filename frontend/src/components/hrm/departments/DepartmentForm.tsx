// src/components/hrm/departments/DepartmentForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import { listDepartments } from "@/lib/hrm/departments";
import { listEmployees } from "@/lib/hrm/employees";
import type { Department, Employee } from "@/types/hrm";

const schema = z.object({
  name: z.string().min(1, "Name is required").max(150, "Max 150 characters"),
  description: z.string().optional(),
  parent_department_id: z.string().optional(),
  head_employee_id: z.string().optional(),
  is_active: z.boolean().optional(),
});
type DepartmentFormValues = z.infer<typeof schema>;

interface DepartmentFormProps {
  orgId: string;
  department?: Department | null;
  onSave: (values: DepartmentFormValues) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function DepartmentForm({
  orgId,
  department,
  onSave,
}: DepartmentFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const isEdit = !!department;

  useEffect(() => {
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
  }, [orgId]);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<DepartmentFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: department?.name ?? "",
      description: department?.description ?? "",
      parent_department_id: department?.parent_department_id ?? "",
      head_employee_id: department?.head_employee_id ?? "",
      is_active: department?.is_active ?? true,
    },
  });

  const onSubmit = async (values: DepartmentFormValues) => {
    setError(null);
    const payload = Object.fromEntries(
      Object.entries(values).map(([k, v]) => [k, v === "" ? undefined : v]),
    ) as DepartmentFormValues;

    try {
      await onSave(payload);
      closeDrawer();
    } catch {
      setError("Failed to save department. Please try again.");
    }
  };

  // A department can't be its own parent
  const parentOptions = departments.filter((d) => d.id !== department?.id);

  return (
    <div className="flex flex-col h-full">
      <form
        id="department-form"
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
            placeholder="Engineering"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Description
          </label>
          <textarea
            {...register("description")}
            rows={3}
            placeholder="What this department is responsible for"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Parent department
          </label>
          <select {...register("parent_department_id")} className={inputCls}>
            <option value="">None (top-level)</option>
            {parentOptions.map((d) => (
              <option
                key={d.id}
                value={d.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {d.name}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Department head
          </label>
          <select {...register("head_employee_id")} className={inputCls}>
            <option value="">Unassigned</option>
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

        {isEdit && (
          <div className="flex items-center gap-2.5 pt-1">
            <input
              type="checkbox"
              id="is_active"
              {...register("is_active")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="is_active"
              className="text-sm text-[var(--text-secondary)]"
            >
              Active
            </label>
          </div>
        )}
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
          form="department-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create department"}
        </button>
      </div>
    </div>
  );
}
