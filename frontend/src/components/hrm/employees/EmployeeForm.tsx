// src/components/hrm/employees/EmployeeForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import { listDepartments } from "@/lib/hrm/departments";
import { listPositions } from "@/lib/hrm/positions";
import { listEmployees } from "@/lib/hrm/employees";
import type { Employee, Department, Position } from "@/types/hrm";

const GENDERS = [
  { value: "male", label: "Male" },
  { value: "female", label: "Female" },
  { value: "other", label: "Other" },
  { value: "prefer_not_to_say", label: "Prefer not to say" },
];

const EMPLOYMENT_TYPES = [
  { value: "full_time", label: "Full-time" },
  { value: "part_time", label: "Part-time" },
  { value: "contractor", label: "Contractor" },
  { value: "intern", label: "Intern" },
];

const STATUSES = [
  { value: "active", label: "Active" },
  { value: "inactive", label: "Inactive" },
  { value: "on_leave", label: "On leave" },
  { value: "terminated", label: "Terminated" },
];

const schema = z.object({
  first_name: z
    .string()
    .min(1, "First name is required")
    .max(100, "Max 100 characters"),
  last_name: z.string().optional(),
  email: z.string().email("Invalid email").optional().or(z.literal("")),
  work_email: z.string().email("Invalid email").optional().or(z.literal("")),
  phone: z.string().optional(),
  work_phone: z.string().optional(),
  employee_number: z.string().optional(),
  date_of_birth: z.string().optional(),
  gender: z.string().optional(),
  hire_date: z.string().min(1, "Hire date is required"),
  employment_type: z.string().optional(),
  status: z.string().optional(),
  department_id: z.string().optional(),
  position_id: z.string().optional(),
  manager_id: z.string().optional(),
  address: z.string().optional(),
  city: z.string().optional(),
  country: z.string().optional(),
  notes: z.string().optional(),
});
type EmployeeFormValues = z.infer<typeof schema>;

interface EmployeeFormProps {
  orgId: string;
  employee?: Employee | null;
  onSave: (values: EmployeeFormValues) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

const sectionLabelCls =
  "text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] pt-2";

// Backend dates are RFC3339 ("2026-07-09T00:00:00Z"); <input type="date"> needs "YYYY-MM-DD"
function toDateInput(iso?: string) {
  if (!iso) return "";
  return iso.slice(0, 10);
}

export default function EmployeeForm({
  orgId,
  employee,
  onSave,
}: EmployeeFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const isEdit = !!employee;

  useEffect(() => {
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
    listPositions(orgId)
      .then((r) => setPositions(r.positions))
      .catch(() => {});
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
  }, [orgId]);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<EmployeeFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      first_name: employee?.first_name ?? "",
      last_name: employee?.last_name ?? "",
      email: employee?.email ?? "",
      work_email: employee?.work_email ?? "",
      phone: employee?.phone ?? "",
      work_phone: employee?.work_phone ?? "",
      employee_number: employee?.employee_number ?? "",
      date_of_birth: toDateInput(employee?.date_of_birth),
      gender: employee?.gender ?? "",
      hire_date: toDateInput(employee?.hire_date),
      employment_type: employee?.employment_type ?? "full_time",
      status: employee?.status ?? "active",
      department_id: employee?.department_id ?? "",
      position_id: employee?.position_id ?? "",
      manager_id: employee?.manager_id ?? "",
      address: employee?.address ?? "",
      city: employee?.city ?? "",
      country: employee?.country ?? "",
      notes: employee?.notes ?? "",
    },
  });

  const onSubmit = async (values: EmployeeFormValues) => {
    setError(null);
    const payload = Object.fromEntries(
      Object.entries(values).map(([k, v]) => [k, v === "" ? undefined : v]),
    ) as EmployeeFormValues;

    try {
      await onSave(payload);
      closeDrawer();
    } catch {
      setError("Failed to save employee. Please try again.");
    }
  };

  // Manager can be anyone except the employee themselves
  const managerOptions = employees.filter((e) => e.id !== employee?.id);

  return (
    <div className="flex flex-col h-full">
      <form
        id="employee-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <p className={sectionLabelCls}>Basic info</p>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              First name <span className="text-red-400">*</span>
            </label>
            <input
              {...register("first_name")}
              autoFocus
              placeholder="Ayesha"
              className={inputCls}
            />
            {errors.first_name && (
              <p className="text-xs text-red-400">
                {errors.first_name.message}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Last name
            </label>
            <input
              {...register("last_name")}
              placeholder="Rahman"
              className={inputCls}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Date of birth
            </label>
            <input
              {...register("date_of_birth")}
              type="date"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Gender
            </label>
            <select {...register("gender")} className={inputCls}>
              <option value="">Prefer not to say</option>
              {GENDERS.map((g) => (
                <option
                  key={g.value}
                  value={g.value}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {g.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <p className={sectionLabelCls}>Contact</p>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Personal email
            </label>
            <input
              {...register("email")}
              type="email"
              placeholder="ayesha@gmail.com"
              className={inputCls}
            />
            {errors.email && (
              <p className="text-xs text-red-400">{errors.email.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Work email
            </label>
            <input
              {...register("work_email")}
              type="email"
              placeholder="ayesha@company.com"
              className={inputCls}
            />
            {errors.work_email && (
              <p className="text-xs text-red-400">
                {errors.work_email.message}
              </p>
            )}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Phone
            </label>
            <input
              {...register("phone")}
              type="tel"
              placeholder="+880 1XXXXXXXXX"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Work phone
            </label>
            <input
              {...register("work_phone")}
              type="tel"
              placeholder="Extension or direct line"
              className={inputCls}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Address
          </label>
          <input
            {...register("address")}
            placeholder="Street address"
            className={inputCls}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              City
            </label>
            <input
              {...register("city")}
              placeholder="Dhaka"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Country
            </label>
            <input
              {...register("country")}
              placeholder="Bangladesh"
              className={inputCls}
            />
          </div>
        </div>

        <p className={sectionLabelCls}>Employment</p>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Hire date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("hire_date")}
              type="date"
              className={inputCls}
            />
            {errors.hire_date && (
              <p className="text-xs text-red-400">{errors.hire_date.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Employee number
            </label>
            <input
              {...register("employee_number")}
              placeholder="EMP-0042"
              className={inputCls}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Employment type
            </label>
            <select {...register("employment_type")} className={inputCls}>
              {EMPLOYMENT_TYPES.map((t) => (
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
          {isEdit && (
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-[var(--text-secondary)]">
                Status
              </label>
              <select {...register("status")} className={inputCls}>
                {STATUSES.map((s) => (
                  <option
                    key={s.value}
                    value={s.value}
                    style={{ background: "var(--bg-elevated)" }}
                  >
                    {s.label}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Department
            </label>
            <select {...register("department_id")} className={inputCls}>
              <option value="">Unassigned</option>
              {departments.map((d) => (
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
              Position
            </label>
            <select {...register("position_id")} className={inputCls}>
              <option value="">Unassigned</option>
              {positions.map((p) => (
                <option
                  key={p.id}
                  value={p.id}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {p.title}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Manager
          </label>
          <select {...register("manager_id")} className={inputCls}>
            <option value="">No manager</option>
            {managerOptions.map((e) => (
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
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Notes
          </label>
          <textarea
            {...register("notes")}
            rows={3}
            placeholder="Internal notes"
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
          form="employee-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create employee"}
        </button>
      </div>
    </div>
  );
}
