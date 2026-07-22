// src/components/crm/deals/DealForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import { listPipelines, listStages, createPipeline } from "@/lib/crm/pipelines";
import { listContacts, getContact } from "@/lib/crm/contacts";
import { listCompanies, getCompany } from "@/lib/crm/companies";
import type { Deal, Pipeline, Stage } from "@/types/crm";
import { Select } from "@/components/ui/Select";
import { Combobox } from "@/components/ui/Combobox";
import { AsyncCombobox } from "@/components/ui/AsyncCombobox";

const CURRENCIES = [
  { value: "USD", label: "USD - US Dollar" },
  { value: "EUR", label: "EUR - Euro" },
  { value: "GBP", label: "GBP - British Pound" },
  { value: "CAD", label: "CAD - Canadian Dollar" },
  { value: "AUD", label: "AUD - Australian Dollar" },
  { value: "JPY", label: "JPY - Japanese Yen" },
  { value: "INR", label: "INR - Indian Rupee" },
  { value: "CHF", label: "CHF - Swiss Franc" },
  { value: "CNY", label: "CNY - Chinese Yuan" },
  { value: "SEK", label: "SEK - Swedish Krona" },
  { value: "NZD", label: "NZD - New Zealand Dollar" },
];

const schema = z.object({
  title: z.string().optional(),
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

  const [defaultContactName, setDefaultContactName] = useState("");
  const [defaultCompanyName, setDefaultCompanyName] = useState("");
  const [selectedContactName, setSelectedContactName] = useState("");
  const [selectedCompanyName, setSelectedCompanyName] = useState("");

  const [loadingStages, setLoadingStages] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!deal;

  const {
    register,
    handleSubmit,
    setValue,
    control,
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

  const watchedPipeline = useWatch({ control, name: "pipeline_id" });
  const watchedCurrency = useWatch({ control, name: "currency" });
  const watchedStage = useWatch({ control, name: "stage_id" });
  const watchedContact = useWatch({ control, name: "contact_id" });
  const watchedCompany = useWatch({ control, name: "company_id" });

  // Load initial names if editing
  useEffect(() => {
    if (isEdit && deal) {
      if (deal.contact_id) {
        getContact(orgId, deal.contact_id)
          .then((c) => {
            const name = `${c.first_name} ${c.last_name || ""}`.trim();
            setDefaultContactName(name);
            setSelectedContactName(name);
          })
          .catch(() => {});
      }
      if (deal.company_id) {
        getCompany(orgId, deal.company_id)
          .then((c) => {
            setDefaultCompanyName(c.name);
            setSelectedCompanyName(c.name);
          })
          .catch(() => {});
      }
    }
  }, [isEdit, deal, orgId]);

  // Load pipelines on mount
  useEffect(() => {
    listPipelines(orgId).then(setPipelines);
  }, [orgId]);

  const loadStages = async (pipelineId: string, autoSelect: boolean) => {
    if (!pipelineId) {
      setStages([]);
      return;
    }
    setLoadingStages(true);
    try {
      const s = await listStages(orgId, pipelineId);
      setStages(s);
      if (autoSelect && s.length > 0) {
        setValue("stage_id", s[0].id);
      }
    } finally {
      setLoadingStages(false);
    }
  };

  // Initial load of stages if a pipeline is already selected (e.g. edit mode or default)
  useEffect(() => {
    let active = true;
    if (watchedPipeline) {
      listStages(orgId, watchedPipeline).then((s) => {
        if (active) setStages(s);
      });
    }
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handlePipelineChange = (newPipelineId: string) => {
    setValue("pipeline_id", newPipelineId);
    if (!isEdit) {
      setValue("stage_id", "");
      loadStages(newPipelineId, true);
    } else {
      loadStages(newPipelineId, false);
    }
  };

  const onSubmit = async (values: DealFormValues) => {
    setError(null);

    // Auto-generate title if blank
    if (!values.title?.trim()) {
      values.title = "New Deal";
      if (selectedCompanyName)
        values.title = `Deal with ${selectedCompanyName}`;
      else if (selectedContactName)
        values.title = `Deal with ${selectedContactName}`;
    }

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
            Title
            <span className="ml-2 text-xs font-normal text-(--text-muted)">
              auto-generates if empty
            </span>
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
          <div className="col-span-1 space-y-1.5">
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
          <div className="space-y-1.5 col-span-2">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Currency
            </label>
            <input type="hidden" {...register("currency")} />
            <Combobox
              value={watchedCurrency || "USD"}
              onChange={(v) => setValue("currency", v)}
              options={CURRENCIES}
              placeholder="Select currency"
            />
          </div>
        </div>

        {/* Pipeline */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Pipeline <span className="text-red-400">*</span>
          </label>
          <input type="hidden" {...register("pipeline_id")} />
          <Select
            value={watchedPipeline || ""}
            onChange={handlePipelineChange}
            options={[
              { value: "", label: "Select pipeline" },
              ...pipelines.map((p) => ({
                value: p.id,
                label: p.name + (p.is_default ? " (default)" : ""),
              })),
            ]}
          />
          {errors.pipeline_id && (
            <p className="text-xs text-red-400">{errors.pipeline_id.message}</p>
          )}
          <button
            type="button"
            onClick={async () => {
              const name = window.prompt("Enter new pipeline name:");
              if (!name) return;
              try {
                const newPipe = await createPipeline(orgId, { name });
                setPipelines((prev) => [...prev, newPipe]);
                handlePipelineChange(newPipe.id);
              } catch {
                alert("Failed to create pipeline.");
              }
            }}
            className="text-xs text-purple-500 hover:text-purple-400 font-medium pt-0.5 inline-block"
          >
            + Create new pipeline
          </button>
        </div>

        {/* Stage */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Stage <span className="text-red-400">*</span>
          </label>
          <input type="hidden" {...register("stage_id")} />
          <Select
            value={watchedStage || ""}
            onChange={(v) => setValue("stage_id", v)}
            disabled={!watchedPipeline || loadingStages}
            options={[
              {
                value: "",
                label: loadingStages ? "Loading stages…" : "Select stage",
              },
              ...stages.map((s) => ({ value: s.id, label: s.name })),
            ]}
          />
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
          <input type="hidden" {...register("contact_id")} />
          <AsyncCombobox
            value={watchedContact || ""}
            defaultLabel={defaultContactName}
            onChange={(v, label) => {
              setValue("contact_id", v);
              setSelectedContactName(label);
            }}
            placeholder="Search contacts..."
            fetchOptions={async (q) => {
              const res = await listContacts(orgId, q);
              return res.contacts.map((c) => ({
                value: c.id,
                label: `${c.first_name} ${c.last_name ?? ""}`.trim(),
              }));
            }}
          />
        </div>

        {/* Company */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Company
            <span className="ml-2 text-xs font-normal text-(--text-muted)">
              optional
            </span>
          </label>
          <input type="hidden" {...register("company_id")} />
          <AsyncCombobox
            value={watchedCompany || ""}
            defaultLabel={defaultCompanyName}
            onChange={(v, label) => {
              setValue("company_id", v);
              setSelectedCompanyName(label);
            }}
            placeholder="Search companies..."
            fetchOptions={async (q) => {
              const res = await listCompanies(orgId, q);
              return res.companies.map((c) => ({
                value: c.id,
                label: c.name,
              }));
            }}
          />
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
