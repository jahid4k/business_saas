// src/components/crm/companies/CompanyForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Company } from "@/types/crm";
import { Select } from "@/components/ui/Select";

const INDUSTRIES = [
  "Technology",
  "Healthcare",
  "Finance",
  "Retail",
  "Manufacturing",
  "Education",
  "Real Estate",
  "Consulting",
  "Media",
  "Other",
];

const schema = z.object({
  name: z.string().min(1, "Company name is required"),
  domain: z.string().optional(),
  industry: z.string().optional(),
  website: z.string().url("Enter a valid URL").optional().or(z.literal("")),
  phone: z.string().optional(),
  address: z.string().optional(),
  country: z.string().optional(),
});
type CompanyFormValues = z.infer<typeof schema>;

interface CompanyFormProps {
  company?: Company | null;
  onSave: (values: CompanyFormValues) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function CompanyForm({ company, onSave }: CompanyFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!company;

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<CompanyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: company?.name ?? "",
      domain: company?.domain ?? "",
      industry: company?.industry ?? "",
      website: company?.website ?? "",
      phone: company?.phone ?? "",
      address: company?.address ?? "",
      country: company?.country ?? "",
    },
  });

  const onSubmit = async (values: CompanyFormValues) => {
    setError(null);
    const payload = Object.fromEntries(
      Object.entries(values).map(([k, v]) => [k, v === "" ? undefined : v]),
    ) as CompanyFormValues;
    try {
      await onSave(payload);
      closeDrawer();
    } catch {
      setError("Failed to save company. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="company-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {/* Name */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Company name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("name")}
            autoFocus
            placeholder="Acme Corporation"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        {/* Domain + Industry */}
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Domain
            </label>
            <input
              {...register("domain")}
              placeholder="acme.com"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Industry
            </label>
            <input type="hidden" {...register("industry")} />
            <Select
              value={watch("industry") || ""}
              onChange={(v) => setValue("industry", v)}
              options={[
                { value: "", label: "Select industry" },
                ...INDUSTRIES.map((i) => ({ value: i, label: i })),
              ]}
            />
          </div>
        </div>

        {/* Website */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Website
          </label>
          <input
            {...register("website")}
            type="url"
            placeholder="https://acme.com"
            className={inputCls}
          />
          {errors.website && (
            <p className="text-xs text-red-400">{errors.website.message}</p>
          )}
        </div>

        {/* Phone */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Phone
          </label>
          <input
            {...register("phone")}
            type="tel"
            placeholder="+1 555 000 0000"
            className={inputCls}
          />
        </div>

        {/* Address */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Address
          </label>
          <input
            {...register("address")}
            placeholder="123 Main St, San Francisco, CA"
            className={inputCls}
          />
        </div>

        {/* Country */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Country
          </label>
          <input
            {...register("country")}
            placeholder="US"
            className={inputCls}
          />
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
          form="company-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create company"}
        </button>
      </div>
    </div>
  );
}
