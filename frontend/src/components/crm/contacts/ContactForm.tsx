// src/components/crm/contacts/ContactForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import { listCompanies } from "@/lib/crm/companies";
import type { Contact, Company } from "@/types/crm";

const SOURCES = [
  { value: "linkedin", label: "LinkedIn" },
  { value: "website", label: "Website" },
  { value: "referral", label: "Referral" },
  { value: "cold_call", label: "Cold Call" },
  { value: "email_campaign", label: "Email Campaign" },
  { value: "trade_show", label: "Trade Show" },
  { value: "other", label: "Other" },
];

const schema = z.object({
  first_name: z.string().min(1, "First name is required"),
  last_name: z.string().optional(),
  email: z.string().email("Invalid email").optional().or(z.literal("")),
  phone: z.string().optional(),
  title: z.string().optional(),
  company_id: z.string().optional(),
  source: z.string().optional(),
});
type ContactFormValues = z.infer<typeof schema>;

interface ContactFormProps {
  contact?: Contact | null;
  preselectedCompany?: string; // company_id to pre-select (from company detail page)
  orgId: string;
  onSave: (values: ContactFormValues) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function ContactForm({
  contact,
  preselectedCompany,
  orgId,
  onSave,
}: ContactFormProps) {
  const { closeDrawer } = useDrawer();
  const [companies, setCompanies] = useState<Company[]>([]);
  const [error, setError] = useState<string | null>(null);
  const isEdit = !!contact;

  // Fetch companies for the dropdown
  useEffect(() => {
    listCompanies(orgId)
      .then((res) => setCompanies(res.companies))
      .catch(() => {
        /* non-critical, dropdown will be empty */
      });
  }, [orgId]);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ContactFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      first_name: contact?.first_name ?? "",
      last_name: contact?.last_name ?? "",
      email: contact?.email ?? "",
      phone: contact?.phone ?? "",
      title: contact?.title ?? "",
      company_id: contact?.company_id ?? preselectedCompany ?? "",
      source: contact?.source ?? "",
    },
  });

  const onSubmit = async (values: ContactFormValues) => {
    setError(null);
    const payload = Object.fromEntries(
      Object.entries(values).map(([k, v]) => [k, v === "" ? undefined : v]),
    ) as ContactFormValues;
    try {
      await onSave(payload);
      closeDrawer();
    } catch {
      setError("Failed to save contact. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="contact-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {/* Name row */}
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              First name <span className="text-red-400">*</span>
            </label>
            <input
              {...register("first_name")}
              autoFocus
              placeholder="Alice"
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
              placeholder="Johnson"
              className={inputCls}
            />
          </div>
        </div>

        {/* Email */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Email
          </label>
          <input
            {...register("email")}
            type="email"
            placeholder="alice@company.com"
            className={inputCls}
          />
          {errors.email && (
            <p className="text-xs text-red-400">{errors.email.message}</p>
          )}
        </div>

        {/* Phone + Title */}
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Phone
            </label>
            <input
              {...register("phone")}
              type="tel"
              placeholder="+1 555 000 0000"
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Job title
            </label>
            <input
              {...register("title")}
              placeholder="Head of Product"
              className={inputCls}
            />
          </div>
        </div>

        {/* Company dropdown */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Company
          </label>
          <select {...register("company_id")} className={inputCls}>
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

        {/* Source */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Source
          </label>
          <select {...register("source")} className={inputCls}>
            <option value="">Select source</option>
            {SOURCES.map((s) => (
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
          form="contact-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create contact"}
        </button>
      </div>
    </div>
  );
}
