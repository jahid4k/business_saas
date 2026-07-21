// src/components/hrm/salary/SalaryStructureForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { CreateSalaryStructurePayload } from "@/types/hrm";

const schema = z.object({
  name: z.string().min(1, "Name is required").max(150, "Max 150 characters"),
  description: z.string().optional(),
  grade_label: z.string().optional(),
});
type SalaryStructureFormValues = z.infer<typeof schema>;

interface SalaryStructureFormProps {
  onSave: (payload: CreateSalaryStructurePayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function SalaryStructureForm({
  onSave,
}: SalaryStructureFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<SalaryStructureFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", description: "", grade_label: "" },
  });

  const onSubmit = async (values: SalaryStructureFormValues) => {
    setError(null);
    try {
      await onSave({
        name: values.name,
        description: values.description || undefined,
        grade_label: values.grade_label || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to create structure. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="salary-structure-form"
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
            placeholder="e.g. Senior Engineer Grade"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Grade label
          </label>
          <input
            {...register("grade_label")}
            placeholder="e.g. L4, Senior"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Description
          </label>
          <textarea
            {...register("description")}
            rows={3}
            placeholder="Optional"
            className={inputCls}
          />
        </div>

        <p className="text-xs text-(--text-muted)">
          You can add salary components to this structure after creating it.
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
          form="salary-structure-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create structure"}
        </button>
      </div>
    </div>
  );
}
