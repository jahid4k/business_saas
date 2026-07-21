"use client";

import { use, useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import { getOrganization, updateOrganization } from "@/lib/org";
import type { Business, UpdateOrgRequest } from "@/types/org";

// ── Common timezones & currencies ─────────────────────────
const TIMEZONES = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Toronto",
  "America/Vancouver",
  "America/Sao_Paulo",
  "Europe/London",
  "Europe/Paris",
  "Europe/Berlin",
  "Europe/Madrid",
  "Europe/Amsterdam",
  "Europe/Rome",
  "Europe/Moscow",
  "Asia/Dubai",
  "Asia/Kolkata",
  "Asia/Dhaka",
  "Asia/Bangkok",
  "Asia/Singapore",
  "Asia/Shanghai",
  "Asia/Tokyo",
  "Asia/Seoul",
  "Australia/Sydney",
  "Australia/Melbourne",
  "Pacific/Auckland",
  "Pacific/Honolulu",
];

const CURRENCIES = [
  "USD",
  "EUR",
  "GBP",
  "CAD",
  "AUD",
  "JPY",
  "INR",
  "BDT",
  "SGD",
];

// ── Validation ────────────────────────────────────────
const schema = z.object({
  name: z.string().min(2, "Name must be at least 2 characters").max(100),
  legalName: z.string().max(100).optional(),
  type: z.string().max(50).optional(),
  industry: z.string().max(50).optional(),
  website: z.string().url("Must be a valid URL").or(z.literal("")).optional(),
  logoURL: z.string().url("Must be a valid URL").or(z.literal("")).optional(),
  country: z.string().max(50).optional(),
  timezone: z.string().optional(),
  currency: z.string().optional(),
});
type WorkspaceValues = z.infer<typeof schema>;

const cls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all disabled:opacity-50 disabled:cursor-not-allowed
`;

function formValuesFrom(b: Business): WorkspaceValues {
  return {
    name: b.name ?? "",
    legalName: b.legalName ?? "",
    type: b.type ?? "",
    industry: b.industry ?? "",
    website: b.website ?? "",
    logoURL: b.logoURL ?? "",
    country: b.country ?? "",
    timezone: b.timezone ?? "UTC",
    currency: b.currency ?? "USD",
  };
}

export default function WorkspaceSettingsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const queryClient = useQueryClient();
  const { currentOrg, setOrg, currentMembership } = useAuthStore();
  const syncedRef = useRef(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting, isDirty },
  } = useForm<WorkspaceValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      legalName: "",
      type: "",
      industry: "",
      website: "",
      logoURL: "",
      country: "",
      timezone: "UTC",
      currency: "USD",
    },
  });

  // ── Queries ───────────────────────────────────────────────────────────────
  const orgQuery = useQuery({
    queryKey: ["organization", orgId],
    queryFn: () => getOrganization(orgId),
    refetchOnWindowFocus: false,
  });

  // ── Sync query data → form ───────────────────────────────────────────────
  useEffect(() => {
    if (syncedRef.current) return;

    if (orgQuery.data) {
      syncedRef.current = true;
      reset(formValuesFrom(orgQuery.data));
    } else if (orgQuery.isError && currentOrg?.id === orgId) {
      syncedRef.current = true;
      reset(formValuesFrom(currentOrg));
    }
  }, [orgQuery.data, orgQuery.isError, currentOrg, orgId, reset]);

  // ── Mutations ─────────────────────────────────────────────────────────────
  const onSubmit = async (values: WorkspaceValues) => {
    try {
      const payload: UpdateOrgRequest = {
        name: values.name,
        legalName: values.legalName || undefined,
        type: values.type || undefined,
        industry: values.industry || undefined,
        website: values.website || undefined,
        logoURL: values.logoURL || undefined,
        country: values.country || undefined,
        timezone: values.timezone || undefined,
        currency: values.currency || undefined,
      };

      const updated = await updateOrganization(orgId, payload);
      queryClient.setQueryData(["organization", orgId], updated);

      // Update the current organization in the store if it's the active one
      if (currentOrg?.id === orgId) {
        setOrg(updated, currentMembership);
      }

      reset(formValuesFrom(updated));
      toast.success("Workspace updated successfully.");
    } catch (err: unknown) {
      const errorObj = err as {
        response?: { data?: { error?: { message?: string } } };
      };
      const errDetail =
        errorObj?.response?.data?.error?.message ||
        "Failed to save workspace settings.";
      toast.error(errDetail);
    }
  };

  const initial =
    orgQuery.data?.name?.[0]?.toUpperCase() ??
    currentOrg?.name?.[0]?.toUpperCase() ??
    "?";
  const logoUrl = orgQuery.data?.logoURL ?? currentOrg?.logoURL;

  const formatDate = (iso?: string) =>
    iso
      ? new Date(iso).toLocaleDateString("en-US", {
          month: "long",
          day: "numeric",
          year: "numeric",
        })
      : "—";

  return (
    <div className="p-6 md:p-8 max-w-2xl">
      {/* Header */}
      <div className="mb-8">
        <h1
          className="text-2xl font-bold text-(--text-primary) mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          Workspace Settings
        </h1>
        <p className="text-sm text-(--text-muted)">
          Manage your organization&apos;s general information
        </p>
      </div>

      {orgQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          Could not refresh workspace data from the server.
        </div>
      )}

      {orgQuery.isPending ? (
        <div className="flex items-center gap-3 py-16 text-sm text-(--text-muted)">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading workspace details…
        </div>
      ) : (
        <div className="space-y-6">
          {/* Logo preview */}
          <div className="p-6 rounded-xl border border-(--border) bg-(--bg-surface)">
            <div className="flex items-start gap-6">
              <div
                className="w-20 h-20 rounded-xl overflow-hidden relative shrink-0"
                style={{
                  background: logoUrl
                    ? undefined
                    : "linear-gradient(135deg, #7c3aed, #a855f7)",
                }}
              >
                {logoUrl ? (
                  /* eslint-disable-next-line @next/next/no-img-element */
                  <img
                    src={logoUrl}
                    alt="Logo"
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <span
                    className="w-full h-full flex items-center justify-center text-2xl font-bold text-white"
                    style={{
                      fontFamily: "var(--font-syne, Syne, sans-serif)",
                    }}
                  >
                    {initial}
                  </span>
                )}
              </div>
              <div>
                <p
                  className="text-lg font-bold text-(--text-primary) mb-0.5"
                  style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
                >
                  {orgQuery.data?.name ?? currentOrg?.name}
                </p>
                <p className="text-sm text-(--text-muted)">
                  {orgQuery.data?.slug ?? currentOrg?.slug}
                </p>
              </div>
            </div>
          </div>

          {/* ── Organization information form ──────────── */}
          <div className="rounded-xl border border-(--border) bg-(--bg-surface) overflow-hidden">
            <div className="px-6 py-4 border-b border-(--border)">
              <p
                className="text-sm font-semibold text-(--text-primary)"
                style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
              >
                Workspace Details
              </p>
            </div>

            <form onSubmit={handleSubmit(onSubmit)} className="p-6 space-y-5">
              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-(--text-secondary)">
                  Organization Name <span className="text-red-400">*</span>
                </label>
                <input
                  {...register("name")}
                  placeholder="Acme Corp"
                  className={cls}
                />
                {errors.name && (
                  <p className="text-xs text-red-400">{errors.name.message}</p>
                )}
              </div>

              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-(--text-secondary)">
                  Legal Name
                </label>
                <input
                  {...register("legalName")}
                  placeholder="Acme Corporation Ltd."
                  className={cls}
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    Industry
                  </label>
                  <input
                    {...register("industry")}
                    placeholder="Software, Retail, etc."
                    className={cls}
                  />
                </div>
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    Company Type
                  </label>
                  <input
                    {...register("type")}
                    placeholder="B2B, B2C, Agency..."
                    className={cls}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    Website
                  </label>
                  <input
                    {...register("website")}
                    placeholder="https://example.com"
                    className={cls}
                  />
                  {errors.website && (
                    <p className="text-xs text-red-400">
                      {errors.website.message}
                    </p>
                  )}
                </div>
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    Logo URL
                  </label>
                  <input
                    {...register("logoURL")}
                    placeholder="https://example.com/logo.png"
                    className={cls}
                  />
                  {errors.logoURL && (
                    <p className="text-xs text-red-400">
                      {errors.logoURL.message}
                    </p>
                  )}
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-(--text-secondary)">
                  Country
                </label>
                <input
                  {...register("country")}
                  placeholder="United States, United Kingdom..."
                  className={cls}
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    Timezone
                  </label>
                  <select {...register("timezone")} className={cls}>
                    {TIMEZONES.map((tz) => (
                      <option
                        key={tz}
                        value={tz}
                        style={{ background: "var(--bg-elevated)" }}
                      >
                        {tz}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    Currency
                  </label>
                  <select {...register("currency")} className={cls}>
                    {CURRENCIES.map((c) => (
                      <option
                        key={c}
                        value={c}
                        style={{ background: "var(--bg-elevated)" }}
                      >
                        {c}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="flex items-center gap-3 pt-2">
                <button
                  type="submit"
                  disabled={isSubmitting || !isDirty}
                  className="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 size={14} className="animate-spin" />
                      Saving…
                    </>
                  ) : (
                    "Save changes"
                  )}
                </button>
              </div>
            </form>
          </div>

          {/* ── System information (read only) ─────── */}
          <div className="rounded-xl border border-(--border) bg-(--bg-surface) overflow-hidden">
            <div className="px-6 py-4 border-b border-(--border)">
              <p
                className="text-sm font-semibold text-(--text-primary)"
                style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
              >
                System information
              </p>
            </div>

            <div className="divide-y divide-(--border)">
              {[
                {
                  label: "Workspace ID",
                  value: orgQuery.data?.publicId ?? "—",
                },
                { label: "Status", value: orgQuery.data?.status ?? "—" },
                {
                  label: "Created at",
                  value: formatDate(orgQuery.data?.createdAt),
                },
              ].map((row) => (
                <div
                  key={row.label}
                  className="flex items-center justify-between px-6 py-3.5"
                >
                  <span className="text-sm text-(--text-muted)">
                    {row.label}
                  </span>
                  <span
                    className="text-sm text-(--text-secondary) capitalize"
                    style={{
                      fontFamily: "var(--font-inter, Inter, sans-serif)",
                    }}
                  >
                    {row.value}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
