// src/components/hrm/compliance/EmployeeDocumentForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Employee, CreateEmployeeDocumentPayload } from "@/types/hrm";

const schema = z.object({
  employee_id: z.string().min(1, "Employee is required"),
  title: z.string().min(1, "Title is required"),
  document_type: z.string().min(1, "Document type is required"),
  file_url: z.string().min(1, "File URL is required"),
  file_name: z.string().min(1, "File name is required"),
  mime_type: z.string().optional(),
  expiry_date: z.string().optional(),
});
type EmployeeDocumentFormValues = z.infer<typeof schema>;

interface EmployeeDocumentFormProps {
  employees: Employee[];
  defaultEmployeeId?: string;
  onSave: (
    employeeId: string,
    payload: CreateEmployeeDocumentPayload,
  ) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function EmployeeDocumentForm({
  employees,
  defaultEmployeeId,
  onSave,
}: EmployeeDocumentFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<EmployeeDocumentFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? "",
      title: "",
      document_type: "",
      file_url: "",
      file_name: "",
      mime_type: "application/pdf",
      expiry_date: "",
    },
  });

  const onSubmit = async (values: EmployeeDocumentFormValues) => {
    setError(null);
    try {
      await onSave(values.employee_id, {
        title: values.title,
        document_type: values.document_type,
        file_url: values.file_url,
        file_name: values.file_name,
        mime_type: values.mime_type || "application/pdf",
        expiry_date: values.expiry_date || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to create document. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="employee-document-form"
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
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            placeholder="e.g. Employment Contract"
            className={inputCls}
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Document type <span className="text-red-400">*</span>
          </label>
          <input
            {...register("document_type")}
            placeholder="e.g. contract, id_card, certificate"
            className={inputCls}
          />
          {errors.document_type && (
            <p className="text-xs text-red-400">
              {errors.document_type.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            File URL <span className="text-red-400">*</span>
          </label>
          <input
            {...register("file_url")}
            placeholder="https://…"
            className={inputCls}
          />
          {errors.file_url && (
            <p className="text-xs text-red-400">{errors.file_url.message}</p>
          )}
          <p className="text-xs text-(--text-muted)">
            Paste a link to an already-uploaded file — direct file upload
            isn&apos;t wired here yet.
          </p>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            File name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("file_name")}
            placeholder="contract.pdf"
            className={inputCls}
          />
          {errors.file_name && (
            <p className="text-xs text-red-400">{errors.file_name.message}</p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              MIME type
            </label>
            <input
              {...register("mime_type")}
              placeholder="application/pdf"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Expiry date
            </label>
            <input
              {...register("expiry_date")}
              type="date"
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
          form="employee-document-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create document"}
        </button>
      </div>
    </div>
  );
}
