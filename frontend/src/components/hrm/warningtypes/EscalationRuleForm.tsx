// src/components/hrm/warningtypes/EscalationRuleForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { WarningType, CreateEscalationRulePayload } from "@/types/hrm";

const ROLES = ["owner", "admin", "manager"];

const ACTIONS = [
  { value: "notify_hr", label: "Notify HR" },
  { value: "notify_management", label: "Notify management" },
  { value: "flag_termination_review", label: "Flag for termination review" },
];

const schema = z.object({
  trigger_warning_type_id: z.string().min(1, "Warning type is required"),
  trigger_count: z.coerce.number().int().min(1, "Must be at least 1"),
  within_days: z.coerce.number().int().min(0).optional(),
  action: z.string().min(1, "Action is required"),
  notification_roles: z.array(z.string()).optional(),
});
type EscalationRuleFormInput = z.input<typeof schema>;
type EscalationRuleFormValues = z.infer<typeof schema>;

interface EscalationRuleFormProps {
  warningTypes: WarningType[];
  onSave: (payload: CreateEscalationRulePayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function EscalationRuleForm({
  warningTypes,
  onSave,
}: EscalationRuleFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<EscalationRuleFormInput, undefined, EscalationRuleFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      trigger_warning_type_id: "",
      trigger_count: 3,
      within_days: 0,
      action: "notify_hr",
      notification_roles: ["admin"],
    },
  });

  const onSubmit = async (values: EscalationRuleFormValues) => {
    setError(null);
    try {
      await onSave({
        trigger_warning_type_id: values.trigger_warning_type_id,
        trigger_count: values.trigger_count,
        within_days: values.within_days,
        action: values.action as CreateEscalationRulePayload["action"],
        notification_roles: values.notification_roles,
      });
      closeDrawer();
    } catch {
      setError("Failed to create escalation rule. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="escalation-rule-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <p className="text-xs text-(--text-muted)">
          By design, this only alerts HR when the threshold is reached — it
          never creates a warning automatically. HR reviews and decides next
          steps manually.
        </p>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Warning type <span className="text-red-400">*</span>
          </label>
          <select
            {...register("trigger_warning_type_id")}
            autoFocus
            className={inputCls}
          >
            <option value="">Select warning type</option>
            {warningTypes.map((t) => (
              <option
                key={t.id}
                value={t.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {t.name}
              </option>
            ))}
          </select>
          {errors.trigger_warning_type_id && (
            <p className="text-xs text-red-400">
              {errors.trigger_warning_type_id.message}
            </p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Trigger count <span className="text-red-400">*</span>
            </label>
            <input
              {...register("trigger_count")}
              type="number"
              min={1}
              className={inputCls}
            />
            {errors.trigger_count && (
              <p className="text-xs text-red-400">
                {errors.trigger_count.message}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Within (days)
            </label>
            <input
              {...register("within_days")}
              type="number"
              min={0}
              placeholder="0 = all-time"
              className={inputCls}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Action
          </label>
          <select {...register("action")} className={inputCls}>
            {ACTIONS.map((a) => (
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

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Notify roles
          </label>
          <select
            multiple
            {...register("notification_roles")}
            className={`${inputCls} h-20`}
          >
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
          form="escalation-rule-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create rule"}
        </button>
      </div>
    </div>
  );
}
