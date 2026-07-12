// src/components/hrm/approvals/ApprovalTemplateForm.tsx
"use client";

import { useState } from "react";
import { useForm, useFieldArray } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Trash2 } from "lucide-react";
import { useDrawer } from "@/contexts/DrawerContext";
// import line-এ যোগ করো:
import type {
  Employee,
  CreateTemplatePayload,
  ApproverType,
  SLABreachAction,
} from "@/types/hrm";

const ACTION_TYPES = [
  { value: "leave", label: "Leave" },
  { value: "resignation", label: "Resignation" },
  { value: "promotion", label: "Promotion" },
  { value: "transfer", label: "Transfer" },
  { value: "warning", label: "Warning" },
  { value: "document", label: "Document" },
  { value: "termination", label: "Termination" },
  { value: "attendance_regularization", label: "Attendance regularization" },
  { value: "custom", label: "Custom" },
];

const APPROVER_TYPES = [
  { value: "reporting_manager", label: "Reporting manager" },
  { value: "dept_head", label: "Department head" },
  { value: "role", label: "Specific role" },
  { value: "specific_user", label: "Specific person" },
];

const ROLES = ["owner", "admin", "manager"];

const SLA_ACTIONS = [
  { value: "escalate_next", label: "Escalate to next level" },
  { value: "auto_approve", label: "Auto-approve" },
  { value: "auto_reject", label: "Auto-reject" },
];

const levelSchema = z.object({
  level: z.number(),
  approver_type: z.string().min(1),
  approver_role: z.string().optional(),
  approver_user_id: z.string().optional(),
  sla_hours: z.coerce.number().int().min(1, "Must be at least 1 hour"),
  on_sla_breach: z.string().min(1),
});

const schema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
  action_type: z.string().min(1, "Action type is required"),
  is_default: z.boolean().optional(),
  levels: z.array(levelSchema).min(1, "At least one level is required"),
});
type TemplateFormInput = z.input<typeof schema>;
type TemplateFormValues = z.infer<typeof schema>;

interface ApprovalTemplateFormProps {
  employees: Employee[];
  onSave: (payload: CreateTemplatePayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function ApprovalTemplateForm({
  employees,
  onSave,
}: ApprovalTemplateFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    control,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<TemplateFormInput, undefined, TemplateFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      description: "",
      action_type: "promotion",
      is_default: true,
      levels: [
        {
          level: 1,
          approver_type: "reporting_manager",
          sla_hours: 48,
          on_sla_breach: "escalate_next",
        },
      ],
    },
  });

  const { fields, append, remove } = useFieldArray({ control, name: "levels" });

  const onSubmit = async (values: TemplateFormValues) => {
    setError(null);
    try {
      await onSave({
        name: values.name,
        description: values.description || undefined,
        action_type: values.action_type as CreateTemplatePayload["action_type"],
        is_default: values.is_default,
        levels: values.levels.map((l, i) => ({
          ...l,
          level: i + 1,
          approver_type: l.approver_type as ApproverType,
          on_sla_breach: l.on_sla_breach as SLABreachAction,
        })),
      });
      closeDrawer();
    } catch {
      setError("Failed to create approval template. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="approval-template-form"
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
            placeholder="e.g. Standard Promotion Approval"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Applies to
          </label>
          <select {...register("action_type")} className={inputCls}>
            {ACTION_TYPES.map((t) => (
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
            Description
          </label>
          <input
            {...register("description")}
            placeholder="Optional"
            className={inputCls}
          />
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
            className="text-sm text-[var(--text-secondary)]"
          >
            Default template for this action type
          </label>
        </div>

        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
              Approval levels (in order)
            </p>
            <button
              type="button"
              onClick={() =>
                append({
                  level: fields.length + 1,
                  approver_type: "reporting_manager",
                  sla_hours: 48,
                  on_sla_breach: "escalate_next",
                })
              }
              className="flex items-center gap-1 text-xs font-medium text-purple-400 hover:text-purple-300"
            >
              <Plus size={13} />
              Add level
            </button>
          </div>

          {fields.map((field, index) => {
            const approverType = watch(`levels.${index}.approver_type`);
            return (
              <div
                key={field.id}
                className="p-3 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] space-y-2"
              >
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-[var(--text-muted)]">
                    Level {index + 1}
                  </span>
                  {fields.length > 1 && (
                    <button
                      type="button"
                      onClick={() => remove(index)}
                      className="text-red-400 hover:text-red-300"
                    >
                      <Trash2 size={13} />
                    </button>
                  )}
                </div>
                <select
                  {...register(`levels.${index}.approver_type` as const)}
                  className={inputCls}
                >
                  {APPROVER_TYPES.map((t) => (
                    <option
                      key={t.value}
                      value={t.value}
                      style={{ background: "var(--bg-elevated)" }}
                    >
                      {t.label}
                    </option>
                  ))}
                </select>
                {approverType === "role" && (
                  <select
                    {...register(`levels.${index}.approver_role` as const)}
                    className={inputCls}
                  >
                    <option value="">Select role</option>
                    {ROLES.map((r) => (
                      <option
                        key={r}
                        value={r}
                        style={{ background: "var(--bg-elevated)" }}
                      >
                        {r}
                      </option>
                    ))}
                  </select>
                )}
                {approverType === "specific_user" && (
                  <select
                    {...register(`levels.${index}.approver_user_id` as const)}
                    className={inputCls}
                  >
                    <option value="">Select person</option>
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
                )}
                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1">
                    <label className="block text-xs text-[var(--text-muted)]">
                      SLA (hours)
                    </label>
                    <input
                      {...register(`levels.${index}.sla_hours` as const)}
                      type="number"
                      min={1}
                      className={inputCls}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="block text-xs text-[var(--text-muted)]">
                      On SLA breach
                    </label>
                    <select
                      {...register(`levels.${index}.on_sla_breach` as const)}
                      className={inputCls}
                    >
                      {SLA_ACTIONS.map((a) => (
                        <option
                          key={a.value}
                          value={a.value}
                          style={{ background: "var(--bg-elevated)" }}
                        >
                          {a.label}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
              </div>
            );
          })}
          {errors.levels?.message && (
            <p className="text-xs text-red-400">{errors.levels.message}</p>
          )}
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
          form="approval-template-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create template"}
        </button>
      </div>
    </div>
  );
}
