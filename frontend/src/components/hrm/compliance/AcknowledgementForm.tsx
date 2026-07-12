// src/components/hrm/compliance/AcknowledgementForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, CreateAcknowledgementPayload } from "@/types/hrm";

const ACK_TYPES = [
  { value: "policy", label: "Policy" },
  { value: "announcement", label: "Announcement" },
  { value: "warning", label: "Warning" },
  { value: "document", label: "Document" },
  { value: "calendar_event", label: "Calendar event" },
];

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  acknowledgeable_type: z.string().min(1, "Type is required"),
  acknowledgeable_id: z.string().min(1, "Reference ID is required"),
  entity_title: z.string().min(1, "Title is required"),
  signature_required: z.boolean().optional(),
  expires_at: z.string().optional(),
});
type AcknowledgementFormValues = z.infer<typeof schema>;

interface AcknowledgementFormProps {
  employees: Employee[];
  onSave: (payload: CreateAcknowledgementPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function AcknowledgementForm({
  employees,
  onSave,
}: AcknowledgementFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<AcknowledgementFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: "",
      acknowledgeable_type: "policy",
      acknowledgeable_id: "",
      entity_title: "",
      signature_required: false,
      expires_at: "",
    },
  });

  const onSubmit = async (values: AcknowledgementFormValues) => {
    setError(null);
    try {
      await onSave({
        employee_id: values.employee_id,
        acknowledgeable_type:
          values.acknowledgeable_type as CreateAcknowledgementPayload["acknowledgeable_type"],
        acknowledgeable_id: values.acknowledgeable_id,
        entity_title: values.entity_title,
        signature_required: values.signature_required,
        expires_at: values.expires_at || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to create acknowledgement request. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="acknowledgement-form"
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
          <select {...register("acknowledgeable_type")} className={inputCls}>
            {ACK_TYPES.map((t) => (
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
            {...register("entity_title")}
            placeholder="e.g. Code of Conduct v2"
            className={inputCls}
          />
          {errors.entity_title && (
            <p className="text-xs text-red-400">
              {errors.entity_title.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Reference ID <span className="text-red-400">*</span>
          </label>
          <input
            {...register("acknowledgeable_id")}
            placeholder="Free text — policy version, doc ID, etc."
            className={inputCls}
          />
          {errors.acknowledgeable_id && (
            <p className="text-xs text-red-400">
              {errors.acknowledgeable_id.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Expires on
          </label>
          <input {...register("expires_at")} type="date" className={inputCls} />
        </div>

        <div className="flex items-center gap-2.5 pt-1">
          <input
            type="checkbox"
            id="signature_required"
            {...register("signature_required")}
            className="w-4 h-4 accent-purple-600"
          />
          <label
            htmlFor="signature_required"
            className="text-sm text-[var(--text-secondary)]"
          >
            Require signature
          </label>
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
          form="acknowledgement-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Sending…" : "Send request"}
        </button>
      </div>
    </div>
  );
}
