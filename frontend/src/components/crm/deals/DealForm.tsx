// src/components/crm/deals/DealForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import { listPipelines, listStages } from "@/lib/crm/pipelines";
import { listContacts } from "@/lib/crm/contacts";
import { listCompanies } from "@/lib/crm/companies";
import type { Deal, Pipeline, Stage, Contact, Company } from "@/types/crm";

const schema = z.object({
  title: z.string().min(1, "Title is required"),
  value: z.coerce.number().min(0, "Value must be 0 or more"),
  currency: z.string().default("USD"),
  pipeline_id: z.string().min(1, "Pipeline is required"),
  stage_id: z.string().min(1, "Stage is required"),
  contact_id: z.string().optional(),
  company_id: z.string().optional(),
  close_date: z.string().optional(),
});
type DealFormValues = z.infer<typeof schema>;

interface DealFormProps {
  deal?: Deal | null;
  orgId: string;
  defaultPipelineId?: string;
  defaultStageId?: string;
  onSave: (values: DealFormValues) => Promise<void>;
}

const cls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function DealForm({
  deal,
  orgId,
  defaultPipelineId,
  defaultStageId,
  onSave,
}: DealFormProps) {
  const { closeDrawer } = useDrawer();

  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [stages, setStages] = useState<Stage[]>([]);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [loadingStages, setLoadingStages] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!deal;

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<z.input<typeof schema>, unknown, DealFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: deal?.title ?? "",
      value: deal?.value ?? 0,
      currency: deal?.currency ?? "USD",
      pipeline_id: deal?.pipeline_id ?? defaultPipelineId ?? "",
      stage_id: deal?.stage_id ?? defaultStageId ?? "",
      contact_id: deal?.contact_id ?? "",
      company_id: deal?.company_id ?? "",
      close_date: deal?.close_date?.split("T")[0] ?? "",
    },
  });

  const watchedPipeline = watch("pipeline_id");

  // Load pipelines, contacts, companies on mount
  useEffect(() => {
    Promise.all([
      listPipelines(orgId),
      listContacts(orgId).then((r) => r.contacts),
      listCompanies(orgId).then((r) => r.companies),
    ]).then(([pipes, cons, comps]) => {
      setPipelines(pipes);
      setContacts(cons);
      setCompanies(comps);
    });
  }, [orgId]);

  // Reload stages when pipeline changes
  useEffect(() => {
    if (!watchedPipeline) {
      setStages([]);
      return;
    }
    setLoadingStages(true);
    if (!isEdit) setValue("stage_id", "");
    listStages(orgId, watchedPipeline)
      .then((s) => {
        setStages(s);
        // Auto-select first stage on pipeline change (create mode)
        if (!isEdit && s.length > 0) setValue("stage_id", s[0].id);
      })
      .finally(() => setLoadingStages(false));
  }, [watchedPipeline, orgId, isEdit, setValue]);

  const onSubmit = async (values: DealFormValues) => {
    setError(null);
    const clean = Object.fromEntries(
      Object.entries(values).map(([k, v]) => [k, v === "" ? undefined : v]),
    ) as DealFormValues;
    try {
      await onSave(clean);
      closeDrawer();
    } catch {
      setError("Failed to save deal. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="deal-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {/* Title */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            autoFocus
            placeholder="Acme Corp — Enterprise Plan"
            className={cls}
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        {/* Value + Currency */}
        <div className="grid grid-cols-3 gap-3">
          <div className="col-span-2 space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Value <span className="text-red-400">*</span>
            </label>
            <div className="relative">
              <span className="absolute left-3.5 top-1/2 -translate-y-1/2 text-sm text-(--text-muted)">
                $
              </span>
              <input
                {...register("value")}
                type="number"
                min="0"
                step="0.01"
                placeholder="0"
                className={`${cls} pl-7`}
              />
            </div>
            {errors.value && (
              <p className="text-xs text-red-400">{errors.value.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Currency
            </label>
            <input
              {...register("currency")}
              placeholder="USD"
              className={cls}
            />
          </div>
        </div>

        {/* Pipeline */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Pipeline <span className="text-red-400">*</span>
          </label>
          <select {...register("pipeline_id")} className={cls}>
            <option value="">Select pipeline</option>
            {pipelines.map((p) => (
              <option
                key={p.id}
                value={p.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {p.name}
                {p.is_default ? " (default)" : ""}
              </option>
            ))}
          </select>
          {errors.pipeline_id && (
            <p className="text-xs text-red-400">{errors.pipeline_id.message}</p>
          )}
        </div>

        {/* Stage */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Stage <span className="text-red-400">*</span>
          </label>
          <select
            {...register("stage_id")}
            disabled={!watchedPipeline || loadingStages}
            className={`${cls} disabled:opacity-50`}
          >
            <option value="">
              {loadingStages ? "Loading stages…" : "Select stage"}
            </option>
            {stages.map((s) => (
              <option
                key={s.id}
                value={s.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {s.name}
              </option>
            ))}
          </select>
          {errors.stage_id && (
            <p className="text-xs text-red-400">{errors.stage_id.message}</p>
          )}
        </div>

        {/* Contact */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Contact
            <span className="ml-2 text-xs font-normal text-(--text-muted)">
              optional
            </span>
          </label>
          <select {...register("contact_id")} className={cls}>
            <option value="">No contact</option>
            {contacts.map((c) => (
              <option
                key={c.id}
                value={c.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {c.first_name} {c.last_name ?? ""}
              </option>
            ))}
          </select>
        </div>

        {/* Company */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Company
            <span className="ml-2 text-xs font-normal text-(--text-muted)">
              optional
            </span>
          </label>
          <select {...register("company_id")} className={cls}>
            <option value="">No company</option>
            {companies.map((c) => (
              <option
                key={c.id}
                value={c.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {c.name}
              </option>
            ))}
          </select>
        </div>

        {/* Close date */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Close date
            <span className="ml-2 text-xs font-normal text-(--text-muted)">
              optional
            </span>
          </label>
          <input {...register("close_date")} type="date" className={cls} />
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
          form="deal-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create deal"}
        </button>
      </div>
    </div>
  );
}
