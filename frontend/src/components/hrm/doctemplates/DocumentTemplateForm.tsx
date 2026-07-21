// src/components/hrm/doctemplates/DocumentTemplateForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type {
  DocumentTemplate,
  CreateDocumentTemplatePayload,
} from "@/types/hrm";

const DOC_TYPES = [
  { value: "offer_letter", label: "Offer letter" },
  { value: "contract", label: "Contract" },
  { value: "warning_letter", label: "Warning letter" },
  { value: "promotion_letter", label: "Promotion letter" },
  { value: "transfer_letter", label: "Transfer letter" },
  { value: "termination_letter", label: "Termination letter" },
  { value: "resignation_acceptance", label: "Resignation acceptance" },
  { value: "experience_letter", label: "Experience letter" },
  { value: "nda", label: "NDA" },
  { value: "policy", label: "Policy" },
  { value: "custom", label: "Custom" },
];

const schema = z.object({
  name: z.string().min(1, "Name is required").max(100, "Max 100 characters"),
  document_type: z.string().min(1, "Type is required"),
  description: z.string().optional(),
  body_markdown: z.string().min(1, "Template body is required"),
  available_variables: z.string().optional(),
  requires_acknowledgement: z.boolean().optional(),
  is_active: z.boolean().optional(),
});
type DocumentTemplateFormValues = z.infer<typeof schema>;

interface DocumentTemplateFormProps {
  template?: DocumentTemplate | null;
  onSave: (payload: CreateDocumentTemplatePayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function DocumentTemplateForm({
  template,
  onSave,
}: DocumentTemplateFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!template;

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<DocumentTemplateFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: template?.name ?? "",
      document_type: template?.document_type ?? "custom",
      description: template?.description ?? "",
      body_markdown: template?.body_markdown ?? "",
      available_variables: template?.available_variables?.join(", ") ?? "",
      requires_acknowledgement: template?.requires_acknowledgement ?? false,
      is_active: template?.is_active ?? true,
    },
  });

  const onSubmit = async (values: DocumentTemplateFormValues) => {
    setError(null);
    const variables = (values.available_variables ?? "")
      .split(",")
      .map((v) => v.trim())
      .filter(Boolean);
    try {
      await onSave({
        name: values.name,
        document_type:
          values.document_type as CreateDocumentTemplatePayload["document_type"],
        description: values.description || undefined,
        body_markdown: values.body_markdown,
        available_variables: variables,
        requires_acknowledgement: values.requires_acknowledgement,
      });
      closeDrawer();
    } catch {
      setError("Failed to save template. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="doc-template-form"
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
            Name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("name")}
            autoFocus
            placeholder="e.g. Standard Offer Letter"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Document type
          </label>
          <select {...register("document_type")} className={inputCls}>
            {DOC_TYPES.map((t) => (
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
            Description
          </label>
          <input
            {...register("description")}
            placeholder="Optional"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Available variables
          </label>
          <input
            {...register("available_variables")}
            placeholder="e.g. employee_name, position, start_date"
            className={inputCls}
          />
          <p className="text-xs text-(--text-muted)">
            Comma-separated. Use as {"{{employee_name}}"} in the body below.
          </p>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Template body (Markdown) <span className="text-red-400">*</span>
          </label>
          <textarea
            {...register("body_markdown")}
            rows={12}
            placeholder={
              "Dear {{employee_name}},\n\nWe are pleased to offer you the position of {{position}}..."
            }
            className={`${inputCls} font-mono`}
          />
          {errors.body_markdown && (
            <p className="text-xs text-red-400">
              {errors.body_markdown.message}
            </p>
          )}
        </div>

        <div className="flex items-center gap-6">
          <div className="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="requires_acknowledgement"
              {...register("requires_acknowledgement")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="requires_acknowledgement"
              className="text-sm text-(--text-secondary)"
            >
              Requires acknowledgement
            </label>
          </div>
          {isEdit && (
            <div className="flex items-center gap-2.5">
              <input
                type="checkbox"
                id="is_active"
                {...register("is_active")}
                className="w-4 h-4 accent-purple-600"
              />
              <label
                htmlFor="is_active"
                className="text-sm text-(--text-secondary)"
              >
                Active
              </label>
            </div>
          )}
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
          form="doc-template-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create template"}
        </button>
      </div>
    </div>
  );
}
